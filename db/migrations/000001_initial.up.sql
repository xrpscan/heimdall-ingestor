BEGIN;

CREATE TABLE validations (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    heim_timestamp  TIMESTAMP WITH TIME ZONE NOT NULL,

    master_key      TEXT NOT NULL,
    ledger_index    BIGINT NOT NULL,
    payload         JSONB NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

COMMIT;
