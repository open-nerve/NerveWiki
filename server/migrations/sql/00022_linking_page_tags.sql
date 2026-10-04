-- page_tags: the tags of each page's content, the body's and the frontmatter's tags (M6 design 4.3; M6/P3
-- design 3.2). Derived data, rebuilt by nervewiki reindex; no foreign key, as indexed_pages.

-- +goose Up
CREATE TABLE page_tags (
    source_id uuid NOT NULL,
    -- The tag's title key, which tags compare by, and the tag as first written.
    tag_key text COLLATE "C" NOT NULL CONSTRAINT page_tags_tag_key_check CHECK (tag_key <> ''),
    notebook_id uuid NOT NULL,
    tag text NOT NULL CONSTRAINT page_tags_tag_check CHECK (tag <> ''),
    -- How often the page writes it.
    count integer NOT NULL CONSTRAINT page_tags_count_check CHECK (count >= 1),
    PRIMARY KEY (source_id, tag_key)
);
-- The pages of a notebook with a tag.
CREATE INDEX page_tags_notebook_id_tag_key_idx ON page_tags (notebook_id, tag_key);

-- +goose Down
DROP TABLE page_tags;
