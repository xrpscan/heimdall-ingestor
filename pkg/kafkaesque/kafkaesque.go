package kafkaesque

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// Client represents a generic Kafka client.
type Client interface {
	Close(ctx context.Context) error
}

// FranzGoClient implements [Client] using franz-go.
type FranzGoClient struct {
	client *kgo.Client
}

// ClientParams contain params required to create a Kafka client.
type ClientParams struct {
	Brokers            []string
	Username, Password string
	CACertPath         string

	Logger Logger
}

// NewFranzGoClient returns a new FranzGoClient instance.
func NewFranzGoClient(ctx context.Context, params ClientParams) (*FranzGoClient, error) {
	// Logger is optional.
	if params.Logger == nil {
		params.Logger = noopLogger{}
	}

	// Common config.
	opts := []kgo.Opt{
		kgo.SeedBrokers(params.Brokers...),
		kgo.RecordPartitioner(kgo.RoundRobinPartitioner()),
		// franz-go already has sensible defaults for configs like RetryCount and RetryBackoff.
	}

	// Enable SCRAM-SHA and TLS if username and password provided.
	if params.Username != "" && params.Password != "" {
		params.Logger.Info("username and password are present, SCRAM-SHA and TLS will be enabled")

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
		params.Logger.Info("username and password are absent, SCRAM-SHA and TLS will be disabled")
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

	return &FranzGoClient{client: cl}, nil
}

func (f *FranzGoClient) Close(_ context.Context) error {
	f.client.Close()
	return nil
}
