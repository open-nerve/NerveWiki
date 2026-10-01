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
