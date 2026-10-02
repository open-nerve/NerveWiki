-- name: CreateSession :exec
-- An edit session opened, under its page's content row's lock.
INSERT INTO edit_sessions (id, node_id, notebook_id, user_id, client, created_at, expires_at)
VALUES (sqlc.arg(id), sqlc.arg(node_id), sqlc.arg(notebook_id), sqlc.arg(user_id), sqlc.arg(client), sqlc.arg(now),
    sqlc.arg(expires_at));

-- name: LockSession :one
-- The edit session a content write names, FOR UPDATE, after its page's content row (M4 design 4): the write
-- sets its changeset and revision.
SELECT id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at
FROM edit_sessions WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: SetSessionWrite :exec
-- The changeset an edit session's write went to and the revision it wrote, under LockSession's lock.
UPDATE edit_sessions SET changeset_id = sqlc.arg(changeset_id), revision = sqlc.arg(revision) WHERE id = sqlc.arg(id);

-- name: FindLiveSession :one
-- The caller's edit session alive at now, unlocked: a heartbeat reads its notebook to decide on.
SELECT id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at
FROM edit_sessions
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND expires_at > sqlc.arg(now);

-- name: HeartbeatSession :one
-- A heartbeat: one statement on the caller's own session, alive at now, which it keeps until until (M4 design
-- 4, "locks": no workspace or notebook row).
UPDATE edit_sessions SET expires_at = sqlc.arg(until)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND expires_at > sqlc.arg(now)
RETURNING id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at;

-- name: EndSession :one
-- The caller's end of their own session alive at now: its row goes. An expired one is left to the cleanup.
DELETE FROM edit_sessions
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND expires_at > sqlc.arg(now)
RETURNING id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at;

-- name: DeleteNodeSessions :many
-- The edit sessions of pages deleted, in their deletion's unit, after the nodes (nodes -> page_contents ->
-- edit_sessions).
DELETE FROM edit_sessions WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[])
RETURNING id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at;

-- name: DeleteNotebookSessions :many
-- The edit sessions of notebooks deleted, in their deletion's transaction.
DELETE FROM edit_sessions WHERE notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[])
RETURNING id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at;

-- name: DeleteExpiredSessions :execrows
-- The periodic cleanup (M4/P4 design 3.5): up to batch sessions expired at now. A row another transaction holds
-- (a content write with it, a heartbeat) is skipped, not waited for: the next run deletes it, and the cleanup
-- never joins the lock order. The subquery needs its alias: without it sqlc finds expires_at ambiguous.
DELETE FROM edit_sessions
WHERE id IN (
    SELECT s.id FROM edit_sessions s
    WHERE s.expires_at <= sqlc.arg(now)
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
);
