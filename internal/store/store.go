package store

import (
	"context"
)

// Client for the application's storage layer.
type Client interface {
	// InsertValidationMessagesIfNotExist inserts multiple entries in the validations table.
	// If any of the given messages already exist in the table, they are skipped without errors.
	InsertValidationMessagesIfNotExist(
		ctx context.Context, messages []ValidationMessage,
	) (int64, error)

	// InsertLedgerMessagesIfNotExist inserts multiple entries in the ledger table.
	// If any of the given messages already exist in the table, they are skipped without errors.
	InsertLedgerMessagesIfNotExist(
		ctx context.Context, messages []LedgerMessage,
	) (int64, error)

	// UpsertValidatorManifests upserts multiple entries in the validator_manifests table.
	//
	// Note that the CreatedAt and UpdatedAt fields are auto-populated. The provided values get
	// ignored.
	UpsertValidatorManifests(ctx context.Context, manifests []ValidatorManifest) (int64, error)
}
