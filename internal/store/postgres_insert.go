package store

import (
	"context"
	"fmt"
	"log/slog"
)

func (p *PostgresClient) InsertValidationMessagesIfNotExist(
	ctx context.Context, messages []ValidationMessage,
) (int64, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	// Form query.
	query, args := p.queryInsertValidationMessagesIfNotExist(messages)
	// Execute query.
	result, err := p.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("error in query execution: %w", err)
	}

	// We should verify the number of rows affected to report unexpected behaviours.
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to obtain affected row count: %w", err)
	}

	// Verify that affected row count does not exceed the message count.
	// It can be less than the message count as duplicates are skipped.
	if count > int64(len(messages)) {
		return count, fmt.Errorf("unexpected number of rows were inserted: %d, expected <= %d",
			count, len(messages))
	}

	return count, nil
}

func (p *PostgresClient) InsertLedgerMessagesIfNotExist(
	ctx context.Context, messages []LedgerMessage,
) (int64, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	// Form query.
	query, args := p.queryInsertLedgerMessagesIfNotExist(messages)
	// Execute query.
	result, err := p.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("error in query execution: %w", err)
	}

	// We should verify the number of rows affected to report unexpected behaviours.
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to obtain affected row count: %w", err)
	}

	// Verify that affected row count does not exceed the message count.
	// It can be less than the message count as duplicates are skipped.
	if count > int64(len(messages)) {
		return count, fmt.Errorf("unexpected number of rows were inserted: %d, expected <= %d",
			count, len(messages))
	}

	return count, nil
}

func (p *PostgresClient) UpsertValidatorManifests(
	ctx context.Context, manifests []ValidatorManifest,
) (int64, error) {
	if len(manifests) == 0 {
		return 0, nil
	}

	// Form query and args.
	query, args := p.queryUpsertValidatorManifests(manifests)
	result, err := p.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute query: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows count: %w", err)
	}

	return count, nil
}

func (p *PostgresClient) UpdateUNLValidators(ctx context.Context, masterKeys []string) error {
	if len(masterKeys) == 0 {
		return nil
	}

	// Form query and args.
	query, args := p.queryUpdateUNLValidators(masterKeys)
	result, err := p.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows count: %w", err)
	}

	slog.InfoContext(ctx, "rows affected in unl update operation", "count", count)
	return nil
}

func (p *PostgresClient) queryInsertValidationMessagesIfNotExist(
	messages []ValidationMessage,
) (string, []any) {
	var rowCount, columnCount = len(messages), 6
	args := make([]any, rowCount*columnCount)

	for i := 0; i < rowCount*columnCount; i += columnCount {
		item := messages[i/columnCount]
		args[i], args[i+1], args[i+2], args[i+3], args[i+4], args[i+5] =
			item.MasterKey, item.LedgerIndex, item.LedgerHash,
			item.Payload, item.UnixSigningTime, item.ObserverCreatedAt
	}

	return `INSERT INTO
	validations
		(master_key, ledger_index, ledger_hash, payload, unix_signing_time, observer_created_at)
	VALUES ` + buildValueString(rowCount, columnCount) + `
	ON CONFLICT (master_key, ledger_index) DO NOTHING;`, args
}

func (p *PostgresClient) queryInsertLedgerMessagesIfNotExist(
	messages []LedgerMessage,
) (string, []any) {
	var rowCount, columnCount = len(messages), 3
	args := make([]any, rowCount*columnCount)

	for i := 0; i < rowCount*columnCount; i += columnCount {
		item := messages[i/columnCount]
		args[i], args[i+1], args[i+2] = item.LedgerIndex,
			item.LedgerHash, item.ObserverCreatedAt
	}

	return `INSERT INTO ledger (ledger_index, ledger_hash, observer_created_at) VALUES ` +
		buildValueString(rowCount, columnCount) + ` ON CONFLICT (ledger_index) DO NOTHING;`, args
}

func (p *PostgresClient) queryUpsertValidatorManifests(
	manifests []ValidatorManifest,
) (string, []any) {
	var rowCount, columnCount = len(manifests), 2
	args := make([]any, rowCount*columnCount)

	for i := 0; i < rowCount*columnCount; i += columnCount {
		item := manifests[i/columnCount]
		args[i], args[i+1] = item.MasterKey, item.Domain
	}

	return `INSERT INTO validator_manifests (master_key, domain) VALUES ` +
		buildValueString(rowCount, columnCount) + ` ON CONFLICT (master_key)
		DO UPDATE SET domain = EXCLUDED.domain;`, args
}

func (p *PostgresClient) queryUpdateUNLValidators(masterKeys []string) (string, []any) {
	valueString := buildValueString(1, len(masterKeys))

	args := make([]any, len(masterKeys))
	for i, key := range masterKeys {
		args[i] = key
	}

	return `
UPDATE
	validator_manifests
SET
	is_unl = CASE
    WHEN master_key IN ` + valueString + ` THEN TRUE ELSE FALSE END
WHERE is_unl IS DISTINCT FROM (master_key IN ` + valueString + `);`, args
}
