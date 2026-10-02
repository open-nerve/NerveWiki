-- The purge's statements (v0.1 design 13.1, item 6; M4/P1 design 3.10): each deletes up to batch rows
-- deleted before before, skipping those another transaction holds. Leaf to root: what follows a node
-- first, then the nodes, then the changesets.

-- name: PurgeChangesetItems :execrows
DELETE FROM changeset_items WHERE id IN (
    SELECT i.id FROM changeset_items i
    WHERE i.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch) FOR UPDATE SKIP LOCKED);

-- name: PurgePageRevisions :execrows
DELETE FROM page_revisions WHERE id IN (
    SELECT r.id FROM page_revisions r
    WHERE r.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch) FOR UPDATE SKIP LOCKED);

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
-- The changesets whose items and versions are gone.
DELETE FROM changesets WHERE id IN (
    SELECT s.id FROM changesets s
    WHERE s.deleted_at < sqlc.arg(before)::timestamptz
        AND NOT EXISTS (SELECT 1 FROM changeset_items x WHERE x.changeset_id = s.id)
        AND NOT EXISTS (SELECT 1 FROM page_revisions x WHERE x.changeset_id = s.id)
    LIMIT sqlc.arg(batch) FOR UPDATE SKIP LOCKED);
