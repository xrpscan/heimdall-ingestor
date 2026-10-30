package store

import (
	"context"
	"fmt"
	"log/slog"
)

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
