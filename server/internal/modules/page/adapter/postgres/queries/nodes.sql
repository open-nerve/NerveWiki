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

-- name: FindAnyNode :one
-- FindNode alive or in the trash, with when it was deleted (M8/P1 design 3.4): a page's history reads either; and
-- whether it is a trash item's root (M8/P2 design 3.4).
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at, deleted_at, trash_root
FROM nodes
WHERE id = sqlc.arg(id);

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

-- name: RootByKey :one
-- The node not deleted at the notebook's root whose title key is name_key: a path's first segment (M9/P1 design
-- 3.9.2), by the roots' unique index (00042), its two columns equalities: a point lookup with statistics or without.
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id IS NULL AND name_key = sqlc.arg(name_key)
    AND deleted_at IS NULL;

-- name: ChildByKey :one
-- The child not deleted of parent_id whose title key is name_key: a path's next segment, by the children's unique
-- index (00042), its two columns equalities, the notebook checked on the row: a point lookup once the table is
-- analyzed. Without statistics the planner may read the notebook's nodes of the key by the titles' index (00019)
-- instead, 00042's header says why. IS NOT DISTINCT FROM would not use either (M9/P1 design 2.1, item 13).
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id = sqlc.arg(parent_id) AND name_key = sqlc.arg(name_key)
    AND deleted_at IS NULL;

-- name: AliveNodes :many
-- The nodes not deleted among ids (M9/P1 design 3.10.3): list's attachments, whose infos asset reads deleted or not.
-- The array is a page of list, at most 1,000: the store plans it with its argument.
SELECT id FROM nodes WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

-- name: RootPageIDs :many
-- The root's pages not deleted (M9/P4 design 3.9.1): index's parent "/", by an index of the root's nodes, the
-- notebook's and parents' (00012) or the roots' (00042) as the planner takes it, notebook_id = its condition and
-- parent_id IS NULL its condition or its predicate (TestTheChildPagesReadsUseTheirIndexes).
SELECT id FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id IS NULL AND kind = 'page' AND deleted_at IS NULL;

-- name: ChildPageIDs :many
-- A page's child pages not deleted (M9/P4 design 3.9.1): index's parent, by an index of a parent's children, the
-- notebook's and parents' (00012) or the children's (00042) as the planner takes it, parent_id = its condition.
SELECT id FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id = sqlc.arg(parent_id) AND kind = 'page' AND deleted_at IS NULL;

-- name: AgentsPage :one
-- The notebook's root node whose title key is name_key, TitleKey("AGENTS"), with its title now and its content's
-- revision when it is a page, 0 for none (M9 design 4.6; M9/P1 design 3.15.1): one point lookup by the roots' unique
-- index (00042), as RootByKey, and the content by its key. The revision is a subquery of the node found, not a join: a
-- key that many nodes share, an AGENTS page in each folder, is estimated as many rows once analyzed, and a join of
-- them read the contents of every notebook by a hash.
SELECT n.id, n.kind, n.name,
    coalesce((SELECT c.revision FROM page_contents c WHERE c.node_id = n.id AND c.deleted_at IS NULL), 0)::integer
        AS revision
FROM nodes n
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.parent_id IS NULL AND n.name_key = sqlc.arg(name_key)
    AND n.deleted_at IS NULL;

-- name: NotebookCounts :one
-- A notebook's pages and attachments not deleted (M9/P1 design 3.17.3): open_notebook's overview, read from the
-- notebook's nodes by an index of them.
SELECT
    count(*) FILTER (WHERE n.kind = 'page')::bigint AS pages,
    count(*) FILTER (WHERE n.kind = 'asset')::bigint AS attachments
FROM nodes n
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL;

-- name: FirstChildren :many
-- The root's first n children not deleted, in order: open_notebook's root, n = MaxOverviewRoot + 1 to tell whether
-- more follow (M9/P1 design 3.17.3). parent_id IS NULL is an index condition; IS NOT DISTINCT FROM is not.
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id IS NULL AND deleted_at IS NULL
ORDER BY sort_order, id
LIMIT sqlc.arg(n);

