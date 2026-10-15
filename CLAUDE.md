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
- `internal/rest/` — HTTP server with middleware (health endpoint only)
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
- The `internal/` packages are not importable externally; reusable code goes in `pkg/`.
