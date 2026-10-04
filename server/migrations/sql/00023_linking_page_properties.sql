-- page_properties: the properties of each page's frontmatter, in the order written (M6 design 4.3; M6/P3
-- design 3.2). Derived data, rebuilt by nervewiki reindex; no foreign key, as indexed_pages.

-- +goose Up
CREATE TABLE page_properties (
    source_id uuid NOT NULL,
    position integer NOT NULL CONSTRAINT page_properties_position_check CHECK (position >= 0),
    notebook_id uuid NOT NULL,
    key text NOT NULL,
    -- Its value as the fixture set's JSON writes it.
    value jsonb NOT NULL,
    PRIMARY KEY (source_id, position)
);
-- A notebook's properties, which its deletion and reindex go by.
CREATE INDEX page_properties_notebook_id_idx ON page_properties (notebook_id);

-- +goose Down
DROP TABLE page_properties;
