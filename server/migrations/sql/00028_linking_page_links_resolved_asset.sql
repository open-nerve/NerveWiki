-- page_links.resolved_asset: whether a link resolves to an attachment, not a page (M7/P3 design 4.3), which the
-- reading view and the property links tell apart. A node's kind never changes, so it follows resolved_id; before it,
-- a link resolved to a page or none.

-- +goose Up
ALTER TABLE page_links ADD COLUMN resolved_asset boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT page_links_resolved_asset_check CHECK (NOT resolved_asset OR resolved_id IS NOT NULL);

-- +goose Down
-- A program before it reads every resolved link as a page's: the links to attachments resolve to none, as they did.
UPDATE page_links SET resolved_id = NULL, ambiguous = false WHERE resolved_asset;
ALTER TABLE page_links DROP COLUMN resolved_asset;
