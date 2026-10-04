-- page_aliases: the aliases of each page, from its frontmatter's aliases, by which a name alone resolves (v0.1
-- design 4.4; M6/P3 design 2). Derived data, rebuilt by nervewiki reindex; no foreign key, as indexed_pages.

-- +goose Up
CREATE TABLE page_aliases (
    source_id uuid NOT NULL,
    -- The alias's title key, which a link's name compares by, and the alias as written.
    alias_key text COLLATE "C" NOT NULL CONSTRAINT page_aliases_alias_key_check CHECK (alias_key <> ''),
    notebook_id uuid NOT NULL,
    alias text NOT NULL CONSTRAINT page_aliases_alias_check CHECK (alias <> ''),
    PRIMARY KEY (source_id, alias_key)
);
-- The pages of a notebook with an alias.
CREATE INDEX page_aliases_notebook_id_alias_key_idx ON page_aliases (notebook_id, alias_key);

-- +goose Down
DROP TABLE page_aliases;
