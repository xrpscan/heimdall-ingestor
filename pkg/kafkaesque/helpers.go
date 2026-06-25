package kafkaesque

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// createTLSDialer returns a TLS dialer that has the given CA Cert in its truststore.
func createTLSDialer(caCertPath string) (*tls.Dialer, error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		Config:    &tls.Config{},
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