-- name: AssetsUnder :many
-- A parent's attachments not deleted (the root's when parent_id is NULL), by title key and id, after the cursor's key
-- and id when it has one, at most max_rows: a page of the attachments' list (M7/P2 design 3.3). As Children, it
-- reads the notebook's nodes and filters the parent, which IS NOT DISTINCT FROM does not narrow in an index.
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
-- One complete level of a subtree, in global level order, with no row limit. Deduplicate parents before visiting
-- their notebook/parent ranges. OFFSET 0 keeps the per-parent range separate; the live filter stays outside so a
-- live-only notebook/title index cannot replace that range when column statistics are absent (M10/P2 CI fix).
-- Store.Subtree uses DescribeExec, but force_generic_plan still applies to its unnamed statement. The former ANY
-- filter could compare every notebook row with every parent. Earlier recursive forms also scanned a table per
-- parent or level: 40 s for 10,000 without statistics (M6 FA-M1), and majority-parent regressions (FA2-I1/FA3-M1).
SELECT child.id, child.notebook_id, child.parent_id, child.kind, child.name, child.name_key,
    child.sort_order, child.created_by_id, child.updated_by_id, child.created_at, child.updated_at
FROM (
    SELECT DISTINCT parent_id
    FROM unnest(sqlc.arg(parents)::uuid[]) AS input(parent_id)
    WHERE parent_id IS NOT NULL
) AS p
CROSS JOIN LATERAL (
    SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key,
        n.sort_order, n.created_by_id, n.updated_by_id, n.created_at, n.updated_at, n.deleted_at
    FROM nodes n
    WHERE n.notebook_id = sqlc.arg(notebook_id)::uuid
      AND n.parent_id >= p.parent_id
      AND n.parent_id <= p.parent_id
      AND n.parent_id IS NOT NULL
    OFFSET 0
) AS child
WHERE child.deleted_at IS NULL
ORDER BY child.sort_order, child.id;

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

-- name: RelocateNode :exec
-- A node's parent, order and name at once (M9 design 4.10): one statement, the siblings' partial unique indexes
-- (00042) checking it under its last parent with its last key.
UPDATE nodes
SET parent_id = sqlc.narg(parent_id), sort_order = sqlc.arg(sort_order), name = sqlc.arg(name),
    name_key = sqlc.arg(name_key), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: DeleteNodes :one
-- A subtree's deletion (M4 design 4; M8/P2 design 3.3): the nodes not deleted, and what follows them not deleted, at the
-- unit's time, by its caller, the root marked as a trash item with its group's counts and height. What an earlier
-- deletion put in the trash keeps its own time and mark. stray counts the nodes deleted at this time already whose
-- parent this deletion deletes: a unit that deleted a descendant, then its ancestor, would leave the descendant a root
-- within the ancestor's group, which the ancestor's counts miss; its caller's defect (a unit deletes from the top
-- down). It reads the statement's snapshot, before the update, the nodes of this time through the partial index on
-- deleted_at.
WITH stray AS (
    SELECT count(*) AS n FROM nodes c
    WHERE c.deleted_at = sqlc.arg(now)::timestamptz AND c.parent_id = ANY(sqlc.arg(ids)::uuid[])
), gone AS (
    UPDATE nodes n
    SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now),
        source_hash = NULL,
        trash_root = (n.id = sqlc.arg(root)),
        trash_pages = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.arg(pages)::integer END,
        trash_assets = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.arg(assets)::integer END,
        trash_height = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.arg(height)::integer END
    WHERE n.id = ANY(sqlc.arg(ids)::uuid[]) AND n.deleted_at IS NULL
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
SELECT n::integer AS stray FROM stray;

