package handlers

import (
	"fmt"
	"strconv"

	"github.com/xrpscan/heimdall-ingestor/internal/store"
)

// checkValidationReceivedMessage returns an error if the given message is invalid.
func checkValidationReceivedMessage(message store.ValidationMessagePayload) error {
	if message.MasterKey == "" && message.ValidationPublicKey == "" {
		return fmt.Errorf("both master_key and validation_public_key are empty")
	}

	if message.LedgerIndex == "" {
		return fmt.Errorf("ledger_index is empty")
	}

	if _, err := strconv.ParseUint(message.LedgerIndex, 10, 64); err != nil {
		return fmt.Errorf("ledger_index is not a positive int '%s': %w", message.LedgerIndex, err)
	}

	return nil
}
