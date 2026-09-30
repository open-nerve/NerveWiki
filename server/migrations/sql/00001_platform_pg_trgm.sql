-- Trigram search over page titles and bodies (v0.1 design 7.1). The extension
-- is a database capability, owned by no module.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- +goose Down
DROP EXTENSION IF EXISTS pg_trgm;
