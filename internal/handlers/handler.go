package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/xrpscan/heimdall-ingestor/pkg/kafkaesque"
)

// KafkaRecordHandler encapsulates methods required to handle/process Kafka records/messages.
type KafkaRecordHandler struct{}

// NewKafkaRecordHandler returns a new KafkaRecordHandler instance.
func NewKafkaRecordHandler() KafkaRecordHandler {
	return KafkaRecordHandler{}
}

// HandleValidationsMessageBatch processes a batch (or list) of xrpld's validationReceived messages.
func (k KafkaRecordHandler) HandleValidationsMessageBatch(
	ctx context.Context, record kafkaesque.Record,
) error {
	// Unmarshal to processable type.
	var batch []MessageValidationReceived
	if err := json.Unmarshal(record.Payload, &batch); err != nil {
		return fmt.Errorf("failed to unmarshal record: %w", err)
	}

	// Filter out the invalid messages.
	filteredBatch := filterValidationMessages(ctx, batch)

	// If all messages were invalid, return early.
	if len(filteredBatch) == 0 {
		slog.WarnContext(ctx, "no valid messages in the batch")
		return nil
	}

	// Process each message. No errors must be swallowed here.
	for _, message := range filteredBatch {
		if err := k.handleValidationsMessage(ctx, message); err != nil {
			return fmt.Errorf("failed to process message: %w", err)
		}
	}

	return nil
}

// handleValidationsMessage processes a single validationReceived message.
//
// It is idempotent as Ingestor is designed to handle duplicate messages.
func (k KafkaRecordHandler) handleValidationsMessage(
	ctx context.Context, message MessageValidationReceived,
) error {
	// If master key is absent, the validation public key can be assumed to be the master key.
	if message.MasterKey == "" {
		message.MasterKey = message.ValidationPublicKey
	}

	// signing_time is in XRPL epoch format.
	message.HeimTimestamp = xrplEpochToUnixEpoch(message.SigningTime)

	// TODO: Store in database.
	return nil
}
