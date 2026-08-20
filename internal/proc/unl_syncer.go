package proc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/xrpscan/heimdall-ingestor/internal/store"
	"github.com/xrpscan/heimdall-ingestor/pkg/xrpld"
)

// UNLSyncer periodically polls https://unl.xrplf.org to stay in sync with the UNL of validators.
type UNLSyncer struct {
	database    store.Client
	httpClient  *http.Client
	runInterval time.Duration
}

// NewUNLSyncer returns a new instance of [UNLSyncer].
//
// The syncer will run after every given runInterval duration.
func NewUNLSyncer(database store.Client, runInterval time.Duration) *UNLSyncer {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return &UNLSyncer{database: database, httpClient: httpClient, runInterval: runInterval}
}

// Start the UNL syncing process. This is a blocking call which returns only once the given context
// expires. The Close method will not unblock this call.
func (u *UNLSyncer) Start(ctx context.Context) {
	// Ticker for periodic auto-flushing.
	ticker := time.NewTicker(u.runInterval)
	defer ticker.Stop()

	sync := func() {
		// Create a new context for the operation with timeout as the run-interval.
		// This makes sure this call does not eat into the next iteration's time.
		ktx, cancel := context.WithTimeout(ctx, u.runInterval)
		defer cancel()

		if err := u.sync(ktx); err != nil {
			slog.ErrorContext(ctx, "failed to sync unl", "error", err)
		} else {
			slog.InfoContext(ctx, "successfully updated unl validators in the database")
		}
	}

	// Call once at startup so UNL data is available quicker.
	sync()

	// Periodic caller.
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "context expired, returning from unl-syncer loop")
			return
		case <-ticker.C:
			sync()
		}
	}
}

// Close the process. This does not unblock the Start call.
func (u *UNLSyncer) Close() {
	u.httpClient.CloseIdleConnections()
}

func (u *UNLSyncer) sync(ctx context.Context) error {
	uResponse, err := u.invokeXRPLF(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch unl: %w", err)
	}

	// Decode the blob from base64.
	decodedBytes, err := base64.StdEncoding.DecodeString(uResponse.Blob)
	if err != nil {
		return fmt.Errorf("failed to decode unl blob: %w", err)
	}

	// Unmarshal the decoded blob to readable struct.
	var decodedBlob unlValidatorInfo
	if err := json.Unmarshal(decodedBytes, &decodedBlob); err != nil {
		return fmt.Errorf("failed to unmarshal decoded unl blob: %w", err)
	}

	var unlValidators []string

	// Now decode all keys from hex to xrpl base58.
	for _, validator := range decodedBlob.Validators {
		key, err := xrpld.EncodeNodePublicKey(validator.ValidationPublicKey)
		if err != nil {
			slog.ErrorContext(ctx, "failed to decode a master key", "error", err,
				"key", validator.ValidationPublicKey)
			continue
		}

		unlValidators = append(unlValidators, key)
	}

	slog.InfoContext(ctx, "successfully decoded unl validators", "count", len(unlValidators))
	// Update flags in the database.
	if err := u.database.UpdateUNLValidators(ctx, unlValidators); err != nil {
		return fmt.Errorf("failed to update unl validators in the database: %w", err)
	}

	return nil
}

func (u *UNLSyncer) invokeXRPLF(ctx context.Context) (unlResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, xrplFoundationURL, nil)
	if err != nil {
		return unlResponse{}, fmt.Errorf("failed to create http request: %w", err)
	}

	response, err := u.httpClient.Do(request)
	if err != nil {
		return unlResponse{}, fmt.Errorf("failed to execute http request: %w", err)
	}

	defer func() { _ = response.Body.Close() }()

	// Read the body as we may have to unmarshal it multiple times.
	// Here, io.LimitReader ensures that a maliciously large response
	// body is not loaded into memory. 1<<20 represents 1 MB.
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return unlResponse{}, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle error response.
	if response.StatusCode/100 != 2 {
		slog.ErrorContext(ctx, "xrplf returned non-2xx status code",
			"code", response.StatusCode, "body", stringUpto(string(responseBody), 1024))
		return unlResponse{},
			fmt.Errorf("xrplf returned non-2xx status code: %d", response.StatusCode)
	}

	var uResponse unlResponse
	if err := json.Unmarshal(responseBody, &uResponse); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal xrplf response",
			"error", err, "body", stringUpto(string(responseBody), 1024))
		return unlResponse{},
			fmt.Errorf("failed to unmarshal xrplf response: %w", err)
	}

	return uResponse, nil
}
