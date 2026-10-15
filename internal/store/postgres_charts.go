package store

import (
	"context"
	"fmt"
	"slices"
	"time"
)

func (p *PostgresClient) GetValidatorCount(
	ctx context.Context, startTime, endTime time.Time, unlOnly bool,
) (uint, error) {
	query := `
SELECT
	COUNT(DISTINCT a.master_key)
FROM
	agreements a
LEFT JOIN validator_manifests vm ON a.master_key = vm.master_key
WHERE
	a.ledger_closed_at BETWEEN $1 AND $2
	AND ($3 = FALSE OR vm.is_unl = TRUE)`

	var total uint
	if err := p.db.QueryRowContext(ctx, query, startTime, endTime, unlOnly).Scan(&total); err != nil {
		return 0, fmt.Errorf("error in query execution: %w", err)
	}

	return total, nil
}

// GetLedgerCount fetches the count of ledgerClosed messages recevied within the given time window
// grouped by the 'minute' interval.
func (p *PostgresClient) GetLedgerCount(
	ctx context.Context, startTime, endTime time.Time, bucket Bucket,
) (map[time.Time]int, error) {
	query := `
	SELECT
		date_trunc($1, l.observer_created_at) AS time,
		COUNT(*)
	FROM ledger l
	WHERE l.observer_created_at BETWEEN $2 AND $3
	GROUP BY date_trunc($1, l.observer_created_at)
	ORDER BY time;
	`

	// Execute query.
	rows, err := p.db.QueryContext(ctx, query, string(bucket), startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("error in QueryContext call: %w", err)
	}

	defer func() { _ = rows.Close() }()

	result := map[time.Time]int{}
	for rows.Next() {
		var tim time.Time
		var count int
		if err := rows.Scan(&tim, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		result[tim] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return result, nil
}

func (p *PostgresClient) GetValidatorAgreementRateBuckets(
	ctx context.Context, options GetValidatorAgreementRateBucketsOptions,
) (AgreementRateData, error) {
	// Required to calculate agreement percentages.
	ledgerCounts, err := p.GetLedgerCount(ctx, options.StartTime, options.EndTime, options.Bucket)
	if err != nil {
		return AgreementRateData{}, fmt.Errorf("failed to get ledger counts: %w", err)
	}

	query := `
		SELECT
		  date_trunc($1, a.ledger_closed_at) AS time,
		  vm.domain,
		  vm.domain_verified,
		  COUNT(*) FILTER (WHERE a.agreed)
		FROM agreements a
		LEFT JOIN validator_manifests vm ON a.master_key = vm.master_key
		WHERE a.ledger_closed_at BETWEEN $2 AND $3
		AND a.master_key = $4
		GROUP BY date_trunc($1, a.ledger_closed_at), a.master_key, vm.domain, vm.domain_verified
		ORDER BY time;
	`

	// Execute query.
	rows, err := p.db.QueryContext(ctx, query, string(options.Bucket),
		options.StartTime, options.EndTime, options.MasterKey)
	if err != nil {
		return AgreementRateData{}, fmt.Errorf("error in QueryContext call: %w", err)
	}

	defer func() { _ = rows.Close() }()

	// Decode results.
	var results ValidatorAgreementCounts
	results.MasterKey = options.MasterKey

	for rows.Next() {
		var domain string
		var domainVerified bool
		var row TimedInt
		if err := rows.Scan(&row.Time, &domain, &domainVerified, &row.Value); err != nil {
			return AgreementRateData{}, fmt.Errorf("failed to scan row: %w", err)
		}

		results.Agreement = append(results.Agreement, row)
		results.Domain = domain
		results.DomainVerified = domainVerified
	}

	if err := rows.Err(); err != nil {
		return AgreementRateData{}, fmt.Errorf("row iteration error: %w", err)
	}

	return AgreementRateData{
		Validator:    results,
		LedgerCounts: ledgerCounts,
	}, nil
}

func (p *PostgresClient) GetAllValidatorAgreementRateBuckets(
	ctx context.Context, options GetAllValidatorAgreementRateBucketsOptions,
) (AgreementRatesData, error) {
	// Required to calculate agreement percentages.
	ledgerCounts, err := p.GetLedgerCount(ctx, options.StartTime, options.EndTime, options.Bucket)
	if err != nil {
		return AgreementRatesData{}, fmt.Errorf("failed to get ledger counts: %w", err)
	}

	query := `
		SELECT
		  date_trunc($1, a.ledger_closed_at) AS time,
		  a.master_key,
		  vm.domain,
		  vm.domain_verified,
		  COUNT(*) FILTER (WHERE a.agreed)
		FROM agreements a
		LEFT JOIN validator_manifests vm ON a.master_key = vm.master_key
		WHERE a.ledger_closed_at BETWEEN $2 AND $3
		AND ($4 = FALSE OR vm.is_unl = TRUE)
		GROUP BY date_trunc($1, a.ledger_closed_at), a.master_key, vm.domain, vm.domain_verified
		ORDER BY time;
	`

	// Execute query.
	rows, err := p.db.QueryContext(ctx, query, string(options.Bucket),
		options.StartTime, options.EndTime, options.UNLOnly)
	if err != nil {
		return AgreementRatesData{}, fmt.Errorf("error in QueryContext call: %w", err)
	}

	defer func() { _ = rows.Close() }()

	// Decode results.
	results := map[string]ValidatorAgreementCounts{}
	for rows.Next() {
		var masterKey, domain string
		var domainVerified bool
		var row TimedInt
		if err := rows.Scan(&row.Time, &masterKey, &domain, &domainVerified, &row.Value); err != nil {
			return AgreementRatesData{}, fmt.Errorf("failed to scan row: %w", err)
		}

		if _, exists := results[masterKey]; !exists {
			results[masterKey] = ValidatorAgreementCounts{
				MasterKey:      masterKey,
				Domain:         domain,
				DomainVerified: domainVerified,
			}
		}

		item := results[masterKey]
		item.Agreement = append(item.Agreement, row)
		results[masterKey] = item
	}

	if err := rows.Err(); err != nil {
		return AgreementRatesData{}, fmt.Errorf("row iteration error: %w", err)
	}

	// Sum up agreement count for all validators to rank their performance.
	agreementCount := map[string]int{}
	for _, item := range results {
		for _, v := range item.Agreement {
			agreementCount[item.MasterKey] += v.Value
		}
	}

	// The slice that will be sorted (currently unsorted).
	counts := make([]ValidatorAgreementCounts, 0, len(results))
	for _, data := range results {
		counts = append(counts, data)
	}

	// Sort as per validator performance.
	slices.SortStableFunc(counts, func(a ValidatorAgreementCounts, b ValidatorAgreementCounts) int {
		return agreementCount[b.MasterKey] - agreementCount[a.MasterKey]
	})

	return AgreementRatesData{
		Validators:   counts,
		LedgerCounts: ledgerCounts,
	}, nil
}

func (p *PostgresClient) GetLedgerCloseIntervals(
	ctx context.Context, startTime, endTime time.Time,
) ([]LedgerCloseInterval, error) {
	query := `
		SELECT
		   observer_created_at AS time,
		   EXTRACT(EPOCH FROM observer_created_at - LAG(observer_created_at) OVER (ORDER BY ledger_index)) AS interval_seconds,
		   ledger_index
		 FROM ledger
		 WHERE observer_created_at BETWEEN $1 AND $2
		 ORDER BY ledger_index
	`

	rows, err := p.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("error in QueryContext call: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []LedgerCloseInterval
	for rows.Next() {
		var row LedgerCloseInterval
		var intervalSeconds *float64 // NULL for the first row (no previous ledger)

		if err := rows.Scan(&row.Time, &intervalSeconds, &row.LedgerIndex); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Skip the first row since it has no interval (LAG returns NULL)
		if intervalSeconds == nil {
			continue
		}

		row.IntervalSeconds = *intervalSeconds
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return results, nil
}
