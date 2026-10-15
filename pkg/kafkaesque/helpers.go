package kafkaesque

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// createTLSDialer returns a TLS dialer that has the given CA Cert in its truststore.
func createTLSDialer(caCertPath string) (*tls.Dialer, error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		Config: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// If no CA cert provided, go ahead with just the system CAs.
	if strings.TrimSpace(caCertPath) == "" {
		return dialer, nil
	}

	// Private CA cert is provided. Load it.
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ca-cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("ca-cert failed to parse")
	}

	dialer.Config.RootCAs = caCertPool
	return dialer, nil
}

// newRecord converts franz-go's Record type to this package's Record type.
func newRecord(r *kgo.Record) Record {
	// Translate headers into a convenient map.
	headers := map[string]string{}
	for _, header := range r.Headers {
		headers[header.Key] = string(header.Value)
	}

	return Record{Payload: r.Value, Headers: headers}
}

// isFatalConsumerError returns true if the given error implies the consumer is stopped
// or should be stopped.
func isFatalConsumerError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, kgo.ErrClientClosed)
}
