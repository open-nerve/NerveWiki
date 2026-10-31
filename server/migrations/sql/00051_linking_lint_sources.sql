-- Lint reads a page's first ASCII-case-insensitive sources key in position order.
-- Keep unrelated properties and JSON values out of this index (M10/P3 3.5).
-- The ordinary build scans page_properties under SHARE: writes wait, reads pass.

-- +goose Up
CREATE INDEX page_properties_sources_idx ON page_properties (source_id, position)
    WHERE translate(key, 'SOURCE', 'source') = 'sources';

-- +goose Down
DROP INDEX page_properties_sources_idx;
