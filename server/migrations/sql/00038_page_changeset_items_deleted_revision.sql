-- changeset_items: a deletion's revision (M8/P3 design 2.1, 3.2). An item's revision is the page's revision as the
-- item left it where its changeset wrote no version of it: a creation's, a restore's, and from M8/P3 a deletion's,
-- which the revert of the deletion compares the trash's content with (M8/P3 design 3.5). restored stays a restore's
-- alone.

-- +goose Up
ALTER TABLE changeset_items DROP CONSTRAINT changeset_items_restored_check,
    ADD CONSTRAINT changeset_items_restored_check CHECK (NOT restored OR (before_name IS NULL AND after_name IS NOT NULL)),
    ADD CONSTRAINT changeset_items_revision_state_check CHECK (revision IS NULL OR before_name IS NULL OR after_name IS NULL);
-- The deletions so far whose page is still in the trash at their time: its content there is the one deleted (a
-- deletion's item is written at the deletion's time, which an M8 deletion's one unit gives both). Those restored
-- since are left without: a revert of them finds the page changed. An attachment has no content.
UPDATE changeset_items i SET revision = c.revision
FROM page_contents c
WHERE i.after_name IS NULL AND i.revision IS NULL AND c.node_id = i.node_id AND c.deleted_at = i.updated_at;

-- +goose Down
UPDATE changeset_items SET revision = NULL WHERE after_name IS NULL;
ALTER TABLE changeset_items DROP CONSTRAINT changeset_items_revision_state_check, DROP CONSTRAINT changeset_items_restored_check,
    ADD CONSTRAINT changeset_items_restored_check CHECK ((NOT restored AND revision IS NULL)
        OR (before_name IS NULL AND after_name IS NOT NULL));
