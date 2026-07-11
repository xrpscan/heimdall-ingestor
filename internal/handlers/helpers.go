package handlers

import (
	"context"
	"log/slog"

	"github.com/xrpscan/heimdall-ingestor/internal/store"
)

// xrplEpochToUnixEpoch converts XPRL epoch time (January 1, 2000)
// to Unix epoch time (January 1, 1970).
func xrplEpochToUnixEpoch(xrplEpoch uint64) int64 {
	const offset = 946684800
	return int64(xrplEpoch) + offset
}

// filterValidationMessages validates each message in the given batch and returns a slice of only
// the valid messages.
//
// It accepts context only for context-aware logging.
func filterValidationMessages(
	ctx context.Context, batch []store.ValidationMessagePayload,
) []store.ValidationMessagePayload {
	filteredBatch := make([]store.ValidationMessagePayload, 0, len(batch))

	// Filter out the invalid messages from the batch.
	for _, msg := range batch {
		if err := checkValidationReceivedMessage(msg); err != nil {
			slog.ErrorContext(ctx, "validationReceived message is invalid", "error", err)
		} else {
			filteredBatch = append(filteredBatch, msg)
		}
	}

	slog.InfoContext(ctx, "validation check complete for validationReceived messages",
		"valid", len(filteredBatch), "invalid", len(batch)-len(filteredBatch))

	return filteredBatch
}
