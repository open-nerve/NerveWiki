-- page_links.aliases: whether a link is a value of its page's aliases, which a rename's rewrite leaves (M6/P4 design
-- 2). The extraction tells it, by the frontmatter's structure, as the index keeps the rest of a link; no release has
-- rows without it.

-- +goose Up
ALTER TABLE page_links ADD COLUMN aliases boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE page_links DROP COLUMN aliases;
