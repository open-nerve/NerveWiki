-- name: CreateWorkspace :exec
-- The audit columns come from the use case's clock and caller.
INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(slug), sqlc.arg(name), sqlc.arg(by), sqlc.arg(by), sqlc.arg(now), sqlc.arg(now));

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

-- name: LockWorkspaceBySlug :one
-- The workspace not deleted with the slug, locked FOR NO KEY UPDATE until the transaction ends: every
-- change of a workspace and of its members takes this lock first, then decides (M2 design 8). A
-- deletion committed while it waited leaves no row.
SELECT id, slug, name, created_at, updated_at
FROM workspaces
WHERE slug = sqlc.arg(slug) AND deleted_at IS NULL
FOR NO KEY UPDATE;

-- name: LockWorkspaceByID :one
-- LockWorkspaceBySlug by the workspace's id: for the operations that name a member.
SELECT id, slug, name, created_at, updated_at
FROM workspaces
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
FOR NO KEY UPDATE;

-- name: LockWorkspacesOf :many
-- The workspaces not deleted of the account's active memberships, locked FOR NO KEY UPDATE in id order until
-- the transaction ends: a deactivation's (M2/P4 design 3.1). The memberships are those of the statement's
-- snapshot; the caller reads them again under the locks (ListStandings).
SELECT id, slug, name, created_at, updated_at
FROM workspaces
WHERE id IN (
    SELECT workspace_id FROM workspace_members
    WHERE user_id = sqlc.arg(user_id) AND ended_at IS NULL AND deleted_at IS NULL
) AND deleted_at IS NULL
ORDER BY id
FOR NO KEY UPDATE;

-- name: RenameWorkspace :exec
UPDATE workspaces
SET name = sqlc.arg(name), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: DeleteWorkspace :exec
-- The deleter is recorded as the last to update the row.
UPDATE workspaces
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: ShareWorkspaceBySlug :one
-- The workspace not deleted with the slug, locked FOR SHARE until the transaction ends: the invitations'
-- writes take it. They run beside each other, and wait for a change of the workspace or of its members
-- (FOR NO KEY UPDATE), which waits for them (M2/P3 design 3.3).
SELECT id, slug, name, created_at, updated_at
FROM workspaces
WHERE slug = sqlc.arg(slug) AND deleted_at IS NULL
FOR SHARE;

-- name: ShareWorkspaceByID :one
-- ShareWorkspaceBySlug by the workspace's id: for the operations that name an invitation, and for the notebook
-- module's writes (M3 design 4).
SELECT id, slug, name, created_at, updated_at
FROM workspaces
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
FOR SHARE;

-- name: FindWorkspaceByID :one
-- FindWorkspaceBySlug by the workspace's id.
SELECT id, slug, name, created_at, updated_at
FROM workspaces
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: WorkspaceSlugs :many
-- The slugs of the workspaces not deleted among the ids, unlocked: the notebook module's rule two names them
-- (M3 design 4).
SELECT id, slug
FROM workspaces
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;
