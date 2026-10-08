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

-- name: AssetsUnder :many
-- A parent's attachments not deleted (the root's when parent_id is NULL), by title key and id, after the cursor's key
-- and id when it has one, at most max_rows: a page of the attachments' list (M7/P2 design 3.3). The siblings' unique
-- index on their title keys serves it.
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)
    AND kind = 'asset' AND deleted_at IS NULL
    AND (sqlc.narg(after_key)::text IS NULL OR (name_key, id) > (sqlc.narg(after_key)::text, sqlc.narg(after_id)::uuid))
ORDER BY name_key, id
LIMIT sqlc.arg(max_rows);

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

-- name: ChildrenOfAll :many
-- The children not deleted of parents, of the notebook, in order: a level of a subtree, read a level a statement and
-- planned with its parents each time (Store.Subtree, postgres.Planned). One recursive statement is planned whole, and
-- some of its plans read the whole table for each parent or each level: without statistics, the notebook by the titles'
-- partial index for each parent, 40 s for a folder of 10,000 (M6 closeout FA-M1); with them and one folder holding most
-- nodes, each level a scan of every notebook's nodes (FA2-I1, FA3-M1). A plan for any parents, without statistics,
-- reads the notebook by the titles' index for each level and compares each row with the parents one by one (FA4-M1).
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id = ANY(sqlc.arg(parents)::uuid[]) AND deleted_at IS NULL
ORDER BY sort_order, id;

-- name: RenameNode :exec
UPDATE nodes
SET name = sqlc.arg(name), name_key = sqlc.arg(name_key), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: SetSortOrder :exec
-- Renumbering a parent's children moves no one in the tree: no audit columns change.
UPDATE nodes SET sort_order = sqlc.arg(sort_order) WHERE id = sqlc.arg(id);

-- name: MoveNode :exec
UPDATE nodes
SET parent_id = sqlc.narg(parent_id), sort_order = sqlc.arg(sort_order), updated_by_id = sqlc.arg(by),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: DeleteNodes :exec
-- A subtree's deletion (M4 design 4): the nodes not deleted, and what follows them not deleted, at the
-- unit's time, by its caller. What an earlier deletion put in the trash keeps its own time.
WITH gone AS (
    UPDATE nodes n
    SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
    WHERE n.id = ANY(sqlc.arg(ids)::uuid[]) AND n.deleted_at IS NULL
    RETURNING n.id
), contents AS (
    UPDATE page_contents c SET deleted_at = sqlc.arg(now)::timestamptz
    WHERE c.node_id IN (SELECT id FROM gone) AND c.deleted_at IS NULL
), revisions AS (
    UPDATE page_revisions r SET deleted_at = sqlc.arg(now)::timestamptz
    WHERE r.node_id IN (SELECT id FROM gone) AND r.deleted_at IS NULL
)
UPDATE changeset_items i SET deleted_at = sqlc.arg(now)::timestamptz
WHERE i.node_id IN (SELECT id FROM gone) AND i.deleted_at IS NULL;

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
