-- name: ListVersions :many
-- A page's versions newer-first below the revision below (0: from the newest), each with its changeset (M8/P1 design
-- 3.4): on the unique index (node_id, revision) backward. The content is not read; deleted rows are listed, as a
-- page in the trash has its history.
SELECT r.id, r.base_revision, r.revision, r.byte_size, r.created_at, r.updated_at, c.id AS changeset_id, c.kind,
    c.client, c.created_by_id, c.created_at AS changeset_created_at
FROM page_revisions r
JOIN changesets c ON c.id = r.changeset_id
WHERE r.node_id = sqlc.arg(node_id) AND (sqlc.arg(below)::integer = 0 OR r.revision < sqlc.arg(below)::integer)
ORDER BY r.revision DESC
LIMIT sqlc.arg(max);

-- name: FindVersion :one
-- The first version of a page at or above the revision: the only one that may hold it, as each version's base is the
-- one before's revision. The caller checks its base.
SELECT r.id, r.base_revision, r.revision, r.byte_size, r.created_at, r.updated_at, c.id AS changeset_id, c.kind,
    c.client, c.created_by_id, c.created_at AS changeset_created_at
FROM page_revisions r
JOIN changesets c ON c.id = r.changeset_id
WHERE r.node_id = sqlc.arg(node_id) AND r.revision >= sqlc.arg(revision)
ORDER BY r.revision
LIMIT 1;

-- name: VersionContent :one
-- A version's content, as written.
SELECT content FROM page_revisions WHERE id = sqlc.arg(id);

-- name: AliveChangesetOf :many
-- The changeset of the page's edit session alive at now (AliveSessionsOf's condition), if it wrote; a page has at
-- most one alive session, its lock.
SELECT changeset_id
FROM edit_sessions
WHERE node_id = sqlc.arg(node_id) AND ended_reason IS NULL AND expires_at > sqlc.arg(now) AND changeset_id IS NOT NULL;
