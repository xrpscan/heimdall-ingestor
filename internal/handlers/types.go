package handlers

import (
	"github.com/xrpscan/heimdall-ingestor/internal/store"
)

// validationBatchItem: In the validations topic of Kafka, a single message is an array of objects.
// This struct is the schema of each object in that array.
type validationBatchItem struct {
	ID        int
	Message   store.ValidationMessagePayload
	CreatedAt int64
}

// ledgerBatchItem: In the ledger topic of Kafka, a single message is an array of objects.
// This struct is the schema of each object in that array.
type ledgerBatchItem struct {
	ID        int
	Message   store.LedgerMessagePayload
	CreatedAt int64
}
