# Heimdall Ingestor

## Introduction

Listens on two Kafka topics, one to receive validationReceived XRPLD messages and the other to receive the ledgerClosed messages.

The validationReceived messages are persisted in the validations table and the ledgerClosed messages are persisted in the ledger table. The table names mirror the XRPLD stream names. The Kafka consumer process also keeps track of all encountered validators.

A separate process called the Manifest Updater continuously fetches the manifests of all encountered validators and upserts them in the validator_manifests table.

Another separate process called the UNL Syncer periodically polls the XRPL foundation URL (https://unl.xrplf.org) to fetch the UNL validators and updates them in the validator_manifests table as well.

## How to Run

The ingestor consumes messages from Kafka topics and does not care how they get there. The standard producer is the [Heimdall Observer](https://github.com/xrpscan/heimdall-observer), but any producer that writes `validationReceived` and `ledgerClosed` messages in the expected format will work. The ingestor runs fine without any producer — it will simply idle until messages arrive.

### Prerequisites

- Go 1.26+
- PostgreSQL
- Access to a Kafka cluster
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI (optional, for manual migrations)

### Steps

1. **Create a config file** by copying the example and filling in your values:

   ```sh
   cp config/config.example.json config/config.json
   # Edit config/config.json with your database, Kafka, and XRPL settings.
   ```

2. **Run database migrations** (the binary runs migrations automatically on startup, but they can also be run manually):

   ```sh
   make migrate-up
   ```

3. **Build and run**:

   ```sh
   make build
   bin/ingestor
   ```

   Or in one step with linting and tidying:

   ```sh
   make run
   ```

   A custom config path can be passed with the `-config` flag:

   ```sh
   bin/ingestor -config path/to/config.json
   ```

### Docker

```sh
make image      # Build the container image.
make container  # Run the container (uses config/config.docker.json).
```

## Long Running Processes

The ingestor runs four concurrent processes. If any process encounters a fatal error, it cancels the root context and triggers a graceful shutdown of all other processes.

### 1. Kafka Consumer

Connects to the configured Kafka brokers and subscribes to two topics: one for `validationReceived` messages and one for `ledgerClosed` messages. Messages arrive in batches from the observer.

- **Validation messages** are unmarshalled and inserted into the `validations` table, deduplicated by `(master_key, ledger_index)`. Each new validator's master key is registered with the Manifest Updater for periodic manifest fetching.
- **Ledger messages** are inserted into the `ledger` table, deduplicated by `ledger_index`. A database trigger (`trg_compute_agreement`) fires on each insert and populates the `agreements` table by comparing each validator's vote against the canonical ledger hash.

If a message fails to process, it is retried up to `maxMessageRetryCount` times with a delay of `messageRetryIntervalMs` between attempts. If all retries are exhausted, the offset is committed and the consumer moves on.

Authentication is optional: if `username` and `password` are provided, the consumer enables SCRAM-SHA-512 and TLS using the CA certificate at `caCertPath`. Otherwise it connects without authentication.

### 2. Manifest Updater

Periodically fetches validator manifest data (domain, signing keys) from an XRPL node via the `manifest` RPC method. Validators are registered dynamically as they appear in incoming Kafka validation messages.

- Runs every `manifestUpdater.runIntervalSec` seconds.
- Only fetches manifests for validators whose last update is older than `manifestUpdater.maxAgeSec` seconds.
- Fetches are parallelized with a concurrency limit of 20 simultaneous XRPL requests.
- Results are upserted into the `validator_manifests` table.

### 3. UNL Syncer

Periodically polls the XRPL Foundation's validator list at https://unl.xrplf.org to determine which validators are on the recommended Unique Node List (UNL).

- Runs once immediately on startup, then every `unlSyncer.runIntervalSec` seconds.
- Decodes the base64 blob from the response, extracts validator public keys, and converts them from hex to XRPL base58 encoding.
- Updates the `is_unl` flag on matching rows in the `validator_manifests` table.

### 4. HTTP Server

Exposes a REST API for health checking. Currently serves a single endpoint:

- `GET /healthz` — returns `{"code": "OK"}` with a 200 status.

The server includes recovery, access logging, CORS, and body size limit (16 KB) middleware.

## Configuration

All configuration is provided via a JSON file (default: `config/config.json`). See `config/config.example.json` for a complete example. Every field below is required unless noted otherwise.

### `database`

| Field      | Description                              | Example          |
|------------|------------------------------------------|------------------|
| `addr`     | PostgreSQL host and port.                | `localhost:5432` |
| `username` | Database username.                       | `postgres`       |
| `password` | Database password.                       | `dev`            |
| `database` | Database name.                           | `heimdall`       |

### `httpServer`

| Field            | Description                                                                                           | Example          |
|------------------|-------------------------------------------------------------------------------------------------------|------------------|
| `addr`           | Address for the HTTP server to listen on.                                                             | `localhost:9983` |
| `allowedOrigins` | List of allowed CORS origins.                                                                         | `["*"]`          |
| `corsMaxAgeSec`  | Max age (in seconds) for the CORS preflight cache. See [MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Access-Control-Max-Age). | `86400`          |

### `kafka`

| Field                   | Description                                                                                     | Example                        |
|-------------------------|-------------------------------------------------------------------------------------------------|--------------------------------|
| `brokers`               | List of Kafka broker addresses.                                                                 | `["localhost:19095"]`          |
| `username`              | SASL username. Leave empty (with `password`) to disable authentication.                         | `""`                           |
| `password`              | SASL password. Leave empty (with `username`) to disable authentication.                         | `""`                           |
| `caCertPath`            | Path to CA certificate for TLS. Only used when username and password are set.                   | `/path/to/ca/cert`             |
| `validationsTopic`      | Kafka topic for `validationReceived` messages.                                                  | `your-validations-topic`       |
| `ledgerTopic`           | Kafka topic for `ledgerClosed` messages.                                                        | `your-ledger-topic`            |
| `consumerGroupID`       | Kafka consumer group ID.                                                                        | `something-group`              |
| `maxMessageRetryCount`  | Max number of times a failed message is retried before its offset is committed. Must be > 0.    | `100`                          |
| `messageRetryIntervalMs`| Delay in milliseconds between retries. Must be > 0.                                            | `100`                          |

### `logger`

| Field      | Description                                                        | Example                |
|------------|--------------------------------------------------------------------|------------------------|
| `filePath` | Path to the log file. Leave empty to log to stdout.                | `./logs/ingestor.log`  |
| `level`    | Log level (`debug`, `info`, `warn`, `error`).                      | `debug`                |
| `pretty`   | Whether to use pretty (human-readable) log formatting.             | `true`                 |

### `manifestUpdater`

| Field            | Description                                                                  | Example |
|------------------|------------------------------------------------------------------------------|---------|
| `runIntervalSec` | How often (in seconds) the manifest updater runs. Must be > 0.               | `60`    |
| `maxAgeSec`      | Max age (in seconds) before a validator's manifest is considered stale and re-fetched. Must be > 0. | `3600`  |

### `unlSyncer`

| Field            | Description                                                       | Example |
|------------------|-------------------------------------------------------------------|---------|
| `runIntervalSec` | How often (in seconds) the UNL syncer polls the XRPL Foundation. Must be > 0. | `300`   |

### `xrpl`

| Field  | Description                                                              | Example                    |
|--------|--------------------------------------------------------------------------|----------------------------|
| `addr` | URL of the XRPL node used for manifest lookups.                         | `https://xrplcluster.com`  |
