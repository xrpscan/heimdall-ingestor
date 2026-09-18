package store

import (
	"context"
	"fmt"
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

func (p *PostgresClient) GetValidatorAgreementRates(
	ctx context.Context, options ValidatorAgreementHeatmapOptions,
) (AgreementRatesData, error) {
	totalCount, err := p.GetValidatorCount(ctx, options.StartTime, options.EndTime, options.UNLOnly)
	if err != nil {
		return AgreementRatesData{}, fmt.Errorf("failed to get total count: %w", err)
	}

	// TODO: For now, the time bucket is always a minute (see date_trunc in the query). Replace it
	// smarter bucketing derived from the time duration (endTime - startTime).
	//
	// This query yields the list of agreement rates (in %) for every minute, for every validator,
	// within the given time range.
	query := `
		WITH top_validators AS (
			SELECT a.master_key,
			       COUNT(*) FILTER (WHERE a.agreed) * 100.0 / NULLIF(COUNT(*), 0) AS agreement_rate,
			       COUNT(*) FILTER (WHERE a.agreed) AS agreed_count,
			       ROW_NUMBER() OVER (
			           ORDER BY COUNT(*) FILTER (WHERE a.agreed) * 100.0 / NULLIF(COUNT(*), 0) DESC,
			                    COUNT(*) FILTER (WHERE a.agreed) DESC
			       ) AS rank
			FROM agreements a
			LEFT JOIN validator_manifests vm ON a.master_key = vm.master_key
			WHERE a.ledger_closed_at BETWEEN $1 AND $2
			  AND ($5 = FALSE OR vm.is_unl = TRUE)
			GROUP BY a.master_key
			ORDER BY agreement_rate DESC, agreed_count DESC
			LIMIT $3 OFFSET $4
		)
		SELECT
		  date_trunc('minute', a.ledger_closed_at) AS time,
		  CASE WHEN vm.domain > '' THEN vm.domain ELSE a.master_key END AS validator,
		  COUNT(*) FILTER (WHERE a.agreed) * 100.0 / NULLIF(COUNT(*), 0) AS agreement_percent
		FROM agreements a
		JOIN top_validators t ON a.master_key = t.master_key
		LEFT JOIN validator_manifests vm ON a.master_key = vm.master_key
		WHERE a.ledger_closed_at BETWEEN $1 AND $2
		GROUP BY date_trunc('minute', a.ledger_closed_at), a.master_key, vm.domain, t.rank
		ORDER BY time, t.rank
	`

	// Execute query.
	rows, err := p.db.QueryContext(ctx, query, options.StartTime, options.EndTime,
		options.Limit, options.Offset, options.UNLOnly)
	if err != nil {
		return AgreementRatesData{}, fmt.Errorf("error in QueryContext call: %w", err)
	}

	defer func() { _ = rows.Close() }()

	// Decode results.
	var results []AgreementRate
	for rows.Next() {
		var row AgreementRate
		if err := rows.Scan(&row.Time, &row.Validator, &row.AgreementPercent); err != nil {
			return AgreementRatesData{}, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return AgreementRatesData{}, fmt.Errorf("row iteration error: %w", err)
	}

	return AgreementRatesData{Rates: results, TotalValidatorCount: totalCount}, nil
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
