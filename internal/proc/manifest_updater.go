package proc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/xrpscan/heimdall-ingestor/internal/store"
	"github.com/xrpscan/heimdall-ingestor/pkg/xrpld"
)

const (
	// Max number of parallel xrpld invocations allowed.
	maxXRPLDParallelInvocation = 20
)

// ManifestUpdater asynchronously updates validator manifests in the database.
type ManifestUpdater struct {
	database    store.Client
	xrp         xrpld.Interface
	httpClient  *http.Client
	runInterval time.Duration
	maxAge      time.Duration

	mutex       sync.RWMutex
	lastUpdated map[string]time.Time
}

// NewManifestUpdater returns a new ManifestUpdater instance.
//
// It runs after every given runInterval and updates registered validators once their last-update
// time is older than the given maxAge.
func NewManifestUpdater(
	db store.Client, xrp xrpld.Interface, runInterval, maxAge time.Duration,
) *ManifestUpdater {
	return &ManifestUpdater{
		database:    db,
		xrp:         xrp,
		httpClient:  &http.Client{},
		runInterval: runInterval,
		maxAge:      maxAge,
		mutex:       sync.RWMutex{},
		lastUpdated: map[string]time.Time{},
	}
}

// Register a validator for periodic manifest updates.
func (m *ManifestUpdater) Register(masterKey string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, exists := m.lastUpdated[masterKey]; !exists {
		m.lastUpdated[masterKey] = time.Time{}
	}
}

// Start updating the manifests. This is a blocking call and returns only once the context expires.
func (m *ManifestUpdater) Start(ctx context.Context) {
	// Ticker for periodic auto-flushing.
	ticker := time.NewTicker(m.runInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "context expired, returning from manifest updater loop")
			return
		case <-ticker.C:
			// Create a new context for the operation with timeout as the run-interval.
			// This makes sure this call does not eat into the next iteration's time.
			ktx, cancel := context.WithTimeout(ctx, m.runInterval)
			if err := m.updateValidators(ktx); err != nil {
				slog.ErrorContext(ctx, "failed to update validator manifests", "error", err)
			}
			cancel()
		}
	}
}

// updateValidators fetches manifests for all stale validators and updates them in the database.
// The lastUpdate time of the successfully updated validators is refreshed to they don't get picked
// up wastefully.
//
// TODO: If a validator's GetManifest calls keeps failing, its lastUpdated time is never updated,
// and so it keeps getting picked up in every iteration. There should be a mechanism to filter out
// such validators.
func (m *ManifestUpdater) updateValidators(ctx context.Context) error {
	// Only the stale validators will be updated.
	stale := m.getStaleValidators()
	if len(stale) == 0 {
		slog.InfoContext(ctx, "no stale validators, skipping manifest update")
		return nil
	}

	// Fetch all manifests.
	manifests := m.getManifestsFromXRPLD(ctx, stale)

	// Collect domains for verification.
	domains := make([]string, 0, len(manifests))
	for _, m := range manifests {
		if m.Domain != "" {
			domains = append(domains, m.Domain)
		}
	}

	// Fetch all validator TOML files for verification.
	tomls := m.getAllValidatorTOML(ctx, domains)

	// Validators to be unregistered and mark updated locally.
	doneValidators := make([]string, 0, len(stale))
	// Arguments for the database update call.
	databaseArgs := make([]store.ValidatorManifest, 0, len(stale))

	// Loop to collect xrpld results.
	for _, manifest := range manifests {
		if manifest.Domain == "" {
			slog.WarnContext(ctx, "domain is empty for validator", "masterKey", manifest.MasterKey)
		}

		// Mark validator manifest as updated.
		doneValidators = append(doneValidators, manifest.MasterKey)

		// Extract master key from the validator toml file.
		var keyAsPerToml string
		if toml := tomls[manifest.Domain]; toml != nil {
			keyAsPerToml = toml.Validators[0].PublicKey
		}

		// Validator domain verification.
		verified := keyAsPerToml != "" && keyAsPerToml == manifest.MasterKey
		if !verified {
			slog.InfoContext(ctx, "validator domain is unverified", "domain", manifest.Domain,
				"masterKey", manifest.MasterKey, "masterKeyInToml", keyAsPerToml)
		} else {
			slog.InfoContext(ctx, "successfully verified validator domain",
				"domain", manifest.Domain, "masterKey", manifest.MasterKey,
				"masterKeyInToml", keyAsPerToml)
		}

		databaseArgs = append(databaseArgs, store.ValidatorManifest{
			MasterKey:      manifest.MasterKey,
			Domain:         manifest.Domain,
			DomainVerified: verified,
		})
	}

	// Update manifests in the database.
	affected, err := m.database.UpsertValidatorManifests(ctx, databaseArgs)
	if err != nil {
		return fmt.Errorf("error in UpsertValidatorManifests database call: %w", err)
	}

	// Mark the validators updated locally so they don't get picked up in the next iteration.
	m.markValidatorsUpdated(doneValidators)

	slog.InfoContext(ctx, "successfully updated validator manifests",
		"attemptedCount", len(stale), "affectedCount", affected)
	return nil
}

