-- name: ListVersions :many
-- A page's versions newer-first below the revision below, each with its changeset (M8/P1 design 3.4): a range of the
-- unique index (node_id, revision) backward, whatever plan is kept for the statement. The content is not read; deleted
-- rows are listed, as a page in the trash has its history.
SELECT r.id, r.base_revision, r.revision, r.byte_size, r.created_at, r.updated_at, c.id AS changeset_id, c.kind,
    c.client, c.created_by_id, c.created_at AS changeset_created_at
FROM page_revisions r
JOIN changesets c ON c.id = r.changeset_id
WHERE r.node_id = sqlc.arg(node_id) AND r.revision < sqlc.arg(below)::bigint
ORDER BY r.revision DESC
LIMIT sqlc.arg(max);

-- name: FindVersion :one
-- The first version of a page at or above the revision, with its content, in one statement: the only one that may
-- hold it, as each version's base is the one before's revision; an edit session's save, which moves its version's
-- row on, comes before or after the whole read. The caller checks its base.
SELECT r.id, r.base_revision, r.revision, r.byte_size, r.created_at, r.updated_at, c.id AS changeset_id, c.kind,
    c.client, c.created_by_id, c.created_at AS changeset_created_at, r.content
FROM page_revisions r
JOIN changesets c ON c.id = r.changeset_id
WHERE r.node_id = sqlc.arg(node_id) AND r.revision >= sqlc.arg(revision)
ORDER BY r.revision
LIMIT 1;

-- name: AliveChangesetOf :many
-- The changeset of the page's edit session alive at now (AliveSessionsOf's condition), if it wrote; a page has at
-- most one alive session, its lock.
SELECT changeset_id
FROM edit_sessions
WHERE node_id = sqlc.arg(node_id) AND ended_reason IS NULL AND expires_at > sqlc.arg(now) AND changeset_id IS NOT NULL;
