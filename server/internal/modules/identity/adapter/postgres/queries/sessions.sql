-- name: CreateSession :exec
-- A new sign-in: generation 0.
INSERT INTO auth_sessions (id, user_id, token_hash, generation, user_agent, ip, expires_at, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(token_hash), 0, sqlc.arg(user_agent), sqlc.arg(ip),
        sqlc.arg(expires_at), sqlc.arg(now), sqlc.arg(now));

-- name: GetSessionCredential :one
-- What authentication checks on every request, by primary key; the use case judges it against its clock.
SELECT s.user_id, s.expires_at, s.revoked_at, u.is_active AS user_active
FROM auth_sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id = sqlc.arg(id);
