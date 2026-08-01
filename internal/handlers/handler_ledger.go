package handlers

import (
	"context"
)

func (k KafkaRecordHandler) handleLedgerMessageBatch(
	ctx context.Context, batch []ledgerBatchItem,
) error {
	return nil
}
