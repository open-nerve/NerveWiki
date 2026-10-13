-- changeset_items: a restore from the trash and a creation both leave a node with no before (M8 design 4.2; M8/P2
-- design 3.2). restored tells them apart; revision is the revision a creation or a restore left a page's content at
-- (NULL for an attachment): a restore writes no version, and the revert of one (M8/P3) finds the content it left by
-- it. An item merged in its changeset again keeps its first restored and revision.

-- +goose Up
ALTER TABLE changeset_items ADD COLUMN restored boolean NOT NULL DEFAULT false, ADD COLUMN revision integer;
-- The pages' creations so far: a page's creation writes revision 1, as the units record it.
UPDATE changeset_items i SET revision = 1
WHERE i.before_name IS NULL AND i.after_name IS NOT NULL
    AND EXISTS (SELECT 1 FROM page_revisions r
        WHERE r.changeset_id = i.changeset_id AND r.node_id = i.node_id AND r.base_revision IS NULL);
-- Only an item with no before and an after is a restore's or has a revision.
ALTER TABLE changeset_items
    ADD CONSTRAINT changeset_items_restored_check CHECK ((NOT restored AND revision IS NULL)
        OR (before_name IS NULL AND after_name IS NOT NULL)),
    ADD CONSTRAINT changeset_items_revision_check CHECK (revision >= 1);

-- +goose Down
ALTER TABLE changeset_items DROP CONSTRAINT changeset_items_revision_check, DROP CONSTRAINT changeset_items_restored_check,
    DROP COLUMN revision, DROP COLUMN restored;
