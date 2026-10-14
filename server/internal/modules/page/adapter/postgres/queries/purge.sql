-- The purge's statements (v0.1 design 13.1, item 6; M4/P1 design 3.10): each deletes up to batch rows
-- deleted before before, skipping those another transaction holds. Leaf to root: what follows a node
-- first, then the nodes, then the changesets.

-- name: PurgeChangesetItems :execrows
-- The items deleted before before, each locked with its changeset without waiting (M8 design 4.2): a row another
-- transaction holds, or whose changeset another holds (a unit merging into it), waits for a later run, and the purge
-- never waits (v0.1 design 13.1, item 5). The changesets of live notebooks whose items it deletes are marked trimmed,
-- once: they can no longer be reverted whole, which a revert reads after their items (M8/P3 design 3.3). The deletion
-- is the statement, so that its rows are what :execrows counts.
WITH doomed AS (
    SELECT i.id, i.changeset_id
    FROM changeset_items i JOIN changesets c ON c.id = i.changeset_id
    WHERE i.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch)
    FOR UPDATE OF i SKIP LOCKED FOR NO KEY UPDATE OF c SKIP LOCKED
), marked AS (
    UPDATE changesets c SET trimmed = true
    WHERE c.id IN (SELECT d.changeset_id FROM doomed d) AND c.deleted_at IS NULL AND NOT c.trimmed
)
DELETE FROM changeset_items i USING doomed d WHERE i.id = d.id;

-- name: PurgePageRevisions :execrows
-- The versions deleted before before, as PurgeChangesetItems purges the items: each locked with its changeset, which is
-- marked trimmed in a live notebook.
WITH doomed AS (
    SELECT r.id, r.changeset_id
    FROM page_revisions r JOIN changesets c ON c.id = r.changeset_id
    WHERE r.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch)
    FOR UPDATE OF r SKIP LOCKED FOR NO KEY UPDATE OF c SKIP LOCKED
), marked AS (
    UPDATE changesets c SET trimmed = true
    WHERE c.id IN (SELECT d.changeset_id FROM doomed d) AND c.deleted_at IS NULL AND NOT c.trimmed
)
DELETE FROM page_revisions r USING doomed d WHERE r.id = d.id;

-- name: PurgePageContents :execrows
DELETE FROM page_contents WHERE node_id IN (
    SELECT c.node_id FROM page_contents c
    WHERE c.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch) FOR UPDATE SKIP LOCKED);

-- name: PurgeNodeLeaves :execrows
-- The nodes that are leaves now: no child, deleted or not, and nothing that follows them, which an earlier
-- purger may have skipped. One statement sees the rows as they were when it began, so it takes one level of
-- a tree; the store runs it again until the batch is full or nothing is left.
DELETE FROM nodes WHERE id IN (
    SELECT n.id FROM nodes n
    WHERE n.deleted_at < sqlc.arg(before)::timestamptz
        AND NOT EXISTS (SELECT 1 FROM nodes c WHERE c.notebook_id = n.notebook_id AND c.parent_id = n.id)
        AND NOT EXISTS (SELECT 1 FROM page_contents x WHERE x.node_id = n.id)
        AND NOT EXISTS (SELECT 1 FROM page_revisions x WHERE x.node_id = n.id)
        AND NOT EXISTS (SELECT 1 FROM changeset_items x WHERE x.node_id = n.id)
    LIMIT sqlc.arg(batch) FOR UPDATE SKIP LOCKED);

-- name: PurgeChangesets :execrows
-- The changesets deleted with their notebook whose items and versions are gone.
DELETE FROM changesets WHERE id IN (
    SELECT s.id FROM changesets s
    WHERE s.deleted_at < sqlc.arg(before)::timestamptz
        AND NOT EXISTS (SELECT 1 FROM changeset_items x WHERE x.changeset_id = s.id)
        AND NOT EXISTS (SELECT 1 FROM page_revisions x WHERE x.changeset_id = s.id)
    LIMIT sqlc.arg(batch) FOR UPDATE SKIP LOCKED);

-- name: PurgeTrimmedChangesets :execrows
-- The trimmed changesets of live notebooks whose items and versions the purge has all deleted: nothing of them is left
-- to read or revert (M8/P3 design 3.3). Through the partial index on trimmed; at any time, as nothing deleted them. One
-- whose notebook was deleted since waits for the notebook's retention, as PurgeChangesets takes it (M8/P3A review A-6).
DELETE FROM changesets WHERE id IN (
    SELECT s.id FROM changesets s
    WHERE s.trimmed AND s.deleted_at IS NULL
        AND NOT EXISTS (SELECT 1 FROM changeset_items x WHERE x.changeset_id = s.id)
        AND NOT EXISTS (SELECT 1 FROM page_revisions x WHERE x.changeset_id = s.id)
    LIMIT sqlc.arg(batch) FOR UPDATE SKIP LOCKED);
