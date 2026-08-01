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

CREATE TABLE ledger (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    ledger_index    BIGINT NOT NULL,
    ledger_hash     TEXT   NOT NULL,

    observer_created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    UNIQUE (ledger_index)
);