// getManifestsFromXRPLD invokes xrpld to fetch manifests for all given validators.
//
// The returned list may not contain an equal number of items as some calls may error.
func (m *ManifestUpdater) getManifestsFromXRPLD(
	ctx context.Context, mKeys []string,
) []xrpld.ManifestDetails {
	// For controlled xrpld invocation.
	semaphore := make(chan struct{}, maxXRPLDParallelInvocation)
	defer close(semaphore)

	// For reading Manifest call results from the goroutines.
	// We cannot do a defer-close for this channel as it may lead to a send-on-close panic.
	// It will just be garbage-collected when the method returns, no explicit close() is required.
	xrpResults := make(chan *xrpld.ManifestDetails, len(mKeys))

	// Loop to invoke xrpld for fetching manifests.
	for _, mKey := range mKeys {
		// Wait for an empty slot before launching a worker -- while respecting context.
		select {
		case <-ctx.Done():
			return nil
		case semaphore <- struct{}{}:
		}

		// Launch worker.
		go func() {
			// Release a slot for the next worker.
			defer func() { <-semaphore }()

			// Network call.
			details, err := m.xrp.GetManifest(ctx, mKey)
			if err != nil {
				xrpResults <- nil
				slog.ErrorContext(ctx, "error in GetManifest call", "masterKey", mKey, "error", err)
				return
			}

			// Make sure the master key is exactly the same so the channel reader is not confused.
			details.MasterKey = mKey
			xrpResults <- &details
		}()
	}

	// The final list to return.
	manifests := make([]xrpld.ManifestDetails, 0, len(mKeys))

	// Loop to collect xrpld results while respecting context.
	for range mKeys {
		select {
		case <-ctx.Done():
			return manifests
		case manifest := <-xrpResults:
			if manifest != nil {
				manifests = append(manifests, *manifest)
			}
		}
	}

	return manifests
}

// getStaleValidators returns the registered validators (master keys) that either have never been
// updated or were updated too long ago.
func (m *ManifestUpdater) getStaleValidators() []string {
	var stale []string
	// To check if lastUpdated is too old.
	now := time.Now()

	// This method is read-only.
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Loop over all registered validator master keys to find the stale ones.
	for mKey, updatedAt := range m.lastUpdated {
		// If lastUpdatedAt + maxAge is before current time, it is stale.
		if updatedAt.Add(m.maxAge).Before(now) {
			stale = append(stale, mKey)
		}
	}

	return stale
}

// markValidatorsUpdated marks the lastUpdated time of the given validators to the current time so
// they don't get picked up wastefully.
func (m *ManifestUpdater) markValidatorsUpdated(mKeys []string) {
	// To set the update timing.
	now := time.Now()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, mKey := range mKeys {
		m.lastUpdated[mKey] = now
	}
}

// getAllValidatorTOML invokes [getValidatorTOML] in parallel for the all given domains and returns
// a mapping of domain to TOML. For the call fails for a validator, their TOML is nil.
func (m *ManifestUpdater) getAllValidatorTOML(
	ctx context.Context, domains []string,
) map[string]*validatorTOML {
	results := make(chan func() (string, *validatorTOML, error), len(domains))

	// Parallel calls.
	for _, d := range domains {
		go func() {
			toml, err := m.getValidatorTOML(ctx, d)
			if err != nil {
				results <- func() (string, *validatorTOML, error) { return d, nil, err }
			} else {
				results <- func() (string, *validatorTOML, error) { return d, &toml, nil }
			}
		}()
	}

	// Result collection.
	mapping := map[string]*validatorTOML{}
	for range domains {
		d, toml, err := (<-results)()
		if err != nil {
			slog.ErrorContext(ctx, "toml call failed for a validator", "domain", d, "error", err)
			continue
		}

		mapping[d] = toml
	}

	slog.InfoContext(ctx, "successfully fetched toml files for validators", "count", len(mapping))
	return mapping
}

// getValidatorTOML downloads the TOML file present at https://<domain>/.well-known/xrp-ledger.toml,
// parses it and returns the data.
func (m *ManifestUpdater) getValidatorTOML(
	ctx context.Context, domain string,
) (validatorTOML, error) {
	domain = strings.TrimPrefix(domain, "https://")

	endpoint, err := url.JoinPath("https://", domain, ".well-known/xrp-ledger.toml")
	if err != nil {
		return validatorTOML{}, fmt.Errorf("failed to create endpoint url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return validatorTOML{}, fmt.Errorf("failed to create http request: %w", err)
	}

	// Execute request.
	response, err := m.httpClient.Do(req)
	if err != nil {
		return validatorTOML{}, fmt.Errorf("failed to execute http request: %w", err)
	}

	defer func() { _ = response.Body.Close() }()

	// Handle non-success responses.
	if response.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
		return validatorTOML{}, fmt.Errorf("unexpected response status code: %d, body: %s",
			response.StatusCode, string(body))
	}

	// Read the toml data.
	tomlBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return validatorTOML{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var target validatorTOML
	if err := toml.Unmarshal(tomlBytes, &target); err != nil {
		return validatorTOML{}, fmt.Errorf("failed to unmarshal validator toml: %w, bytes: %s", err, string(tomlBytes))
	}

	if len(target.Validators) == 0 || target.Validators[0].PublicKey == "" {
		return validatorTOML{}, fmt.Errorf("toml data seems to be empty")
	}

	return target, nil
}
