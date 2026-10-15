-- Reverse of 000001_initial.up.sql. Drops everything in reverse creation order.

DROP TRIGGER trg_compute_agreement ON ledger;
DROP FUNCTION compute_agreement;
DROP TABLE agreements;

DROP TRIGGER trg_set_updated_at ON validator_manifests;
DROP TABLE validator_manifests;

DROP TABLE ledger;
DROP TABLE validations;

DROP FUNCTION set_updated_at;
