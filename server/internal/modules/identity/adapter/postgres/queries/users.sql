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
-- reference the account does not wait. Lock order: users, then auth_sessions, then api_tokens.
SELECT email, password, is_active
FROM users
WHERE id = sqlc.arg(id)
FOR NO KEY UPDATE;

-- name: UpdatePasswordHash :exec
UPDATE users
SET password = sqlc.arg(password), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: GetPasswordAccount :one
-- What an operation that asks for the current password reads before its transaction: the address for the password
-- rules, the hash as the snapshot (M1/P3 design 3.4).
SELECT email, password
FROM users
WHERE id = sqlc.arg(id);

-- name: UpdateDisplayName :one
-- PATCH /me (M1/P3 design 3.5): one statement.
UPDATE users
SET display_name = sqlc.arg(display_name), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING id, email, display_name, onboarding_steps;

-- name: RecordOnboardingStep :one
-- A completed step is appended once (M1/P3 design 3.5): a step recorded already changes nothing, updated_at
-- included. A concurrent record waits for the row and appends to what it left. users_onboarding_steps_check bounds
-- the count.
UPDATE users
SET onboarding_steps = CASE WHEN sqlc.arg(step)::text = ANY (onboarding_steps) THEN onboarding_steps
                            ELSE array_append(onboarding_steps, sqlc.arg(step)::text) END,
    updated_at       = CASE WHEN sqlc.arg(step)::text = ANY (onboarding_steps) THEN updated_at
                            ELSE sqlc.arg(now)::timestamptz END
WHERE id = sqlc.arg(id)
RETURNING id, email, display_name, onboarding_steps;
