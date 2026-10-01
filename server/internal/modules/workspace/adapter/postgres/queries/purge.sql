-- name: PurgeInvitations :execrows
-- The purge (M2/P4 design 3.4): up to batch invitations deleted before the time. A row another transaction
-- holds is skipped, not waited for: the next run takes it, and the purge never joins the lock order.
DELETE FROM workspace_invitations
WHERE id IN (
    SELECT i.id FROM workspace_invitations i
    WHERE i.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
);

-- name: PurgeMembers :execrows
-- PurgeInvitations for the members: those of the workspaces deleted before the time.
DELETE FROM workspace_members
WHERE id IN (
    SELECT m.id FROM workspace_members m
    WHERE m.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
);

-- name: PurgeWorkspaces :execrows
-- PurgeInvitations for the workspaces, which the purge takes after their invitations and members: the
-- foreign keys' ON DELETE CASCADE finds nothing left to delete.
DELETE FROM workspaces
WHERE id IN (
    SELECT w.id FROM workspaces w
    WHERE w.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
);
