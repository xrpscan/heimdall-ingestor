package store

import "context"

// Client for the application's storage layer.
type Client interface {
	// BulkInsertValidationMessages allows inserting multiple validation messages into the DB.
	// A bulk insert helps because the caller can insert in batches if their message influx is high.
	BulkInsertValidationMessages(ctx context.Context, messages []ValidationMessage) error
}
