-- name: CreateUser :exec
-- is_active and onboarding_steps take their defaults; the audit columns come from the use case's clock.
INSERT INTO users (id, email, password, display_name, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(email), sqlc.arg(password), sqlc.arg(display_name), sqlc.arg(now), sqlc.arg(now));

-- name: GetUser :one
SELECT id, email, display_name, onboarding_steps
FROM users
WHERE id = sqlc.arg(id);

-- name: FindLoginAccount :one
-- What login reads before its transaction; the hash is its snapshot (M1/P2 design 3.4).
SELECT id, password
FROM users
WHERE email = sqlc.arg(email);

-- name: LockUserForCredentials :one
-- The account row lock (M1 design 4, M1/P2 design 3.4). FOR NO KEY UPDATE conflicts with itself
-- and with FOR UPDATE, so the credential transactions of one account run one after another; it
-- does not conflict with the FOR KEY SHARE that foreign-key checks take, so inserting rows that
-- reference the account does not wait. Lock order: users, then auth_sessions.
SELECT password, is_active
FROM users
WHERE id = sqlc.arg(id)
FOR NO KEY UPDATE;

-- name: UpdatePasswordHash :exec
UPDATE users
SET password = sqlc.arg(password), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);
