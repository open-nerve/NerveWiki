-- name: Analyze :exec
-- The statistics of the tables an import writes (M7/P6 design 3.6; M6 handoff, item 4): without them, LinksReached
-- scans a notebook's links.
ANALYZE indexed_pages, page_links, page_tags, page_properties, page_aliases;
