-- name: CreateUser :exec
-- is_active and onboarding_steps take their defaults; the audit columns come from the use case's clock.
INSERT INTO users (id, email, password, display_name, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(email), sqlc.arg(password), sqlc.arg(display_name), sqlc.arg(now), sqlc.arg(now));

-- name: GetUser :one
SELECT id, email, display_name, onboarding_steps
FROM users
WHERE id = sqlc.arg(id);
