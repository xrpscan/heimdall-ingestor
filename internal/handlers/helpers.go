package handlers

import (
	"context"
	"log/slog"
)

// xrplEpochToUnixEpoch converts XPRL epoch time (January 1, 2000)
// to Unix epoch time (January 1, 1970).
func xrplEpochToUnixEpoch(xrplEpoch uint64) int64 {
	const offset = 946684800
	return int64(xrplEpoch) + offset
}

// filterValidationBatch validates each item in the given batch and returns a slice of only
// the valid items.
//
// It accepts context only for context-aware logging.
func filterValidationBatch(
	ctx context.Context, batch []validationBatchItem,
) []validationBatchItem {
	filteredBatch := make([]validationBatchItem, 0, len(batch))

	// Filter out the invalid items from the batch.
	for _, item := range batch {
		if err := checkValidationBatchItem(item); err != nil {
			slog.ErrorContext(ctx, "validation batch item message is invalid", "error", err)
		} else {
			filteredBatch = append(filteredBatch, item)
		}
	}

	slog.InfoContext(ctx, "validation check complete for validation batch item",
		"valid", len(filteredBatch), "invalid", len(batch)-len(filteredBatch))

	return filteredBatch
}
