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
	opts KafkaRecordHandlerOptions
}

// KafkaRecordHandlerOptions encapsulates all dependencies required by the [KafkaRecordHandler].
type KafkaRecordHandlerOptions struct {
	// Database is required so each incoming message can be persisted.
	Database store.Client

	// ValidationTopic is the Kafka topic from which to read validationReceived messages.
	ValidationTopic string
	// LedgerTopic is the Kafka topic from which to read ledgerClosed messages.
	LedgerTopic string

	// Optional dependency. A function that can be wired up with the ManifestUpdater.
	ValidatorManifestUpdaterFunc func(masterKey string)
}

// NewKafkaRecordHandler returns a new KafkaRecordHandler instance.
func NewKafkaRecordHandler(opts KafkaRecordHandlerOptions) KafkaRecordHandler {
	// Since this is an optional dependency.
	if opts.ValidatorManifestUpdaterFunc == nil {
		opts.ValidatorManifestUpdaterFunc = func(masterKey string) {}
	}

	return KafkaRecordHandler{opts: opts}
}

// HandleBatch processes a batch (or list) as coming from the Observer.
func (k KafkaRecordHandler) HandleBatch(
	ctx context.Context, topic string, record kafkaesque.Record,
) error {
	ctx = logger.AddContextValue(ctx, "topic", topic)
	slog.InfoContext(ctx, "new batch from kafka arrived")

	switch topic {
	case k.opts.ValidationTopic:
		var validationBatch []validationBatchItem
		if err := json.Unmarshal(record.Payload, &validationBatch); err != nil {
			return fmt.Errorf("failed to unmarshal validation message batch: %w", err)
		}

		if err := k.handleValidationMessageBatch(ctx, validationBatch); err != nil {
			return fmt.Errorf("failed to process validation message batch: %w", err)
		}
	case k.opts.LedgerTopic:
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
