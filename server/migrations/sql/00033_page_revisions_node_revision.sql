-- page_revisions: a page's version is addressed by the revision it wrote (M8/P1 design 3.2): one row a revision of a
-- page, deleted rows too, which the writes keep so (each writes revision + 1). The unique index lists a page's
-- versions newer-first and finds the one holding a revision; it covers the index on node_id alone, which a subtree's
-- deletion and the purge looked versions up by.

-- +goose Up
CREATE UNIQUE INDEX page_revisions_node_id_revision_key ON page_revisions (node_id, revision);
DROP INDEX page_revisions_node_id_idx;

-- +goose Down
CREATE INDEX page_revisions_node_id_idx ON page_revisions (node_id);
DROP INDEX page_revisions_node_id_revision_key;
