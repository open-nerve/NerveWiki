-- nodes: the trash (M8 design 4.2; M8/P2 design 3.2). A deletion's root is a trash item: its trash_root marks it, and
-- it holds its group's counts and height, the pages and attachments deleted with it at its time and how many levels of
-- pages they span, which the deletion knows and the group keeps (nothing in the trash changes): the trash lists the
-- roots alone, and a restore too large is refused before its group is read. A notebook's deletion marks none: the
-- trash lists the live notebooks' alone.

-- +goose Up
ALTER TABLE nodes ADD COLUMN trash_root boolean NOT NULL DEFAULT false,
    ADD COLUMN trash_pages integer, ADD COLUMN trash_assets integer, ADD COLUMN trash_height integer;
-- The roots deleted so far: those whose parent is none or was not deleted at their time, and not deleted with their
-- notebook. A notebook's deletion gives its changesets its nodes' time (DeleteNotebooksPages), and a subtree's
-- deletion never deletes a changeset: that tells them apart without reading the notebook module's table. The purge
-- deletes a notebook's changesets after its nodes, so leftovers of a deleted notebook being purged may be marked:
-- harmless, the trash lists the live notebooks' alone.
UPDATE nodes n SET trash_root = true
WHERE n.deleted_at IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM changesets s WHERE s.notebook_id = n.notebook_id AND s.deleted_at = n.deleted_at)
    AND (n.parent_id IS NULL OR n.deleted_at IS DISTINCT FROM (SELECT p.deleted_at FROM nodes p WHERE p.id = n.parent_id));
WITH RECURSIVE g AS (
    SELECT id AS root, id, kind, deleted_at, 1 AS level FROM nodes WHERE trash_root
    UNION ALL
    SELECT g.root, c.id, c.kind, c.deleted_at, g.level + 1
    FROM g JOIN nodes c ON c.parent_id = g.id AND c.deleted_at = g.deleted_at
)
UPDATE nodes n SET trash_pages = s.pages, trash_assets = s.assets, trash_height = s.height
FROM (SELECT root, count(*) FILTER (WHERE kind = 'page') AS pages, count(*) FILTER (WHERE kind = 'asset') AS assets,
        coalesce(max(level) FILTER (WHERE kind = 'page'), 0) AS height
      FROM g GROUP BY root) s
WHERE n.id = s.root;
-- Only a deleted node is a root, and a root alone has the counts: a group holds its root at least.
ALTER TABLE nodes ADD CONSTRAINT nodes_trash_root_check CHECK (NOT trash_root OR deleted_at IS NOT NULL),
    ADD CONSTRAINT nodes_trash_counts_check CHECK (trash_root = (trash_pages IS NOT NULL)
        AND trash_root = (trash_assets IS NOT NULL) AND trash_root = (trash_height IS NOT NULL)
        AND trash_pages >= 0 AND trash_assets >= 0 AND trash_height >= 0 AND trash_pages + trash_assets >= 1);
-- A notebook's trash, newer first: a range of the roots alone.
CREATE INDEX nodes_notebook_id_deleted_at_id_idx ON nodes (notebook_id, deleted_at DESC, id DESC) WHERE trash_root;

-- +goose Down
DROP INDEX nodes_notebook_id_deleted_at_id_idx;
ALTER TABLE nodes DROP CONSTRAINT nodes_trash_counts_check, DROP CONSTRAINT nodes_trash_root_check,
    DROP COLUMN trash_height, DROP COLUMN trash_assets, DROP COLUMN trash_pages, DROP COLUMN trash_root;