-- name: RestoreNodes :one
-- A trash item's restore (M8/P2 design 3.5): its nodes still deleted at its time, each with its name and key, the root
-- under its parent at its order, by the unit's caller at its time, no root any more; then what was deleted with them
-- at that time: their contents, and their versions and items in every changeset, the deleting one's among them. A
-- content keeps its writer and time: it did not change. The counts tell what came back, for the unit to check that the
-- purge took nothing: the nodes; the pages' contents, and the versions at their contents' revisions; the versions that
-- created a page, and the versions whose base is not the version before them, a gap in its chain (checkPages' chain).
-- The siblings' unique indexes are checked a row at a time: the group's other nodes are deleted, out of them, until
-- they are updated, and their names were made unique beforehand.
-- The plans hold for an item of any size, at any statistics, planned with the arguments or for any (M8/P2 design
-- 3.11): each form replaced was measured reading rows by rows past the request's 15 seconds. What followed the nodes
-- is reached by its node's id from back, its time compared: back's, which the planner knows nothing of, not the
-- argument, whose rows the statistics estimate at one when they never knew the time (back then read whole for each
-- row of the time), and by IS NOT DISTINCT FROM, which no index serves (the index on deleted_at read for each node).
-- The counts join no CTE with another, whose rows the planner estimates at one: a set operation and a window.
-- Prepared source hashes, already checked by the unit's guards, share this node UPDATE. An absent value leaves
-- the column unchanged; an explicit NULL clears it. This avoids a second physical write of every restored source.
WITH u AS (
    SELECT unnest(sqlc.arg(ids)::uuid[]) AS id, unnest(sqlc.arg(names)::text[]) AS name,
        unnest(sqlc.arg(name_keys)::text[]) AS name_key,
        unnest(sqlc.arg(source_hashes)::bytea[]) AS source_hash, unnest(sqlc.arg(has_source_hashes)::boolean[]) AS has_source_hash
), back AS (
    UPDATE nodes n
    SET deleted_at = NULL, trash_root = false, trash_pages = NULL, trash_assets = NULL, trash_height = NULL,
        name = u.name, name_key = u.name_key, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now),
        source_hash = CASE WHEN u.has_source_hash THEN u.source_hash ELSE n.source_hash END,
        parent_id = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.narg(parent_id)::uuid ELSE n.parent_id END,
        sort_order = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.arg(sort_order)::float8 ELSE n.sort_order END
    FROM u
    WHERE n.id = u.id AND n.deleted_at = sqlc.arg(deleted_at)::timestamptz
    RETURNING n.id, sqlc.arg(deleted_at)::timestamptz AS deleted_at
), contents AS (
    UPDATE page_contents c SET deleted_at = NULL
    FROM back b
    WHERE c.node_id = b.id AND c.deleted_at IS NOT DISTINCT FROM b.deleted_at
    RETURNING c.node_id, c.revision
), revisions AS (
    UPDATE page_revisions r SET deleted_at = NULL
    FROM back b
    WHERE r.node_id = b.id AND r.deleted_at IS NOT DISTINCT FROM b.deleted_at
    RETURNING r.node_id, r.revision, r.base_revision
), items AS (
    UPDATE changeset_items i SET deleted_at = NULL
    FROM back b
    WHERE i.node_id = b.id AND i.deleted_at IS NOT DISTINCT FROM b.deleted_at
)
SELECT (SELECT count(*) FROM back)::integer AS nodes,
    (SELECT count(*) FROM contents)::integer AS contents,
    (SELECT count(*) FROM (SELECT node_id, revision FROM contents INTERSECT ALL
        SELECT node_id, revision FROM revisions) v)::integer AS current_versions,
    (SELECT count(*) FROM revisions WHERE base_revision IS NULL)::integer AS creations,
    (SELECT count(*) FROM (SELECT base_revision, lag(revision) OVER (PARTITION BY node_id ORDER BY revision) AS prior
        FROM revisions) v WHERE base_revision IS DISTINCT FROM prior)::integer AS gaps;

-- name: MarkTrashRoots :execrows
-- The nodes of a trash item a background restore left in the trash under parents it brought back (M8 design 4.10;
-- M8/P4 design 3.4): each its own trash item, with its part of the group's counts and height. Joined by key, as
-- RestoreNodes; a node no longer deleted at the time, or a root already, is not marked, which its caller counts.
UPDATE nodes n SET trash_root = true, trash_pages = u.pages, trash_assets = u.assets, trash_height = u.height
FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id, unnest(sqlc.arg(pages)::integer[]) AS pages,
        unnest(sqlc.arg(assets)::integer[]) AS assets, unnest(sqlc.arg(heights)::integer[]) AS height) u
WHERE n.id = u.id AND n.deleted_at = sqlc.arg(deleted_at)::timestamptz AND NOT n.trash_root;

-- name: DeleteNotebooksPages :exec
-- The notebook deletion's registrant (M4/P1 design 3.9): the nodes not deleted of the notebooks, and what
-- follows them, at the deletion's time, by the deleter; then the notebooks' changesets. What a subtree's
-- deletion put in the trash before keeps its own time. The deletion holds the notebooks' rows FOR NO KEY
-- UPDATE: no page write of these notebooks runs beside it.
WITH gone AS (
    UPDATE nodes n
    SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now), source_hash = NULL
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
