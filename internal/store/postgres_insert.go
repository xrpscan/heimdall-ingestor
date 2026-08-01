package store

import (
	"context"
	"fmt"
	"strings"
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
	if int(count) > len(messages) {
		return count, fmt.Errorf("unexpected number of rows were inserted: %d, expected <= %d",
			count, len(messages))
	}

	return count, nil
}

func (p *PostgresClient) queryInsertValidationMessagesIfNotExist(
	messages []ValidationMessage,
) (string, []any) {
	// Number of columns for each row in the query.
	// This may not be equal to the actual number of columns in the table,
	// since id and created_at are not inserted.
	const columnCount = 5
	// Create size as per total argument count.
	args := make([]any, len(messages)*columnCount)

	var valueBuilder strings.Builder
	valueBuilder.Grow(len(messages) * 25) // Reasonable buffer pre-allocation.

	for i := 0; i < len(messages)*columnCount; i += columnCount {
		fmt.Fprintf(&valueBuilder, `($%d, $%d, $%d, $%d, $%d), `, i+1, i+2, i+3, i+4, i+5)

		// Populate arguments.
		item := messages[i/columnCount]
		args[i], args[i+1], args[i+2], args[i+3], args[i+4] = item.MasterKey, item.LedgerIndex,
			item.Payload, item.UnixSigningTime, item.ObserverCreatedAt
	}

	// Remove trailing comma-space from the earlier string-building.
	values := strings.TrimSuffix(valueBuilder.String(), ", ")
	return `INSERT INTO
	validations
		(master_key, ledger_index, payload, unix_signing_time, observer_created_at)
	VALUES ` + values + `
	ON CONFLICT (master_key, ledger_index) DO NOTHING;`, args
}

func (p *PostgresClient) queryInsertLedgerMessagesIfNotExist(
	messages []LedgerMessage,
) (string, []any) {
	// Number of columns for each row in the query.
	// This may not be equal to the actual number of columns in the table,
	// since id and created_at are not inserted.
	const columnCount = 3
	// Create size as per total argument count.
	args := make([]any, len(messages)*columnCount)

	var valueBuilder strings.Builder
	valueBuilder.Grow(len(messages) * 25) // Reasonable buffer pre-allocation.

	for i := 0; i < len(messages)*columnCount; i += columnCount {
		fmt.Fprintf(&valueBuilder, `($%d, $%d, $%d), `, i+1, i+2, i+3)

		// Populate arguments.
		item := messages[i/columnCount]
		args[i], args[i+1], args[i+2] = item.LedgerIndex,
			item.LedgerHash, item.ObserverCreatedAt
	}

	// Remove trailing comma-space from the earlier string-building.
	values := strings.TrimSuffix(valueBuilder.String(), ", ")
	return `INSERT INTO ledger (ledger_index, ledger_hash, observer_created_at) VALUES ` +
		values + ` ON CONFLICT (ledger_index) DO NOTHING;`, args
}
