-- Lint streams a notebook's links in source/range order (M10/P3 3.5).
-- The full index also retains resolved links and every property origin.
-- Its ordinary build scans page_links under SHARE: writes wait, reads pass.

-- +goose Up
CREATE INDEX page_links_notebook_id_source_id_range_start_idx ON page_links(notebook_id,source_id,range_start);

-- +goose Down
DROP INDEX page_links_notebook_id_source_id_range_start_idx;
