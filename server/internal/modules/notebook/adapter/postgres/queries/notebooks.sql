-- name: CreateNotebook :exec
-- The audit columns come from the use case's clock and caller.
INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(name), sqlc.arg(workspace_access), sqlc.arg(by), sqlc.arg(by),
    sqlc.arg(now), sqlc.arg(now));

-- name: FindNotebook :one
-- A notebook not deleted, unlocked: what a read authorizes against.
SELECT id, workspace_id, name, workspace_access, created_at, updated_at
FROM notebooks
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: LockNotebook :one
-- FindNotebook locked FOR NO KEY UPDATE until the transaction ends: a notebook's management writes take it
-- after the workspace row's FOR SHARE, then decide (M3/P1 design 3.10). A deletion committed while it waited
-- leaves no row.
SELECT id, workspace_id, name, workspace_access, created_at, updated_at
FROM notebooks
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
FOR NO KEY UPDATE;

-- name: UpdateNotebook :exec
UPDATE notebooks
SET name = sqlc.arg(name), workspace_access = sqlc.arg(workspace_access), updated_by_id = sqlc.arg(by),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: DeleteNotebook :exec
-- The deleter is recorded as the last to update the row.
UPDATE notebooks
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: ListNotebooks :many
-- The notebooks not deleted of the workspace that the account is an active member of, or, when the workspace
-- access reaches it (an admin's or a member's), that are open to the workspace; with its explicit role and the
-- count of active members. By name, case-insensitively, then by name and id. The same rule as the access
-- module's decision, shared.EffectiveNotebookRole: bootstrap's visibility test holds the two together.
SELECT n.id, n.workspace_id, n.name, n.workspace_access, n.created_at, n.updated_at, m.role,
    (SELECT count(*) FROM notebook_members c
     WHERE c.notebook_id = n.id AND c.ended_at IS NULL AND c.deleted_at IS NULL) AS member_count
FROM notebooks n
LEFT JOIN notebook_members m
    ON m.notebook_id = n.id AND m.user_id = sqlc.arg(user_id) AND m.ended_at IS NULL AND m.deleted_at IS NULL
WHERE n.workspace_id = sqlc.arg(workspace_id) AND n.deleted_at IS NULL
    AND (m.id IS NOT NULL OR (sqlc.arg(reached)::boolean AND n.workspace_access <> 'none'))
ORDER BY lower(n.name), n.name, n.id;

-- name: NotebookFacts :one
-- The access module's facts of the notebook level: the notebook not deleted, its workspace and access, and
-- the account's active membership's role, NULL for none.
SELECT n.workspace_id, n.workspace_access, m.role
FROM notebooks n
LEFT JOIN notebook_members m
    ON m.notebook_id = n.id AND m.user_id = sqlc.arg(user_id) AND m.ended_at IS NULL AND m.deleted_at IS NULL
WHERE n.id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL;
