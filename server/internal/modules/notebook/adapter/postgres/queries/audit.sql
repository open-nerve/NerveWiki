-- name: DeleteAuditEventsOf :exec
-- A workspace's audit events not deleted, with its deletion's time: no other writer locks them, and the
-- writers that add one hold the workspace's row, which the deletion holds FOR NO KEY UPDATE.
UPDATE notebook_audit_events
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE workspace_id = sqlc.arg(workspace_id) AND deleted_at IS NULL;

-- name: AddAuditEvent :exec
-- The actor is the event's creator and last updater: an event changes only with its workspace's deletion.
INSERT INTO notebook_audit_events (id, workspace_id, notebook_id, notebook_name, action, former_owner_id, created_by_id,
    updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(notebook_id), sqlc.arg(notebook_name), sqlc.arg(action),
    sqlc.arg(former_owner_id), sqlc.arg(actor_id), sqlc.arg(actor_id), sqlc.arg(at), sqlc.arg(at));

-- name: ListAuditEvents :many
-- A page of the workspace's audit events not deleted, newest first, then by id; when after_at is set, those after
-- the position (after_at, after_id) in that order. size is one more than the page, to tell whether another follows.
SELECT id, workspace_id, notebook_id, notebook_name, action, former_owner_id, created_by_id, created_at
FROM notebook_audit_events
WHERE workspace_id = sqlc.arg(workspace_id) AND deleted_at IS NULL
    AND (sqlc.narg(after_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(size);
