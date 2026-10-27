-- A notebook's log and what a revert reads of a changeset (M8 design 4.7, 4.8; M8/P3 design 3.4).

-- name: FindChangeset :one
-- A changeset of a live notebook (its deleted_at is its notebook's), with the brief of the one it reverts while that one
-- is there.
SELECT c.id, c.notebook_id, c.kind, c.reverts_id, c.client, c.message, c.created_by_id, c.created_at, c.updated_at,
    c.trimmed, v.kind AS reverts_kind, v.created_by_id AS reverts_by, v.created_at AS reverts_at
FROM changesets c LEFT JOIN changesets v ON v.id = c.reverts_id
WHERE c.id = sqlc.arg(id) AND c.deleted_at IS NULL;

-- name: ListChangesets :many
-- A notebook's log newer first below the cursor (M8 design 4.7): a range of its index backward, whatever plan is kept;
-- the client a filter on it, its values few. The brief of the changeset a revert reverts by its key.
SELECT c.id, c.notebook_id, c.kind, c.reverts_id, c.client, c.message, c.created_by_id, c.created_at, c.updated_at,
    c.trimmed, v.kind AS reverts_kind, v.created_by_id AS reverts_by, v.created_at AS reverts_at
FROM changesets c LEFT JOIN changesets v ON v.id = c.reverts_id
WHERE c.notebook_id = sqlc.arg(notebook_id) AND c.deleted_at IS NULL
    AND (c.created_at, c.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
    AND (sqlc.narg(client)::text IS NULL OR c.client = sqlc.narg(client)::text)
    AND (NOT sqlc.arg(all_mcp)::boolean OR c.client LIKE 'mcp:%')
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListChangesetsBy :many
-- ListChangesets of one actor: a range of the actor's index backward. A statement of its own: a cached statement's
-- plan for any arguments serves one of the filters' paths.
SELECT c.id, c.notebook_id, c.kind, c.reverts_id, c.client, c.message, c.created_by_id, c.created_at, c.updated_at,
    c.trimmed, v.kind AS reverts_kind, v.created_by_id AS reverts_by, v.created_at AS reverts_at
FROM changesets c LEFT JOIN changesets v ON v.id = c.reverts_id
WHERE c.notebook_id = sqlc.arg(notebook_id) AND c.created_by_id = sqlc.arg(actor) AND c.deleted_at IS NULL
    AND (c.created_at, c.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
    AND (sqlc.narg(client)::text IS NULL OR c.client = sqlc.narg(client)::text)
    AND (NOT sqlc.arg(all_mcp)::boolean OR c.client LIKE 'mcp:%')
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListChangesetsOfPage :many
-- ListChangesets of the changesets that changed a node, of one actor when it is set: the node's items' and versions'
-- changesets through their indexes on node_id, deleted or not (a node in the trash has its history), then each by its
-- key alone, as TrashLines reads a parent, sorted. Its cost is the node's history's: joined as the planner pleased,
-- without statistics a page of a 40,000 changesets' history walked the log's index and compared each with them all,
-- 7 s a page.
WITH touched AS (
    SELECT i.changeset_id FROM changeset_items i WHERE i.node_id = sqlc.arg(page_id)
    UNION
    SELECT r.changeset_id FROM page_revisions r WHERE r.node_id = sqlc.arg(page_id)
), changed AS (
    SELECT x.id, x.notebook_id, x.kind, x.reverts_id, x.client, x.message, x.created_by_id, x.created_at, x.updated_at,
        x.trimmed, x.deleted_at
    FROM touched t CROSS JOIN LATERAL (SELECT * FROM changesets WHERE id = t.changeset_id LIMIT 1) x
)
SELECT c.id, c.notebook_id, c.kind, c.reverts_id, c.client, c.message, c.created_by_id, c.created_at, c.updated_at,
    c.trimmed, v.kind AS reverts_kind, v.created_by_id AS reverts_by, v.created_at AS reverts_at
FROM changed c LEFT JOIN changesets v ON v.id = c.reverts_id
WHERE c.notebook_id = sqlc.arg(notebook_id)::uuid AND c.deleted_at IS NULL
    AND (c.created_at, c.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
    AND (sqlc.narg(actor)::uuid IS NULL OR c.created_by_id = sqlc.narg(actor)::uuid)
    AND (sqlc.narg(client)::text IS NULL OR c.client = sqlc.narg(client)::text)
    AND (NOT sqlc.arg(all_mcp)::boolean OR c.client LIKE 'mcp:%')
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);

-- name: ChangesetFacts :many
-- Each changeset's items and versions counted to 1,000 (M8 design 4.14), through its key on (changeset_id, node_id),
-- and the reverts of it, newest first, 100 at most, through the partial index on reverts_id.
SELECT u.id::uuid AS id,
    (SELECT count(*) FROM (SELECT 1 FROM changeset_items i WHERE i.changeset_id = u.id LIMIT 1000) x)::integer AS node_count,
    (SELECT count(*) FROM (SELECT 1 FROM page_revisions r WHERE r.changeset_id = u.id LIMIT 1000) x)::integer AS content_count,
    ARRAY(SELECT r.id FROM changesets r WHERE r.reverts_id = u.id AND r.deleted_at IS NULL
        ORDER BY r.created_at DESC, r.id DESC LIMIT 100)::uuid[] AS reverted_by
FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id) u;

-- name: ChangesetMoves :one
-- Whether a changeset renames or moves a node, of all its items (M9/P3 design 3.20), which the web's revert warns of
-- (links written to the new place stay) and its conflicts offer to undo by hand: the columns its units set as they
-- end.
SELECT renames, moves FROM changesets WHERE id = sqlc.arg(id);

-- name: ListChanges :many
-- A changeset's changes by node id after after (M8 design 4.7): its items' and its versions' node ids, each a range of
-- its key, at most max_rows of each, merged; then each one's item, version, node and parents' names by key. A node
-- the purge took took its item and version with it (the foreign keys, the purge's order): every row has its node. A
-- parent the purge took has no name.
WITH ids AS (
    SELECT u.node_id FROM (
        (SELECT i.node_id FROM changeset_items i WHERE i.changeset_id = sqlc.arg(id) AND i.node_id > sqlc.arg(after)::uuid
            ORDER BY i.node_id LIMIT sqlc.arg(max_rows))
        UNION
        (SELECT r.node_id FROM page_revisions r WHERE r.changeset_id = sqlc.arg(id) AND r.node_id > sqlc.arg(after)::uuid
            ORDER BY r.node_id LIMIT sqlc.arg(max_rows))
    ) u ORDER BY u.node_id LIMIT sqlc.arg(max_rows)
)
SELECT ids.node_id, n.kind, n.name, n.deleted_at,
    (i.node_id IS NOT NULL)::boolean AS has_item, i.before_parent_id, bp.name AS before_parent_name, i.before_name,
    i.before_sort_order, i.after_parent_id, ap.name AS after_parent_name, i.after_name, i.after_sort_order,
    coalesce(i.restored, false)::boolean AS restored, r.base_revision,
    r.revision
FROM ids
JOIN nodes n ON n.id = ids.node_id
LEFT JOIN changeset_items i ON i.changeset_id = sqlc.arg(id) AND i.node_id = ids.node_id
LEFT JOIN page_revisions r ON r.changeset_id = sqlc.arg(id) AND r.node_id = ids.node_id
LEFT JOIN nodes bp ON bp.id = i.before_parent_id
LEFT JOIN nodes ap ON ap.id = i.after_parent_id
ORDER BY ids.node_id;

-- name: FirstChanges :many
-- ListChanges' first n of each changeset, in one statement: a log page's summaries.
SELECT s.id::uuid AS changeset_id, ids.node_id, n.kind, n.name, n.deleted_at,
    (i.node_id IS NOT NULL)::boolean AS has_item, i.before_parent_id, bp.name AS before_parent_name, i.before_name,
    i.before_sort_order, i.after_parent_id, ap.name AS after_parent_name, i.after_name, i.after_sort_order,
    coalesce(i.restored, false)::boolean AS restored, r.base_revision,
    r.revision
FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id) s
CROSS JOIN LATERAL (
    SELECT u.node_id FROM (
        (SELECT x.node_id FROM changeset_items x WHERE x.changeset_id = s.id ORDER BY x.node_id LIMIT sqlc.arg(n))
        UNION
        (SELECT y.node_id FROM page_revisions y WHERE y.changeset_id = s.id ORDER BY y.node_id LIMIT sqlc.arg(n))
    ) u ORDER BY u.node_id LIMIT sqlc.arg(n)
) ids
JOIN nodes n ON n.id = ids.node_id
LEFT JOIN changeset_items i ON i.changeset_id = s.id AND i.node_id = ids.node_id
LEFT JOIN page_revisions r ON r.changeset_id = s.id AND r.node_id = ids.node_id
LEFT JOIN nodes bp ON bp.id = i.before_parent_id
LEFT JOIN nodes ap ON ap.id = i.after_parent_id
ORDER BY s.id, ids.node_id;

-- name: CountItems :one
-- A changeset's items counted to lim: a revert's tree operations, against its unit's bound, before it reads them.
SELECT count(*)::integer AS items FROM (SELECT 1 FROM changeset_items WHERE changeset_id = sqlc.arg(id) LIMIT sqlc.arg(lim)) x;

-- name: ChangesetItems :many
-- A changeset's items by node: what a revert puts back.
SELECT node_id, before_parent_id, before_name, before_sort_order, after_parent_id, after_name, after_sort_order,
    restored, revision
FROM changeset_items WHERE changeset_id = sqlc.arg(id) ORDER BY node_id;

-- name: ChangesetVersions :many
-- A changeset's versions by page, each with the size of the one it is based on: what a revert writes back.
-- The rewrite marker belongs to the reverted version, not its base (M10/P2 design 3.4).
SELECT r.node_id, r.base_revision, r.revision, r.content_hash, b.byte_size AS base_bytes, r.rewrite
FROM page_revisions r LEFT JOIN page_revisions b ON b.node_id = r.node_id AND b.revision = r.base_revision
WHERE r.changeset_id = sqlc.arg(id) ORDER BY r.node_id;

-- name: ChangesetItemsOf :many
-- A changeset's items of some nodes, by its key on (changeset_id, node_id): a background revert's batch's (M8/P4 design
-- 3.5), as ChangesetItems reads them all.
SELECT node_id, before_parent_id, before_name, before_sort_order, after_parent_id, after_name, after_sort_order,
    restored, revision
FROM changeset_items WHERE changeset_id = sqlc.arg(id) AND node_id = ANY(sqlc.arg(node_ids)::uuid[]) ORDER BY node_id;

-- name: ChangesetVersionsOf :many
-- A changeset's versions of some pages, as ChangesetVersions reads them all: a background revert's batch's.
SELECT r.node_id, r.base_revision, r.revision, r.content_hash, b.byte_size AS base_bytes, r.rewrite
FROM page_revisions r LEFT JOIN page_revisions b ON b.node_id = r.node_id AND b.revision = r.base_revision
WHERE r.changeset_id = sqlc.arg(id) AND r.node_id = ANY(sqlc.arg(node_ids)::uuid[]) ORDER BY r.node_id;

-- name: NodesNow :many
-- Nodes as they are now, alive or in the trash, by their key: a trash item's root with its counts, and whether its
-- deletion undid the restore of the trash item the changeset reverted made (M8/P3A review A-1, fix checks 1 and 2): it
-- is the revert of a changeset that restored the node, a trash restore or a revert, made after the changeset reverted
-- with no deletion of the node between them, so that what it restored was that changeset's item. Each is told by its
-- items' own times, not its changeset's (M8/P4 design 3.7): a background job's later batches write after its
-- changeset was made, by its first. An item that deletes its node is last written as it does, at the node's deletion's
-- time (a merge into it moves updated_at; the deletion's statement sets deleted_at alone); a restored node's item is
-- first written as it is restored (a merge keeps created_at). The root's deletion's item is read by the node's items'
-- index, the reverted changesets' items by their changesets' and the node's key, the deletions between by the node's
-- items' index.
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at, n.deleted_at, n.trash_root, n.trash_pages, n.trash_assets,
    CASE WHEN n.trash_root THEN EXISTS (
        SELECT 1 FROM changeset_items i
        JOIN changesets c ON c.id = i.changeset_id AND c.kind = 'revert'
        JOIN changeset_items ri ON ri.changeset_id = c.reverts_id AND ri.node_id = n.id AND ri.restored
        JOIN changeset_items di ON di.changeset_id = sqlc.arg(reverted)::uuid AND di.node_id = n.id
            AND di.after_name IS NULL AND di.updated_at < ri.created_at
        WHERE i.node_id = n.id AND i.after_name IS NULL AND i.updated_at = n.deleted_at
            AND NOT EXISTS (
                SELECT 1 FROM changeset_items x
                WHERE x.node_id = n.id AND x.after_name IS NULL
                    AND x.updated_at > di.updated_at AND x.updated_at < ri.created_at)
    ) ELSE false END::boolean AS deleted_by_revert
FROM nodes n WHERE n.id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ContentsNow :many
-- The contents of pages alive, their revisions, hashes and byte sizes, by their key, without reading the bodies.
SELECT node_id, revision, content_hash, byte_size FROM page_contents
WHERE node_id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

-- name: VersionHashes :many
-- The hashes of pages' versions, each by its page and revision through their unique index, deleted or not.
SELECT r.node_id, r.revision, r.content_hash
FROM (SELECT unnest(sqlc.arg(node_ids)::uuid[]) AS node_id, unnest(sqlc.arg(revisions)::integer[]) AS revision) v
JOIN page_revisions r ON r.node_id = v.node_id AND r.revision = v.revision;

-- name: VersionContents :many
-- VersionHashes with the versions' contents: what a revert writes back, past its unit's bounds.
SELECT r.node_id, r.revision, r.content_hash, r.content
FROM (SELECT unnest(sqlc.arg(node_ids)::uuid[]) AS node_id, unnest(sqlc.arg(revisions)::integer[]) AS revision) v
JOIN page_revisions r ON r.node_id = v.node_id AND r.revision = v.revision;

-- name: NotebookLastChange :one
-- When a notebook's tree was last written, the pages' part of its activity on the web (activity.sql): the latest
-- updated_at of its changesets, which an edit session's later writes move on (M9/P1 design 3.17.3). GROUP BY answers
-- no row for a notebook without a changeset, which the store maps to nil, so the column is never NULL.
SELECT max(c.updated_at)::timestamptz AS last_change
FROM changesets c
WHERE c.notebook_id = sqlc.arg(notebook_id) AND c.deleted_at IS NULL
GROUP BY c.notebook_id;
