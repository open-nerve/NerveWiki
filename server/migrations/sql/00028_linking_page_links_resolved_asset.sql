-- page_links.resolved_asset: whether a link resolves to an attachment, not a page (M7/P3 design 4.3), which the
-- reading view and the property links tell apart. A node's kind never changes, so it follows resolved_id; before it,
-- a link resolved to a page or none.

-- +goose Up
ALTER TABLE page_links ADD COLUMN resolved_asset boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT page_links_resolved_asset_check CHECK (NOT resolved_asset OR resolved_id IS NOT NULL);

-- +goose Down
-- A program before it does not know attachments as targets: the links to them are set to resolve to none, so that none
-- reads an attachment's id as a page's. Its own rules resolve them after nervewiki reindex.
UPDATE page_links SET resolved_id = NULL, ambiguous = false WHERE resolved_asset;
ALTER TABLE page_links DROP COLUMN resolved_asset;
