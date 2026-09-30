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

-- name: GetSessionForRefresh :one
-- What a refresh judges its token against, by primary key (M1/P2 design 3.5).
SELECT user_id, generation, token_hash, expires_at, revoked_at
FROM auth_sessions
WHERE id = sqlc.arg(id);

-- name: RotateSession :execrows
-- The conditional rotation of M1/P2 design 3.5: it hits only while the session is still at the
-- generation and hash the refresh judged, unrevoked and unexpired. A concurrent writer's commit
-- makes it re-evaluate the WHERE on the new row, and miss.
-- sqlc types a parameter by its first use: updated_at (NOT NULL) comes first in each SET below,
-- so that now is a time.Time, not a *time.Time.
UPDATE auth_sessions
SET updated_at = sqlc.arg(now), last_refreshed_at = sqlc.arg(now),
    generation = generation + 1, token_hash = sqlc.arg(new_token_hash)
WHERE id = sqlc.arg(id) AND generation = sqlc.arg(generation) AND token_hash = sqlc.arg(token_hash)
  AND revoked_at IS NULL AND expires_at > sqlc.arg(now);

-- name: RevokeSessionForReuse :exec
-- A revoked session keeps the reason it was revoked for.
UPDATE auth_sessions
SET updated_at = sqlc.arg(now), revoked_at = sqlc.arg(now), revoke_reason = 'reuse_detected'
WHERE id = sqlc.arg(id) AND revoked_at IS NULL;

-- name: EndSession :execrows
-- Logout: the same conditions as the rotation (M1/P2 design 3.6).
UPDATE auth_sessions
SET updated_at = sqlc.arg(now), revoked_at = sqlc.arg(now), revoke_reason = 'logout'
WHERE id = sqlc.arg(id) AND generation = sqlc.arg(generation) AND token_hash = sqlc.arg(token_hash)
  AND revoked_at IS NULL AND expires_at > sqlc.arg(now);

-- name: RevokeSessions :execrows
-- Every live session of the account but keep, the nil uuid to keep none, with reason (M1/P3 design 3.5). A session
-- revoked or expired already keeps what it has.
UPDATE auth_sessions
SET updated_at = sqlc.arg(now), revoked_at = sqlc.arg(now), revoke_reason = sqlc.arg(reason)::text
WHERE user_id = sqlc.arg(user_id) AND id <> sqlc.arg(keep)
  AND revoked_at IS NULL AND expires_at > sqlc.arg(now);

-- name: DeleteExpiredSessions :execrows
-- The periodic cleanup (M1/P4 design 3.5): up to batch sessions that expired before now. A row another
-- transaction holds (a refresh, a logout, a revocation) is skipped, not waited for: the next run deletes it, and
-- the cleanup never joins the lock order. The subquery needs its alias: without it sqlc finds expires_at
-- ambiguous.
DELETE FROM auth_sessions
WHERE id IN (
    SELECT s.id FROM auth_sessions s
    WHERE s.expires_at < sqlc.arg(now)
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
);
