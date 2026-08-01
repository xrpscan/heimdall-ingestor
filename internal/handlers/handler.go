package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/xrpscan/heimdall-ingestor/internal/logger"
	"github.com/xrpscan/heimdall-ingestor/internal/store"
	"github.com/xrpscan/heimdall-ingestor/pkg/kafkaesque"
)

// KafkaRecordHandler encapsulates methods required to handle/process Kafka records/messages.
type KafkaRecordHandler struct {
	db          store.Client
	validnTopic string
	ledgerTopic string
}

// NewKafkaRecordHandler returns a new KafkaRecordHandler instance.
func NewKafkaRecordHandler(db store.Client, validnTopic, ledgerTopic string) KafkaRecordHandler {
	return KafkaRecordHandler{db: db, validnTopic: validnTopic, ledgerTopic: ledgerTopic}
}

// HandleBatch processes a batch (or list) as coming from the Observer.
func (k KafkaRecordHandler) HandleBatch(
	ctx context.Context, topic string, record kafkaesque.Record,
) error {
	ctx = logger.AddContextValue(ctx, "topic", topic)
	slog.InfoContext(ctx, "new batch from kafka arrived")

	switch topic {
	case k.validnTopic:
		var validationBatch []validationBatchItem
		if err := json.Unmarshal(record.Payload, &validationBatch); err != nil {
			return fmt.Errorf("failed to unmarshal validation message batch: %w", err)
		}

		if err := k.handleValidationMessageBatch(ctx, validationBatch); err != nil {
			return fmt.Errorf("failed to process validation message batch: %w", err)
		}
	case k.ledgerTopic:
		var ledgerBatch []ledgerBatchItem
		if err := json.Unmarshal(record.Payload, &ledgerBatch); err != nil {
			return fmt.Errorf("failed to unmarshal ledger message batch: %w", err)
		}

		if err := k.handleLedgerMessageBatch(ctx, ledgerBatch); err != nil {
			return fmt.Errorf("failed to process ledger message batch: %w", err)
		}
	default:
		return fmt.Errorf("unknown topic: %s", topic)
	}

	return nil
}
