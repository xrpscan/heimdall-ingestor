package handlers

import (
	"fmt"
	"strconv"
)

// checkValidationBatchItem returns an error if the given batch item is invalid.
func checkValidationBatchItem(item validationBatchItem) error {
	message := item.Message

	if message.MasterKey == "" && message.ValidationPublicKey == "" {
		return fmt.Errorf("both master_key and validation_public_key are empty")
	}

	if message.LedgerIndex == "" {
		return fmt.Errorf("ledger_index is empty")
	}

	if _, err := strconv.ParseUint(message.LedgerIndex, 10, 64); err != nil {
		return fmt.Errorf("ledger_index is not a positive int '%s': %w", message.LedgerIndex, err)
	}

	if item.CreatedAt < 1_000_000_000_000 {
		return fmt.Errorf("createdAt seems to be in seconds, not milliseconds: %d", item.CreatedAt)
	}

	return nil
}
