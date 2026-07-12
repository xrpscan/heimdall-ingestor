package store

import "context"

// Client for the application's storage layer.
type Client interface {
	// InsertValidationMessagesIfNotExist inserts multiple entries in the validations table.
	// If any of the given messages already exist in the table, they are skipped without errors.
	InsertValidationMessagesIfNotExist(
		ctx context.Context, messages []ValidationMessage,
	) (int64, error)
}
