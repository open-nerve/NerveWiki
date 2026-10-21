-- page_properties' type and summary, read by index a probe a page (M9/P4 design 3.9.3): a page's first property whose
-- key is "type" (or "summary") but for ASCII case, whatever other properties it has. Without them index's lateral reads
-- every property of a page without a type, up to the frontmatter's 10,000 nodes. A key that matches but for ASCII case
-- has 16 (or 128) spellings, so each index holds a few rows a page at most.

-- +goose Up
CREATE INDEX page_properties_type_idx ON page_properties (source_id, position)
    WHERE translate(key, 'TYPE', 'type') = 'type';
CREATE INDEX page_properties_summary_idx ON page_properties (source_id, position)
    WHERE translate(key, 'SUMARY', 'sumary') = 'summary';

-- +goose Down
DROP INDEX page_properties_summary_idx;
DROP INDEX page_properties_type_idx;
