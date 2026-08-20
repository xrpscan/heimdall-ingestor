package handlers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/xrpscan/heimdall-ingestor/internal/store"
)

// handleLedgerMessageBatch processes a Kafka message payload coming from the ledger topic.
func (k KafkaRecordHandler) handleLedgerMessageBatch(
	ctx context.Context, batch []ledgerBatchItem,
) error {
	// The validation layer.
	filteredBatch := filterBatch(ctx, batch, checkLedgerBatchItem)

	// If all messages were invalid, return early.
	// TODO: Upper limit on batch size? 5000?
	if len(filteredBatch) == 0 {
		slog.WarnContext(ctx, "no valid messages in the batch")
		return nil
	}

	// This will be used for the batch insertion.
	persistable := make([]store.LedgerMessage, len(filteredBatch))

	// Make messages ready for persistence. No errors must be swallowed here.
	for i, item := range filteredBatch {
		// No need to check error since the validation checks are already in place.
		idx, _ := item.Message.LedgerIndexParsed()
		persistable[i] = store.LedgerMessage{
			LedgerIndex:       idx,
			LedgerHash:        item.Message.LedgerHash,
			ObserverCreatedAt: store.Timestamp(item.CreatedAt),
		}
	}

	// Store in database.
	insertedCount, err := k.opts.Database.InsertLedgerMessagesIfNotExist(ctx, persistable)
	if err != nil {
		return fmt.Errorf("failed to persist batch: %w", err)
	}

	slog.InfoContext(ctx, "successfully inserted the batch in the database",
		"insertedCount", insertedCount, "totalCount", len(filteredBatch))
	return nil
}
