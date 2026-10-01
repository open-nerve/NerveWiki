-- name: CreateWorkspace :exec
-- The audit columns come from the use case's clock and caller.
INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(slug), sqlc.arg(name), sqlc.arg(by), sqlc.arg(by), sqlc.arg(now), sqlc.arg(now));

-- name: AddMember :exec
INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(user_id), sqlc.arg(role), sqlc.arg(by), sqlc.arg(by),
    sqlc.arg(now), sqlc.arg(now));

-- name: FindWorkspaceBySlug :one
-- A workspace not deleted, unlocked: what a read authorizes against.
SELECT id, slug, name, created_at, updated_at
FROM workspaces
WHERE slug = sqlc.arg(slug) AND deleted_at IS NULL;

-- name: ListWorkspacesOf :many
-- The workspaces not deleted of the account's active memberships, by name, case-insensitively, then
-- by id. The members of a deleted workspace are deleted with it, in the same transaction (M2/P2);
-- the workspace's own deleted_at is asked too, so the list does not rest on that alone.
SELECT w.id, w.slug, w.name, w.created_at, w.updated_at, m.role
FROM workspace_members m
JOIN workspaces w ON w.id = m.workspace_id
WHERE m.user_id = sqlc.arg(user_id) AND m.ended_at IS NULL AND m.deleted_at IS NULL AND w.deleted_at IS NULL
ORDER BY lower(w.name), w.name, w.id;

-- name: SlugTaken :one
SELECT EXISTS (SELECT 1 FROM workspaces WHERE slug = sqlc.arg(slug) AND deleted_at IS NULL);

-- name: RoleOf :one
-- The access module's fact of the workspace level: the account's active membership of the workspace,
-- read in the caller's transaction.
SELECT role
FROM workspace_members
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
    AND ended_at IS NULL AND deleted_at IS NULL;
