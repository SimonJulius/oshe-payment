-- +goose Up
-- Infrastructure only. Domain schemas belong in subsequent reviewed migrations.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
CREATE SCHEMA platform;
COMMENT ON SCHEMA platform IS 'Shared infrastructure owned by the platform package';

-- +goose Down
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
-- RESTRICT deliberately prevents accidental deletion once the schema has dependents.
DROP SCHEMA platform RESTRICT;
