package kafkaesque

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// Consumer represents a generic Kafka Consumer.
type Consumer interface {
	ConsumeWithRetry(ctx context.Context)
	Close(ctx context.Context) error
}

// FranzGoConsumer implements [Consumer] using github.com/twmb/franz-go.
type FranzGoConsumer struct {
	client *kgo.Client
	params ConsumerParams
}

// ConsumerParams contain params required to create a Kafka Consumer.
type ConsumerParams struct {
	Brokers    []string
	Username   string
	Password   string
	CACertPath string

	Topic           string
	ConsumerGroupID string
	Handler         RecordHandlerFunc

	MaxRetryCount int
	RetryInterval time.Duration

	Logger Logger
}

// NewFranzGoConsumer returns a new FranzGoConsumer instance.
func NewFranzGoConsumer(ctx context.Context, params ConsumerParams) (*FranzGoConsumer, error) {
	// Logger is optional.
	if params.Logger == nil {
		params.Logger = noopLogger{}
	}

	// Common config.
	opts := []kgo.Opt{
		kgo.SeedBrokers(params.Brokers...),
		kgo.ConsumerGroup(params.ConsumerGroupID),
		kgo.ConsumeTopics(params.Topic),
		// If Kafka finds no offsets for this consumer group, it will be reset to the start.
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		// Autocommit only the explicitly marked records.
		kgo.AutoCommitMarks(),
	}

	// Enable SCRAM-SHA and TLS if username and password provided.
	if params.Username != "" && params.Password != "" {
		params.Logger.InfoContext(ctx,
			"username and password are present, SCRAM-SHA and TLS will be enabled")

		// Add SCRAM-SHA option.
		mechanism := scram.Auth{User: params.Username, Pass: params.Password}.AsSha512Mechanism()
		opts = append(opts, kgo.SASL(mechanism))

		// Add TLS option.
		dialer, err := createTLSDialer(params.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS dialer: %w", err)
		}

		opts = append(opts, kgo.Dialer(dialer.DialContext))
	} else {
		params.Logger.InfoContext(ctx,
			"username and password are absent, SCRAM-SHA and TLS will be disabled")
	}

	// Create connection.
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka client: %w", err)
	}

	// Verify connection.
	if err := cl.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping kafka cluster: %w", err)
	}

	return &FranzGoConsumer{client: cl, params: params}, nil
}

// ConsumeWithRetry starts an infinite loop to consume messages from Kafka. The Handler is
// called for each consumed message. If the Handler returns an error, it is called again as per
// the MaxRetryCount and RetryInterval configs.
//
// If retries are exhausted but the record processing never succeeds, its offset will be committed.
func (f *FranzGoConsumer) ConsumeWithRetry(ctx context.Context) {
	log := f.params.Logger

	for {
		select {
		case <-ctx.Done():
			return // End consumer loop.
		default:
		}

		// Get messages from brokers.
		fetches := f.client.PollFetches(ctx)

		// Log errors.
		for _, e := range fetches.Errors() {
			log.ErrorContext(ctx, "error in PollFetches call",
				"error", e.Err, "topic", e.Topic, "partition", e.Partition)

			// Context cancelation or client closure (through Close method or otherwise).
			if isFatalConsumerError(e.Err) {
				return // End consumer loop.
			}
		}

		fetches.EachRecord(func(r *kgo.Record) {
			record := newRecord(r)
			var succeeded bool

			// Keep processing the same record until it succeeds.
			for i := 0; i < f.params.MaxRetryCount; i++ {
				select {
				case <-ctx.Done():
					return // If context is done, skip processing and end the EachRecord call asap.
				default:
				}

				// Process record and set hasFailed to true on error.
				if err := f.params.Handler(ctx, record); err != nil {
					log.ErrorContext(ctx, "failed to process record", "error", err, "count", i+1,
						"topic", r.Topic, "partition", r.Partition, "offset", r.Offset)

					// Record processing failed. Wait while respecting context and reprocess.
					select {
					case <-time.After(f.params.RetryInterval):
						continue
					case <-ctx.Done():
						return // Stop retrying on context expiry.
					}
				}

				// Record processing was successful. Move to next record.
				log.InfoContext(ctx, "successfully processed kafka record",
					"topic", r.Topic, "partition", r.Partition, "offset", r.Offset)
				succeeded = true
				break
			}

			// Logs should mention that the retries were exhausted and the offset will be committed.
			if !succeeded {
				log.ErrorContext(ctx,
					"failed to process record, retries exhausted, committing offset",
					"topic", r.Topic, "partition", r.Partition, "offset", r.Offset)
			}

			// Mark this record for offset commit.
			// The AutoCommitMarks option in the client takes care of the actual commit operation.
			f.client.MarkCommitRecords(r)
		})
	}
}

func (f *FranzGoConsumer) Close(_ context.Context) error {
	f.client.CloseAllowingRebalance()
	return nil
}
