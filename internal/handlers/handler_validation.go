package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/xrpscan/heimdall-ingestor/internal/store"
)

// handleValidationMessageBatch processes a Kafka message payload coming from the validations topic.
func (k KafkaRecordHandler) handleValidationMessageBatch(
	ctx context.Context, batch []validationBatchItem,
) error {
	// The validation layer.
	filteredBatch := filterBatch(ctx, batch, checkValidationBatchItem)

	// If all messages were invalid, return early.
	// TODO: Upper limit on batch size? 5000?
	if len(filteredBatch) == 0 {
		slog.WarnContext(ctx, "no valid messages in the batch")
		return nil
	}

	// This will be used for the batch insertion.
	persistable := make([]store.ValidationMessage, len(filteredBatch))
	// For invoking ValidatorManifestUpdaterFunc.
	uniqueMasterKeys := map[string]struct{}{}

	// Make messages ready for persistence. No errors must be swallowed here.
	for i, item := range filteredBatch {
		enriched, err := k.enrichValidationMessage(ctx, item)
		if err != nil {
			return fmt.Errorf("failed to enrich message: %w", err)
		}
		persistable[i] = enriched
		uniqueMasterKeys[enriched.MasterKey] = struct{}{}
	}

	// Store in database.
	insertedCount, err := k.opts.Database.InsertValidationMessagesIfNotExist(ctx, persistable)
	if err != nil {
		return fmt.Errorf("failed to persist batch: %w", err)
	}

	// Mark validators for manifest update.
	for key := range uniqueMasterKeys {
		k.opts.ValidatorManifestUpdaterFunc(key)
	}

	slog.InfoContext(ctx, "successfully inserted the batch in the database",
		"insertedCount", insertedCount, "totalCount", len(filteredBatch))

	return nil
}

// enrichValidationMessage enriches the given batch item by assigning required fields like
// UnixSigningTime, ObserverCreatedAt etc.
func (k KafkaRecordHandler) enrichValidationMessage(
	_ context.Context, item validationBatchItem,
) (store.ValidationMessage, error) {
	message := item.Message

	// If master key is absent, the validation public key can be assumed to be the master key.
	masterKey := message.MasterKey
	if message.MasterKey == "" {
		masterKey = message.ValidationPublicKey
	}

	// This is already validated so no need to handle error.
	li, _ := strconv.ParseUint(message.LedgerIndex, 10, 64)
	// SigningTime is in seconds but database expects milliseconds.
	unixSigningTimeMs := xrplEpochToUnixEpoch(message.SigningTime) * 1000

	// Payload byte slice.
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return store.ValidationMessage{}, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return store.ValidationMessage{
		MasterKey:         masterKey,
		LedgerIndex:       int64(li),
		LedgerHash:        message.LedgerHash,
		Payload:           messageBytes,
		UnixSigningTime:   store.Timestamp(unixSigningTimeMs),
		ObserverCreatedAt: store.Timestamp(item.CreatedAt),
	}, nil
}
