-- Reusable trigger function that sets updated_at to the current timestamp on row update.
-- Attach to any table with an updated_at column via a BEFORE UPDATE trigger.
CREATE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Stores xrpld validationReceived messages. Each row is one validator's vote for a ledger.
-- Deduplicated by (master_key, ledger_index).
CREATE TABLE validations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    master_key      TEXT    NOT NULL,
    ledger_index    BIGINT  NOT NULL,
    ledger_hash     TEXT    NOT NULL,
    payload         JSONB   NOT NULL,

    unix_signing_time   TIMESTAMP WITH TIME ZONE NOT NULL,
    observer_created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    UNIQUE (master_key, ledger_index)
);

-- Stores xrpld ledgerClosed messages. Each row is a closed ledger as seen by the observer.
-- Deduplicated by ledger_index.
CREATE TABLE ledger (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    ledger_index    BIGINT NOT NULL,
    ledger_hash     TEXT   NOT NULL,

    observer_created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    UNIQUE (ledger_index)
);

-- Caches validator manifest data fetched from xrpld. Keyed by master_key.
-- Upserted periodically by the ManifestUpdater process.
CREATE TABLE validator_manifests (
    master_key  TEXT    PRIMARY KEY,
    domain      TEXT,
    is_unl      BOOLEAN   NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Auto-update updated_at on validator_manifests row updates.
CREATE TRIGGER trg_set_updated_at
    BEFORE UPDATE ON validator_manifests
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Stores whether each validator's vote agreed with the canonical ledger hash.
-- Populated automatically by trg_compute_agreement when a new ledger row is inserted.
-- Deduplicated by (ledger_index, master_key).
CREATE TABLE agreements (
    master_key       TEXT       NOT NULL,
    ledger_index     BIGINT     NOT NULL,
    agreed           BOOLEAN    NOT NULL,
    ledger_closed_at TIMESTAMP  WITH TIME ZONE NOT NULL,

    UNIQUE (ledger_index, master_key)
);

-- Trigger function that runs after a ledger row is inserted. For every validation with the
-- same ledger_index, it inserts an agreement row recording whether the validator's hash matched
-- the canonical ledger hash and whether the vote arrived before the ledger closed.
CREATE FUNCTION compute_agreement()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO agreements (master_key, ledger_index, agreed, ledger_closed_at)
  SELECT
    v.master_key,
    v.ledger_index,
    (v.ledger_hash = NEW.ledger_hash AND v.unix_signing_time <= NEW.observer_created_at),
    NEW.observer_created_at
  FROM validations v
  WHERE v.ledger_index = NEW.ledger_index
  ON CONFLICT DO NOTHING;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Fire compute_agreement() whenever a new ledger is inserted.
CREATE TRIGGER trg_compute_agreement
AFTER INSERT ON ledger
FOR EACH ROW
EXECUTE FUNCTION compute_agreement();
