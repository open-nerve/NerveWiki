-- The trash (M8/P2 design 3.4): a notebook's deletions' roots, each with what was deleted with it, at its time.
--
-- A row read with a node deleted at its time is joined by its key, the time compared as not distinct: no index serves
-- that comparison. As an equality the planner could reach the row by its time, through the partial index on
-- deleted_at, and with the statistics read before a large deletion (none of its rows deleted) it did, each node reading
-- every row of the time: a background restore's plan of 45,000 pages took 55 s (M8/P4A). The nodes compared are
-- deleted and the times given deletions', so not distinct is equal.

-- name: ListTrash :many
-- A notebook's trash items deleted after since, newer first, below the cursor, at most max_rows: a range of the trash's
-- partial index. A page's content's revision with it, at its deletion's time: none for an attachment, or a content the
-- purge took. Who deleted it is its updated_by_id, which the deletion wrote.
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at, n.deleted_at, n.trash_pages, n.trash_assets, n.trash_height, c.revision
FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NOT DISTINCT FROM n.deleted_at
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.trash_root AND n.deleted_at > sqlc.arg(since)::timestamptz
    AND (n.deleted_at, n.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
ORDER BY n.deleted_at DESC, n.id DESC
LIMIT sqlc.arg(max_rows);

-- name: FindTrashRoot :one
-- The trash item of the root id, as ListTrash reads it.
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at, n.deleted_at, n.trash_pages, n.trash_assets, n.trash_height, c.revision
FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NOT DISTINCT FROM n.deleted_at
WHERE n.id = sqlc.arg(id) AND n.trash_root;

-- name: TrashRootOf :one
-- The root of the trash item a deleted node is in: up its parents, deleted at its time, to the first root. The bound
-- stops a chain that loops, which only a defect could make.
WITH RECURSIVE up AS (
    SELECT n.id, n.parent_id, n.trash_root, 0 AS hops FROM nodes n WHERE n.id = sqlc.arg(id) AND n.deleted_at IS NOT NULL
    UNION ALL
    SELECT p.id, p.parent_id, p.trash_root, u.hops + 1
    FROM up u JOIN nodes p ON p.id = u.parent_id
    WHERE NOT u.trash_root AND u.hops < 64
)
SELECT up.id FROM up WHERE up.trash_root;

-- name: TrashLines :many
-- Where trash items were: each root's ancestors, deleted or not, with when each was deleted, a row a step up, its
-- parent's the step 1. Each step reads its parent by its key alone, as LinkTargetsByIDs does, which no statistics
-- (a large deletion's) turn into a read of the notebook. The bound stops a chain that loops, which only a defect could
-- make: such a line reaches no root.
WITH RECURSIVE chain AS (
    SELECT n.id AS root_id, p.id, p.parent_id, p.name, p.deleted_at, 1 AS up
    FROM nodes n JOIN nodes p ON p.id = n.parent_id
    WHERE n.id = ANY(sqlc.arg(ids)::uuid[])
    UNION ALL
    SELECT c.root_id, p.id, p.parent_id, p.name, p.deleted_at, c.up + 1
    FROM chain c CROSS JOIN LATERAL (SELECT id, parent_id, name, deleted_at FROM nodes WHERE id = c.parent_id LIMIT 1) p
    WHERE c.up < 64
)
SELECT root_id, id, parent_id, name, deleted_at, up::integer AS up FROM chain ORDER BY root_id, up DESC;

-- name: TrashGroupRoot :one
-- A trash item's root of the notebook deleted at deleted_at, with a page's content's revision and size, as
-- TrashChildrenOfAll reads its descendants.
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at, c.revision, c.byte_size
FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NOT DISTINCT FROM n.deleted_at
WHERE n.id = sqlc.arg(id) AND n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at = sqlc.arg(deleted_at)::timestamptz;

-- name: TrashChildrenOfAll :many
-- The children of parents of the notebook deleted at deleted_at, in order, each page with its content's revision and
-- size, none when the purge took it: a level of a trash item's group, read a level a statement and planned with its
-- parents, as ChildrenOfAll reads a subtree's, through the parents' index (00012), which holds the deleted rows. A
-- descendant deleted earlier is not of the group: it is an item of its own.
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at, c.revision, c.byte_size
FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NOT DISTINCT FROM n.deleted_at
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.parent_id = ANY(sqlc.arg(parents)::uuid[])
    AND n.deleted_at = sqlc.arg(deleted_at)::timestamptz
ORDER BY n.sort_order, n.id;

-- name: TrashContents :many
-- The contents of pages deleted at deleted_at, read by their primary key or, by a plan for any arguments, the rows of
-- the time through the index on deleted_at: at most an item's pages either way.
SELECT node_id, content FROM page_contents
WHERE node_id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at = sqlc.arg(deleted_at)::timestamptz;

-- name: TrashHistory :one
-- How many versions, and items of every changeset, of nodes deleted at deleted_at were deleted with them, each of the
-- two counted to lim at most: a restore's rows besides the nodes', as many as the bound at most. Planned with ids, it
-- reads the rows of the time through the index on deleted_at, or the nodes' through the one on node_id; its store
-- never runs it as a cached statement (M8/P2A fix check 1, I-1).
SELECT ((SELECT count(*) FROM (SELECT 1 FROM page_revisions r
            WHERE r.node_id = ANY(sqlc.arg(ids)::uuid[]) AND r.deleted_at = sqlc.arg(deleted_at)::timestamptz
            LIMIT sqlc.arg(lim)::integer) v)
    + (SELECT count(*) FROM (SELECT 1 FROM changeset_items i
            WHERE i.node_id = ANY(sqlc.arg(ids)::uuid[]) AND i.deleted_at = sqlc.arg(deleted_at)::timestamptz
            LIMIT sqlc.arg(lim)::integer) w))::integer AS rows;

-- name: TrashNodesAt :many
-- Nodes of the notebook, each deleted at its own time, by key (M8/P4 design 3.3): many trash items' roots at once, or
-- a background job's batch's members; with a root's counts, and a page's content's revision and size as
-- TrashChildrenOfAll reads them. One no longer deleted at its time is not read.
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at, n.trash_root, n.trash_pages, n.trash_assets, n.trash_height, c.revision, c.byte_size
FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id, unnest(sqlc.arg(deleted_ats)::timestamptz[]) AS deleted_at) g
JOIN nodes n ON n.id = g.id AND n.deleted_at IS NOT DISTINCT FROM g.deleted_at
LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NOT DISTINCT FROM n.deleted_at
WHERE n.notebook_id = sqlc.arg(notebook_id);

-- name: TrashChildrenAt :many
-- The children of parents of the notebook, each deleted at its parent's time (M8/P4 design 3.3): a level of many trash
-- items' groups in one statement, through the parents' index as TrashChildrenOfAll reads one group's, in its order:
-- each group's part of the level is the one TrashGroup reads.
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at, c.revision, c.byte_size
FROM (SELECT unnest(sqlc.arg(parents)::uuid[]) AS id, unnest(sqlc.arg(deleted_ats)::timestamptz[]) AS deleted_at) p
JOIN nodes n ON n.notebook_id = sqlc.arg(notebook_id) AND n.parent_id = p.id AND n.deleted_at IS NOT DISTINCT FROM p.deleted_at
LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NOT DISTINCT FROM n.deleted_at
ORDER BY n.sort_order, n.id;

-- name: TrashShapeOfAll :many
-- A level of a trash item's group as a background restore's plan keeps it (M8/P4 design 3.4): no names, a page's
-- content's size; by parent, each parent's children in order, as TrashChildrenOfAll reads them.
SELECT n.id, n.parent_id, n.kind, c.byte_size
FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NOT DISTINCT FROM n.deleted_at
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.parent_id = ANY(sqlc.arg(parents)::uuid[])
    AND n.deleted_at = sqlc.arg(deleted_at)::timestamptz
ORDER BY n.parent_id, n.sort_order, n.id;
