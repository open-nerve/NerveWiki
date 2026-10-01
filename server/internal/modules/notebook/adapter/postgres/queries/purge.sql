-- name: PurgeMembers :execrows
-- The purge (v0.1 design 13.1, item 6): up to batch member rows deleted before the time, those of the deleted
-- notebooks. A row another transaction holds is skipped, not waited for: the next run takes it, and the purge
-- never joins the lock order.
DELETE FROM notebook_members
WHERE id IN (
    SELECT m.id FROM notebook_members m
    WHERE m.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
);

-- name: PurgeNotebooks :execrows
-- PurgeMembers for the notebooks with no member row left: the purge takes those first. One whose members a
-- purger skipped waits for a later run with them, so that the foreign key's ON DELETE CASCADE never deletes,
-- nor waits for, a row another transaction holds.
DELETE FROM notebooks
WHERE id IN (
    SELECT n.id FROM notebooks n
    WHERE n.deleted_at < sqlc.arg(before)::timestamptz
        AND NOT EXISTS (SELECT 1 FROM notebook_members m WHERE m.notebook_id = n.id)
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
);

-- name: PurgeAuditEvents :execrows
-- The audit events deleted with their workspace: they reference no notebook, so no other purge waits for them.
DELETE FROM notebook_audit_events
WHERE id IN (
    SELECT e.id FROM notebook_audit_events e
    WHERE e.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
);
