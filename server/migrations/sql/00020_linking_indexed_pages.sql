-- indexed_pages: which version of each page's content the link index holds (M6 design 4.3; M6/P3 design 3.2),
-- and by which version of the extraction. Derived data, rebuilt by nervewiki reindex. No foreign key to the
-- page's tables, as edit_sessions has none: deleting a page, its subtree or its notebook deletes its rows in
-- their transaction, so a row never outlives its page, and the purge has nothing to order it by.

-- +goose Up
CREATE TABLE indexed_pages (
    node_id uuid PRIMARY KEY,
    -- The page's notebook, which a notebook's deletion and reindex go by.
    notebook_id uuid NOT NULL,
    revision integer NOT NULL CONSTRAINT indexed_pages_revision_check CHECK (revision >= 1),
    -- linking/domain's version of the extraction: a release that changes it asks for a reindex.
    extractor integer NOT NULL CONSTRAINT indexed_pages_extractor_check CHECK (extractor >= 1),
    -- Whether the content's frontmatter, if any, is valid: an invalid one has no properties, tags or aliases.
    frontmatter_valid boolean NOT NULL
);
CREATE INDEX indexed_pages_notebook_id_idx ON indexed_pages (notebook_id);

-- +goose Down
DROP TABLE indexed_pages;
