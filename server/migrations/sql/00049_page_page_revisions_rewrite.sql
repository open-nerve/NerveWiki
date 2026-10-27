-- page_revisions.rewrite records whether every write in this version was a system rewrite (M10/P2 design 3.4).
-- Old versions have no such evidence and remain ordinary writes; their contents and revision chains are kept.

-- +goose Up
ALTER TABLE page_revisions ADD COLUMN rewrite boolean NOT NULL DEFAULT false;

-- +goose Down
-- Going up again cannot recover lost markers: those versions remain false, so source undo becomes conservative.
ALTER TABLE page_revisions DROP COLUMN rewrite;
