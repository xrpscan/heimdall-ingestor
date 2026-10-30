package store

import (
	"context"
	"time"
)

// Client for the application's storage layer.
type Client interface {
	// InsertValidationMessagesIfNotExist inserts multiple entries in the validations table.
	// If any of the given messages already exist in the table, they are skipped without errors.
	InsertValidationMessagesIfNotExist(
		ctx context.Context, messages []ValidationMessage,
	) (int64, error)

	// InsertLedgerMessagesIfNotExist inserts multiple entries in the ledger table.
	// If any of the given messages already exist in the table, they are skipped without errors.
	InsertLedgerMessagesIfNotExist(
		ctx context.Context, messages []LedgerMessage,
	) (int64, error)

	// UpsertValidatorManifests upserts multiple entries in the validator_manifests table.
	//
	// Note that the provided value for the IsUNL field is ignored in this call because no callers
	// required it at the time of writing.
	//
	// Also, the CreatedAt and UpdatedAt fields are auto-populated. The provided values get ignored.
	UpsertValidatorManifests(ctx context.Context, manifests []ValidatorManifest) (int64, error)

	// UpdateUNLValidators updates the validator_manifests table so that the rows with the given
	// master keys have is_unl set to TRUE and all remaining rows have it set to FALSE.
	UpdateUNLValidators(ctx context.Context, masterKeys []string) error

	// GetAllValidatorAgreementRateBuckets retrieves agreement rates for all validators
	// (active within the given time period), as per the given time bucket.
	GetAllValidatorAgreementRateBuckets(
		ctx context.Context, options GetAllValidatorAgreementRateBucketsOptions,
	) (AgreementRatesData, error)

	// GetValidatorAgreementRateBuckets retrieves agreement rates for a single validator
	// (active within the given time period), as per the given time bucket.
	GetValidatorAgreementRateBuckets(
		ctx context.Context, options GetValidatorAgreementRateBucketsOptions,
	) (AgreementRateData, error)

	// GetLedgerCloseIntervals retrieves ledger close intervals (time between consecutive ledgers)
	// for all ledgers within the given time range.
	GetLedgerCloseIntervals(
		ctx context.Context, startTime, endTime time.Time,
	) ([]LedgerCloseInterval, error)
}
