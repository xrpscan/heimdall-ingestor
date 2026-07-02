package proc

import (
	"context"
	"fmt"
	"log/slog"
)

// ProcessMessageBatch processes the given batch of messages.
// It returns an error if even a single message fails to process.
func ProcessMessageBatch(ctx context.Context, batch []MessageValidationReceived) error {
	for i, message := range batch {
		if err := processOneMessage(ctx, message); err != nil {
			return fmt.Errorf("failed to process message at index %d: %w", i, err)
		}
	}

	return nil
}

func processOneMessage(ctx context.Context, message MessageValidationReceived) error {
	if message.MasterKey == "" && message.ValidationPublicKey == "" {
		return fmt.Errorf("both master-key and validation-public-key are empty")
	}

	// Master key is needed to be non-nil.
	if message.MasterKey == "" {
		slog.InfoContext(ctx, "using validation-public-key value for the master key",
			"validationPublicKey", message.ValidationPublicKey)
		message.MasterKey = message.ValidationPublicKey
	}

	// Convert to unix epoch time for general querying.
	message.HeimUnixSigningTime = xrplEpochToUnixEpoch(message.SigningTime)

	// TODO: Validator domain.

	_ = message
	return nil
}
