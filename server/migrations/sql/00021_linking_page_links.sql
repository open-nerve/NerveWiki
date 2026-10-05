-- page_links: the links of each page's content and where each resolves to (M6 design 4.3; M6/P3 design 3.2).
-- Derived data, rebuilt by nervewiki reindex; no foreign key, as indexed_pages.

-- +goose Up
CREATE TABLE page_links (
    -- The page the link is written in, and where its target is written in the content, in bytes: one link a
    -- range (the fixture set's links).
    source_id uuid NOT NULL,
    range_start integer NOT NULL CONSTRAINT page_links_range_start_check CHECK (range_start >= 0),
    range_end integer NOT NULL,
    notebook_id uuid NOT NULL,
    kind text NOT NULL CONSTRAINT page_links_kind_check CHECK (kind IN ('wikilink', 'embed', 'link', 'image')),
    -- A property link's property path ("sources.0"); NULL in the body.
    property_key text,
    target text NOT NULL CONSTRAINT page_links_target_check CHECK (target <> ''),
    anchor text,
    display text,
    -- The title key of the target's last segment without ".md", and with it when written so: the pages a link
    -- may resolve to have one of them. NULL: the target resolves to nothing whatever the tree.
    target_key text COLLATE "C",
    target_alt_key text COLLATE "C",
    -- The page it resolves to, NULL for none; ambiguous when pages alike in every preference tied.
    resolved_id uuid,
    ambiguous boolean NOT NULL,
    PRIMARY KEY (source_id, range_start),
    CONSTRAINT page_links_range_check CHECK (range_end > range_start),
    CONSTRAINT page_links_ambiguous_check CHECK (NOT ambiguous OR resolved_id IS NOT NULL)
);
-- The links a page appearing, going or renamed may change, by the keys of its names; and a notebook's links,
-- which its deletion and reindex go by.
CREATE INDEX page_links_notebook_id_target_key_idx ON page_links (notebook_id, target_key);
CREATE INDEX page_links_notebook_id_target_alt_key_idx ON page_links (notebook_id, target_alt_key)
    WHERE target_alt_key IS NOT NULL;
-- The links to a page: its backlinks, and those its move or deletion may change.
CREATE INDEX page_links_resolved_id_idx ON page_links (resolved_id) WHERE resolved_id IS NOT NULL;

-- +goose Down
DROP TABLE page_links;
