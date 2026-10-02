-- name: CreateNode :exec
-- The audit columns come from the write unit's clock and caller.
INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
    created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(notebook_id), sqlc.narg(parent_id), sqlc.arg(kind), sqlc.arg(name), sqlc.arg(name_key),
    sqlc.arg(sort_order), sqlc.arg(by), sqlc.arg(by), sqlc.arg(now), sqlc.arg(now));

-- name: FindNode :one
-- A node not deleted, unlocked: what a read authorizes against, and what a write reads before its locks.
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: FindNodeIn :one
-- FindNode within one notebook: what a write reads under the notebook's lock.
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE id = sqlc.arg(id) AND notebook_id = sqlc.arg(notebook_id) AND deleted_at IS NULL;

-- name: ListNodes :many
-- A notebook's nodes not deleted, in no particular order: the domain orders the tree.
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND deleted_at IS NULL;

-- name: Children :many
-- A parent's children not deleted (the root's when parent_id is NULL), in order.
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)
    AND deleted_at IS NULL
ORDER BY sort_order, id;

-- name: Ancestors :many
-- The ancestors of a node from its parent up to the root, nearest first. The depth bound stops a chain that
-- loops, which only a defect could make; the caller checks the chain reaches the root.
WITH RECURSIVE chain AS (
    SELECT p.id, p.parent_id, p.name, 1 AS hops
    FROM nodes n JOIN nodes p ON p.id = n.parent_id
    WHERE n.id = sqlc.arg(id)
    UNION ALL
    SELECT p.id, p.parent_id, p.name, c.hops + 1
    FROM chain c JOIN nodes p ON p.id = c.parent_id
    WHERE c.hops < 64
)
SELECT id, parent_id, name, hops::integer AS hops FROM chain ORDER BY hops;

-- name: RenameNode :exec
UPDATE nodes
SET name = sqlc.arg(name), name_key = sqlc.arg(name_key), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: SetSortOrder :exec
-- Renumbering a parent's children moves no one in the tree: no audit columns change.
UPDATE nodes SET sort_order = sqlc.arg(sort_order) WHERE id = sqlc.arg(id);

-- name: DeleteNotebooksPages :exec
-- The notebook deletion's registrant (M4/P1 design 3.9): the nodes not deleted of the notebooks, and what
-- follows them, at the deletion's time, by the deleter; then the notebooks' changesets. What a subtree's
-- deletion put in the trash before keeps its own time. The deletion holds the notebooks' rows FOR NO KEY
-- UPDATE: no page write of these notebooks runs beside it.
WITH gone AS (
    UPDATE nodes n
    SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
    WHERE n.notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]) AND n.deleted_at IS NULL
    RETURNING n.id
), contents AS (
    UPDATE page_contents c SET deleted_at = sqlc.arg(now)::timestamptz
    WHERE c.node_id IN (SELECT id FROM gone) AND c.deleted_at IS NULL
), revisions AS (
    UPDATE page_revisions r SET deleted_at = sqlc.arg(now)::timestamptz
    WHERE r.node_id IN (SELECT id FROM gone) AND r.deleted_at IS NULL
), items AS (
    UPDATE changeset_items i SET deleted_at = sqlc.arg(now)::timestamptz
    WHERE i.node_id IN (SELECT id FROM gone) AND i.deleted_at IS NULL
)
UPDATE changesets s SET deleted_at = sqlc.arg(now)::timestamptz
WHERE s.notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]) AND s.deleted_at IS NULL;
