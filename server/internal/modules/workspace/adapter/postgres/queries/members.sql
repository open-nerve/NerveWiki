-- name: AddMember :exec
-- The audit columns come from the member's creation time and the caller.
INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(user_id), sqlc.arg(role), sqlc.arg(by), sqlc.arg(by),
    sqlc.arg(created_at), sqlc.arg(created_at));

-- name: RoleOf :one
-- The access module's fact of the workspace level: the account's active membership of the workspace,
-- read in the caller's transaction.
SELECT role
FROM workspace_members
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
    AND ended_at IS NULL AND deleted_at IS NULL;

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

-- name: FindMembership :one
-- The account's membership of the workspace, ended or not, deleted excepted: at most one
-- (workspace_members_workspace_id_user_id_key).
SELECT id, workspace_id, user_id, role, created_at, ended_at
FROM workspace_members
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id) AND deleted_at IS NULL;

-- name: RestoreMember :exec
-- An ended membership active again, with the role: the same row, which keeps when the account first
-- joined, created_at (M2/P3 design 3.5).
UPDATE workspace_members
SET ended_at = NULL, role = sqlc.arg(role), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: ListStandings :many
-- Rule two's view (M2/P4 design 3.1) of the account's active memberships of workspaces the caller has
-- locked: its role, and how many active admins and members each workspace has, the account included.
SELECT m.workspace_id, w.slug, m.role,
    count(*) FILTER (WHERE o.role = 'admin') AS admins,
    count(*) AS members
FROM workspace_members m
JOIN workspaces w ON w.id = m.workspace_id
JOIN workspace_members o ON o.workspace_id = m.workspace_id AND o.ended_at IS NULL AND o.deleted_at IS NULL
WHERE m.user_id = sqlc.arg(user_id) AND m.workspace_id = ANY(sqlc.arg(workspace_ids)::uuid[])
    AND m.ended_at IS NULL AND m.deleted_at IS NULL
GROUP BY m.workspace_id, w.slug, m.role
ORDER BY m.workspace_id;
