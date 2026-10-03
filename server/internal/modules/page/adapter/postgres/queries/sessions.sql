-- An edit session is alive while it has no end reason and its lease runs past now (M5 design 4.1): a tombstone,
-- a session taken over or unlocked, is not, whatever its expires_at.

-- name: CreateSession :exec
-- An edit session opened, under its page's content row's lock.
INSERT INTO edit_sessions (id, node_id, notebook_id, user_id, client, created_at, expires_at)
VALUES (sqlc.arg(id), sqlc.arg(node_id), sqlc.arg(notebook_id), sqlc.arg(user_id), sqlc.arg(client), sqlc.arg(now),
    sqlc.arg(expires_at));

-- name: LockSession :one
-- The edit session a content write names, FOR UPDATE, after its page's content row (M4 design 4): the write
-- sets its changeset and revision.
SELECT id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at, ended_reason,
    ended_by_id, ended_at
FROM edit_sessions WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: SetSessionWrite :execrows
-- The changeset an edit session's write went to and the revision it wrote, under LockSession's lock: the row is
-- there, and a row not updated is a fault.
UPDATE edit_sessions SET changeset_id = sqlc.arg(changeset_id), revision = sqlc.arg(revision) WHERE id = sqlc.arg(id);

-- name: AliveSessionsOf :many
-- The sessions of the pages node_ids alive at now, unlocked (M5/P1 design 3.3): the lock's vetoer and guard and
-- the lock's read. The units that call it hold what serializes them with an opening (M5 design 4.4).
SELECT id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at, ended_reason,
    ended_by_id, ended_at
FROM edit_sessions
WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]) AND ended_reason IS NULL AND expires_at > sqlc.arg(now)
ORDER BY created_at, id;

-- name: DeleteExpiredSessionsOf :execrows
-- An opening's first step, under its page's content row's lock: the page's rows expired at now go, tombstones
-- among them, so that a heartbeat that read an earlier time finds no row to keep alive (M5 design 4.1).
DELETE FROM edit_sessions WHERE node_id = sqlc.arg(node_id) AND expires_at <= sqlc.arg(now);

-- name: EndAliveSessions :many
-- The page's sessions alive at at, of user_id alone when it is given, become tombstones of reason by by: a
-- take-over ends its owner's, an unlock anyone's (M5 design 4.2, 4.3). Their lease is kept, and lengthened to
-- until, a lease after the end, so that the tab learns why; the check expires_at > created_at holds.
UPDATE edit_sessions
SET ended_reason = sqlc.arg(reason)::text, ended_by_id = sqlc.arg(by_id)::uuid, ended_at = sqlc.arg(at)::timestamptz,
    expires_at = greatest(expires_at, sqlc.arg(until))
WHERE node_id = sqlc.arg(node_id) AND ended_reason IS NULL AND expires_at > sqlc.arg(at)
    AND (sqlc.narg(user_id)::uuid IS NULL OR user_id = sqlc.narg(user_id))
RETURNING id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at, ended_reason,
    ended_by_id, ended_at;

-- name: FindEndedSession :one
-- The caller's tombstone id: a heartbeat that finds no alive session asks why.
SELECT id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at, ended_reason,
    ended_by_id, ended_at
FROM edit_sessions
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND ended_reason IS NOT NULL;

-- name: FindLiveSession :one
-- The caller's edit session alive at now, unlocked: a heartbeat reads its notebook to decide on.
SELECT id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at, ended_reason,
    ended_by_id, ended_at
FROM edit_sessions
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND ended_reason IS NULL AND expires_at > sqlc.arg(now);

-- name: HeartbeatSession :one
-- A heartbeat: one statement on the caller's own session, alive at now, which it keeps until until (M4 design
-- 4, "locks": no workspace or notebook row). A take-over or unlock it waits behind is seen when it resumes: the
-- row it then finds has an end reason, and it updates nothing.
UPDATE edit_sessions SET expires_at = sqlc.arg(until)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND ended_reason IS NULL AND expires_at > sqlc.arg(now)
RETURNING id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at, ended_reason,
    ended_by_id, ended_at;

-- name: EndSession :one
-- The caller's end of their own session alive at now, or of their tombstone: its row goes. An expired one is
-- left to the cleanup.
DELETE FROM edit_sessions
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)
    AND (ended_reason IS NOT NULL OR expires_at > sqlc.arg(now))
RETURNING id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at, ended_reason,
    ended_by_id, ended_at;

-- name: DeleteNodeSessions :many
-- The edit sessions of pages deleted, in their deletion's unit, after the nodes (nodes -> page_contents ->
-- edit_sessions).
DELETE FROM edit_sessions WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[])
RETURNING id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at, ended_reason,
    ended_by_id, ended_at;

-- name: DeleteNotebookSessions :many
-- The edit sessions of notebooks deleted, in their deletion's transaction.
DELETE FROM edit_sessions WHERE notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[])
RETURNING id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at, ended_reason,
    ended_by_id, ended_at;

-- name: DeleteExpiredSessions :execrows
-- The periodic cleanup (M4/P4 design 3.5): up to batch sessions expired at now, tombstones among them. A row
-- another transaction holds (a content write with it, a heartbeat) is skipped, not waited for: the next run
-- deletes it, and the cleanup never joins the lock order. The subquery needs its alias: without it sqlc finds
-- expires_at ambiguous.
DELETE FROM edit_sessions
WHERE id IN (
    SELECT s.id FROM edit_sessions s
    WHERE s.expires_at <= sqlc.arg(now)
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
);
