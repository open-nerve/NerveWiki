-- name: CreateInvitation :exec
-- The audit columns come from the invitation's creation time and the caller.
INSERT INTO workspace_invitations (id, workspace_id, email, role, created_by_id, updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(email), sqlc.arg(role), sqlc.arg(by), sqlc.arg(by),
    sqlc.arg(created_at), sqlc.arg(created_at));

-- name: ListPendingInvitations :many
-- Newest first.
SELECT id, workspace_id, email, role, created_at
FROM workspace_invitations
WHERE workspace_id = sqlc.arg(workspace_id) AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: FindPendingInvitation :one
-- A pending invitation of a workspace not deleted, unlocked. A deletion of the workspace deletes its
-- invitations in the same transaction (M2/P3); the workspace's own deleted_at is asked too, so the
-- answer does not rest on that alone.
SELECT i.id, i.workspace_id, i.email, i.role, i.created_at
FROM workspace_invitations i
JOIN workspaces w ON w.id = i.workspace_id
WHERE i.id = sqlc.arg(id) AND i.deleted_at IS NULL AND w.deleted_at IS NULL;

-- name: LockPendingInvitation :one
-- The pending invitation, locked FOR UPDATE until the transaction ends, by a transaction that holds its
-- workspace's lock: the invitation's row alone, no join, so as not to lock the workspace's row again.
SELECT id, workspace_id, email, role, created_at
FROM workspace_invitations
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
FOR UPDATE;

-- name: DeleteInvitation :exec
-- The deleter is recorded as the last to update the row.
UPDATE workspace_invitations
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: AcceptInvitation :exec
-- Accepted and deleted at the same moment (workspace_invitations_accepted_check).
UPDATE workspace_invitations
SET accepted_at = sqlc.arg(now)::timestamptz, deleted_at = sqlc.arg(now)::timestamptz,
    updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: DeleteInvitationsTo :exec
-- The pending invitations of the workspaces to the address: a membership's end.
UPDATE workspace_invitations
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE workspace_id = ANY(sqlc.arg(workspace_ids)::uuid[]) AND email = sqlc.arg(email) AND deleted_at IS NULL;

-- name: DeleteInvitationsOf :exec
-- Every pending invitation of the workspace, at the workspace's deletion time.
UPDATE workspace_invitations
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE workspace_id = sqlc.arg(workspace_id) AND deleted_at IS NULL;
