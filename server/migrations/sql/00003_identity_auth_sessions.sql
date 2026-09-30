-- auth_sessions: one row per sign-in (M1 design 4). Earlier generations of
-- the refresh token are not stored: their MAC tag identifies them.

-- +goose Up
CREATE TABLE auth_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    -- SHA-256 of the current generation's secret.
    token_hash bytea NOT NULL CONSTRAINT auth_sessions_token_hash_check CHECK (octet_length(token_hash) = 32),
    generation integer NOT NULL DEFAULT 0 CONSTRAINT auth_sessions_generation_check CHECK (generation >= 0),
    user_agent text NOT NULL DEFAULT '',
    ip inet,
    -- The sign-in time plus auth.session_ttl; refreshing never extends it.
    expires_at timestamptz NOT NULL,
    last_refreshed_at timestamptz,
    revoked_at timestamptz,
    revoke_reason varchar(20) CONSTRAINT auth_sessions_revoke_reason_check CHECK (revoke_reason IN
        ('logout', 'password_changed', 'password_reset', 'email_changed', 'deactivated', 'reuse_detected')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT auth_sessions_revoked_consistent_check CHECK ((revoked_at IS NULL) = (revoke_reason IS NULL))
);
-- Revoking every session of an account.
CREATE INDEX auth_sessions_user_id_idx ON auth_sessions (user_id);
-- The job that deletes expired sessions.
CREATE INDEX auth_sessions_expires_at_idx ON auth_sessions (expires_at);

-- +goose Down
DROP TABLE auth_sessions;
