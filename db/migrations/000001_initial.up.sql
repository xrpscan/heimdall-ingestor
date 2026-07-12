BEGIN;

CREATE TABLE validations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    master_key      TEXT NOT NULL,
    ledger_index    BIGINT NOT NULL,
    payload         JSONB NOT NULL,

    heim_timestamp  TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    UNIQUE (master_key, ledger_index)
);

COMMIT;
