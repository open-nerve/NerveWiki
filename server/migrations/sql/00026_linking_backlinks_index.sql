-- The links to a page by the page they are written in, which its backlinks are paged by; and a page's property
-- links, which its properties read (M6/P5 design 8). page_links_resolved_id_idx's queries, the links to a page
-- its move or deletion may change, use the first's leading column. The first holds the links' ends too, in what its
-- rows' alignment leaves free: a page's first contexts are read from it alone, which keeps it the plan's choice over
-- the primary key even where one page writes most of the links to a page (review c1).

-- +goose Up
CREATE INDEX page_links_resolved_id_source_id_idx ON page_links (resolved_id, source_id, range_start)
    INCLUDE (range_end) WHERE resolved_id IS NOT NULL;
DROP INDEX page_links_resolved_id_idx;
-- The primary key's columns, of the property links alone.
CREATE INDEX page_links_source_id_range_start_idx ON page_links (source_id, range_start) WHERE property_key IS NOT NULL;

-- +goose Down
DROP INDEX page_links_source_id_range_start_idx;
CREATE INDEX page_links_resolved_id_idx ON page_links (resolved_id) WHERE resolved_id IS NOT NULL;
DROP INDEX page_links_resolved_id_source_id_idx;
