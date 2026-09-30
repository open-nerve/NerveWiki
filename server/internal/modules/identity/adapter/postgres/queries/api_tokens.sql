-- name: GetAPITokenByHash :one
-- What authentication checks of a personal access token (M1/P3 design 3.2); the use case judges it against its
-- clock.
SELECT t.id, t.user_id, t.expires_at, t.last_used_at, t.revoked_at, u.is_active AS user_active
FROM api_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.token_hash = sqlc.arg(token_hash);

-- name: GetAPITokenByID :one
-- The same, of the token a request authenticated with: the credential lock checks it again (M1/P3 design 3.3).
SELECT t.id, t.user_id, t.expires_at, t.last_used_at, t.revoked_at, u.is_active AS user_active
FROM api_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.id = sqlc.arg(id);

-- name: TouchAPIToken :exec
-- last_used_at, written at most once a minute: only when it is older than stale_before. Using a token changes
-- nothing of it, so updated_at stays.
UPDATE api_tokens
SET last_used_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND (last_used_at IS NULL OR last_used_at < sqlc.arg(stale_before)::timestamptz);

-- name: CreateAPIToken :exec
INSERT INTO api_tokens (id, user_id, token_hash, name, expires_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(token_hash), sqlc.arg(name), sqlc.arg(expires_at), sqlc.arg(now),
        sqlc.arg(now));
