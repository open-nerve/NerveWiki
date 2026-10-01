-- name: CreateWorkspace :exec
-- The audit columns come from the use case's clock and caller.
INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(slug), sqlc.arg(name), sqlc.arg(by), sqlc.arg(by), sqlc.arg(now), sqlc.arg(now));

-- name: AddMember :exec
-- The audit columns come from the member's creation time and the caller.
INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(user_id), sqlc.arg(role), sqlc.arg(by), sqlc.arg(by),
    sqlc.arg(created_at), sqlc.arg(created_at));

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

-- name: RenameWorkspace :exec
UPDATE workspaces
SET name = sqlc.arg(name), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: DeleteWorkspace :exec
-- The deleter is recorded as the last to update the row.
UPDATE workspaces
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: DeleteMembersOf :exec
-- Every row of the workspace not deleted, ended ones too, at the workspace's deletion time.
UPDATE workspace_members
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE workspace_id = sqlc.arg(workspace_id) AND deleted_at IS NULL;

-- name: FindActiveMember :one
SELECT id, workspace_id, user_id, role, created_at
FROM workspace_members
WHERE id = sqlc.arg(id) AND ended_at IS NULL AND deleted_at IS NULL;

-- name: ListActiveMembers :many
-- By when they joined.
SELECT id, workspace_id, user_id, role, created_at
FROM workspace_members
WHERE workspace_id = sqlc.arg(workspace_id) AND ended_at IS NULL AND deleted_at IS NULL
ORDER BY created_at, id;

-- name: CountActiveAdmins :one
SELECT count(*)
FROM workspace_members
WHERE workspace_id = sqlc.arg(workspace_id) AND role = 'admin' AND ended_at IS NULL AND deleted_at IS NULL;

-- name: UpdateMemberRole :exec
UPDATE workspace_members
SET role = sqlc.arg(role), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: EndMemberships :exec
-- The account's active memberships of the workspaces.
UPDATE workspace_members
SET ended_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE workspace_id = ANY(sqlc.arg(workspace_ids)::uuid[]) AND user_id = sqlc.arg(user_id)
    AND ended_at IS NULL AND deleted_at IS NULL;

-- name: ShareWorkspaceBySlug :one
-- The workspace not deleted with the slug, locked FOR SHARE until the transaction ends: the invitations'
-- writes take it. They run beside each other, and wait for a change of the workspace or of its members
-- (FOR NO KEY UPDATE), which waits for them (M2/P3 design 3.3).
SELECT id, slug, name, created_at, updated_at
FROM workspaces
WHERE slug = sqlc.arg(slug) AND deleted_at IS NULL
FOR SHARE;

-- name: ShareWorkspaceByID :one
-- ShareWorkspaceBySlug by the workspace's id: for the operations that name an invitation.
SELECT id, slug, name, created_at, updated_at
FROM workspaces
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
FOR SHARE;

-- name: FindWorkspaceByID :one
-- FindWorkspaceBySlug by the workspace's id.
SELECT id, slug, name, created_at, updated_at
FROM workspaces
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: FindMembership :one
-- The account's membership of the workspace, ended or not, deleted excepted: at most one
-- (workspace_members_workspace_id_user_id_key).
SELECT id, workspace_id, user_id, role, created_at, (ended_at IS NULL)::boolean AS active
FROM workspace_members
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id) AND deleted_at IS NULL;

-- name: RestoreMember :exec
-- An ended membership active again, with the role: the same row, which keeps when the account first
-- joined, created_at (M2/P3 design 3.5).
UPDATE workspace_members
SET ended_at = NULL, role = sqlc.arg(role), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);
