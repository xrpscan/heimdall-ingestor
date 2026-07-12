package store

import (
	"context"
	"fmt"
	"strings"
)

func (p *PostgresClient) InsertValidationMessagesIfNotExist(
	ctx context.Context, messages []ValidationMessage,
) error {
	// Form query.
	query, args := p.queryInsertValidationMessagesIfNotExist(messages)

	// Execute query.
	result, err := p.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("error in query execution: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to obtain affected row count: %w", err)
	}

	// Verify that affected row count does not exceed the message count.
	// It can be less than the message count as duplicates are skipped.
	if int(count) > len(messages) {
		return fmt.Errorf("unexpected number of rows were inserted: %d, expected <= %d",
			count, len(messages))
	}

	return nil
}

func (p *PostgresClient) queryInsertValidationMessagesIfNotExist(
	messages []ValidationMessage,
) (string, []any) {
	// Number of columns for each row in the query.
	// This may not be equal to the actual number of columns in the table,
	// since id and created_at are not inserted.
	const columnCount = 4
	// Create size as per total argument count.
	args := make([]any, len(messages)*columnCount)

	var valueBuilder strings.Builder
	valueBuilder.Grow(len(messages) * 25) // Reasonable buffer pre-allocation.

	for i := 0; i < len(messages)*columnCount; i += columnCount {
		fmt.Fprintf(&valueBuilder, `($%d, $%d, $%d, $%d), `, i+1, i+2, i+3, i+4)

		// Populate arguments.
		item := messages[i/columnCount]
		args[i], args[i+1], args[i+2], args[i+3] = item.MasterKey, item.LedgerIndex,
			item.Payload, item.HeimTimestamp
	}

	// Remove trailing comma-space from the earlier string-building.
	values := strings.TrimSuffix(valueBuilder.String(), ", ")
	return `INSERT INTO validations (master_key, ledger_index, payload, heim_timestamp) VALUES ` +
		values + ` ON CONFLICT (master_key, ledger_index) DO NOTHING;`, args
}
