-- api_tokens: personal access tokens (M1 design 4, M1/P3 design 3.2). Only
-- the token's hash is stored; the token itself is shown once, at creation.

-- +goose Up
CREATE TABLE api_tokens (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    -- SHA-256 of the whole token, nwk_pat_ and 43 characters: a random
    -- 256-bit secret needs no slow hash.
    token_hash bytea NOT NULL CONSTRAINT api_tokens_token_hash_key UNIQUE
        CONSTRAINT api_tokens_token_hash_check CHECK (octet_length(token_hash) = 32),
    -- The domain trims the name and rejects control characters.
    name varchar(100) NOT NULL CONSTRAINT api_tokens_name_check CHECK (name <> ''),
    -- NULL: never expires.
    expires_at timestamptz CONSTRAINT api_tokens_expires_at_check CHECK (expires_at > created_at),
    -- Written at most once a minute; using a token leaves updated_at alone.
    last_used_at timestamptz,
    -- Revoking is a soft delete (v0.1 design 6.1).
    revoked_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
-- The account's list: its unrevoked tokens, newest first.
CREATE INDEX api_tokens_user_id_created_at_idx ON api_tokens (user_id, created_at DESC, id DESC)
    WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE api_tokens;
