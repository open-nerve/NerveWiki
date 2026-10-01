-- name: DeleteAuditEventsOf :exec
-- A workspace's audit events not deleted, with its deletion's time: no other writer locks them, and the
-- writers that add one hold the workspace's row, which the deletion holds FOR NO KEY UPDATE.
UPDATE notebook_audit_events
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE workspace_id = sqlc.arg(workspace_id) AND deleted_at IS NULL;
