# CLAUDE.md

## Build / Test / Lint

```sh
make build          # go build -> bin/ingestor
make run            # tidy + build + run
make test           # go test -race ./...
make lint           # golangci-lint run ./...
make migrate-up     # run database migrations (requires migrate CLI)
make migrate-down   # rollback database migrations
```

The binary auto-runs migrations on startup, so `make migrate-up` is only needed for manual runs.

## Project Structure

- `cmd/ingestor/main.go` — entrypoint, wires all dependencies
- `internal/config/` — JSON config loading and validation
- `internal/handlers/` — Kafka message handlers (validation + ledger topics)
- `internal/proc/` — long-running processes (ManifestUpdater, UNLSyncer)
- `internal/store/` — PostgreSQL data access layer
- `internal/rest/` — HTTP server with middleware (health + chart APIs)
  - `handler_charts.go` — validator analytics endpoints (heatmap, rating, agreement rate, ledger intervals)
- `internal/logger/` — structured logging setup
- `pkg/kafkaesque/` — Kafka consumer abstraction (franz-go)
- `pkg/xrpld/` — XRPL node client (manifest RPCs, key encoding)
- `pkg/registry/` — service registry for graceful shutdown
- `pkg/httputils/` — HTTP response/error helpers
- `db/migrations/` — PostgreSQL migration files (golang-migrate format)

## Conventions

- Commit messages follow `type: description` format (e.g. `feat:`, `fix:`, `chore:`).
- Import ordering enforced by golangci-lint gci: stdlib, then project (`github.com/xrpscan/heimdall-ingestor`), then third-party.
- Config lives in `config/config.json` (gitignored). Copy `config/config.example.json` to get started.
- All config fields are validated on startup; see `internal/config/config.go` for requirements.
- Database tables: `validations`, `ledger`, `validator_manifests`, `agreements`. Agreement rows are computed automatically by a Postgres trigger on ledger insert.
- The `validator_manifests` table includes a `domain_verified` boolean that indicates whether the validator's domain ownership has been verified via `/.well-known/xrp-ledger.toml`.
- The `internal/` packages are not importable externally; reusable code goes in `pkg/`.

## REST API

The HTTP server exposes validator analytics endpoints in addition to health checks. All chart endpoints accept Unix timestamps in milliseconds for time parameters.

### Endpoints

- `GET /healthz` — health check
- `GET /v1/charts/validator-agreement-rate-heatmap` — heatmap of validator agreement rates over time
  - Query: `startTime`, `endTime` (ms), `bucket` (minute|hour|day|week|month|quarter|year|millennium), `unlOnly` (bool)
- `GET /v1/charts/{validatorMasterKey}/agreement-rate` — time series of agreement rate for a single validator
  - Query: `startTime`, `endTime` (ms), `bucket`
- `GET /v1/charts/validator-rating` — validator scores (80% agreement rate + 20% domain verification)
  - Query: `startTime`, `endTime` (ms), `unlOnly` (bool)
- `GET /v1/charts/ledger-close-interval` — time series of ledger close intervals
  - Query: `startTime`, `endTime` (ms)

Default time range: last 24 hours. Bucket size auto-calculated if not specified.
