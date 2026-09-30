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
-- nothing of it, so updated_at stays. Best effort: a row another transaction holds (a revocation, another
-- request's touch) is skipped rather than waited for, so authentication never waits on it; a later use writes it.
UPDATE api_tokens
SET last_used_at = sqlc.arg(now)::timestamptz
WHERE id = (
    SELECT t.id FROM api_tokens t
    WHERE t.id = sqlc.arg(id) AND (t.last_used_at IS NULL OR t.last_used_at < sqlc.arg(stale_before)::timestamptz)
    FOR NO KEY UPDATE SKIP LOCKED
);

-- name: CreateAPIToken :exec
INSERT INTO api_tokens (id, user_id, token_hash, name, expires_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(token_hash), sqlc.arg(name), sqlc.arg(expires_at), sqlc.arg(now),
        sqlc.arg(now));

-- name: ListAPITokens :many
-- The account's unrevoked tokens, expired ones too, newest first and then by id (M1/P3 design 3.2).
SELECT id, name, expires_at, last_used_at, created_at
FROM api_tokens
WHERE user_id = sqlc.arg(user_id) AND revoked_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: RevokeAPIToken :execrows
-- Revoking is a soft delete. Another account's token, or one revoked already, is not hit.
UPDATE api_tokens
SET updated_at = sqlc.arg(now), revoked_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND revoked_at IS NULL;
