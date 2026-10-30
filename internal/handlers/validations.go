package handlers

import (
	"fmt"
)

// checkValidationBatchItem returns an error if the given validation batch item is invalid.
func checkValidationBatchItem(item validationBatchItem) error {
	message := item.Message

	if message.MasterKey == "" && message.ValidationPublicKey == "" {
		return fmt.Errorf("both master_key and validation_public_key are empty")
	}

	// Verify ledger_index.
	parsed, err := message.LedgerIndexParsed()
	if err != nil {
		return fmt.Errorf("ledger_index is invalid: %w", err)
	}
	if parsed <= 0 {
		return fmt.Errorf("ledger_index must be positive, got: %d", parsed)
	}

	if message.LedgerHash == "" {
		return fmt.Errorf("ledger_hash is empty")
	}

	if item.CreatedAt < 1_000_000_000_000 {
		return fmt.Errorf("createdAt seems to be in seconds, not milliseconds: %d", item.CreatedAt)
	}

	return nil
}

// checkLedgerBatchItem returns an error if the given ledger batch item is invalid.
func checkLedgerBatchItem(item ledgerBatchItem) error {
	message := item.Message

	// Verify ledger_index.
	parsed, err := message.LedgerIndexParsed()
	if err != nil {
		return fmt.Errorf("ledger_index is invalid: %w", err)
	}
	if parsed <= 0 {
		return fmt.Errorf("ledger_index must be positive, got: %d", parsed)
	}

	if message.LedgerHash == "" {
		return fmt.Errorf("ledger_hash is empty")
	}

	if item.CreatedAt < 1_000_000_000_000 {
		return fmt.Errorf("createdAt seems to be in seconds, not milliseconds: %d", item.CreatedAt)
	}

	return nil
}
