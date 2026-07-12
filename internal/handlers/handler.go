package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/xrpscan/heimdall-ingestor/internal/store"
	"github.com/xrpscan/heimdall-ingestor/pkg/kafkaesque"
)

// KafkaRecordHandler encapsulates methods required to handle/process Kafka records/messages.
type KafkaRecordHandler struct {
	db store.Client
}

// NewKafkaRecordHandler returns a new KafkaRecordHandler instance.
func NewKafkaRecordHandler(db store.Client) KafkaRecordHandler {
	return KafkaRecordHandler{db: db}
}

// HandleValidationMessageBatch processes a batch (or list) of xrpld's validationReceived messages.
func (k KafkaRecordHandler) HandleValidationMessageBatch(
	ctx context.Context, record kafkaesque.Record,
) error {
	// Unmarshal to processable type.
	var batch []store.ValidationMessagePayload
	if err := json.Unmarshal(record.Payload, &batch); err != nil {
		return fmt.Errorf("failed to unmarshal record: %w", err)
	}

	slog.InfoContext(ctx, "new validationRecieved message batch from kafka", "size", len(batch))
	// Filter out the invalid messages.
	filteredBatch := filterValidationMessages(ctx, batch)

	// If all messages were invalid, return early.
	if len(filteredBatch) == 0 {
		slog.WarnContext(ctx, "no valid messages in the batch")
		return nil
	}

	// This will be used for the batch insertion.
	persistable := make([]store.ValidationMessage, len(filteredBatch))

	// Make messages ready for persistence. No errors must be swallowed here.
	for i, message := range filteredBatch {
		enriched, err := k.enrichValidationMessage(message)
		if err != nil {
			return fmt.Errorf("failed to enrich message: %w", err)
		}
		persistable[i] = enriched
	}

	// Store in database.
	insertedCount, err := k.db.InsertValidationMessagesIfNotExist(ctx, persistable)
	if err != nil {
		return fmt.Errorf("failed to persist batch: %w", err)
	}

	slog.InfoContext(ctx, "successfully inserted the batch in the database",
		"insertedCount", insertedCount, "totalCount", len(filteredBatch))
	return nil
}

// enrichValidationMessage enriches the given validationReceived message by assigning required
// fields. It is idempotent as Ingestor is designed to handle duplicate messages.
func (k KafkaRecordHandler) enrichValidationMessage(
	message store.ValidationMessagePayload,
) (store.ValidationMessage, error) {
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
		MasterKey:     masterKey,
		LedgerIndex:   int64(li),
		Payload:       messageBytes,
		HeimTimestamp: store.Timestamp(unixSigningTimeMs),
	}, nil
}
