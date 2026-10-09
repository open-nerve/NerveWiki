-- name: Analyze :exec
-- The statistics of the tables an import writes (M7/P6 design 3.6): without them, after thousands of rows, the
-- planner reads the trees as though they were empty.
ANALYZE nodes, page_contents, page_revisions, changesets, changeset_items;
