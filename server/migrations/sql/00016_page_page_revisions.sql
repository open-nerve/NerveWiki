-- page_revisions: a page's content as each changeset left it (v0.1 design 3.8; M4/P1 design 3.4). One row
-- per changeset and page: an edit session's later saves update its row. Every version is kept.

-- +goose Up
CREATE TABLE page_revisions (
    id uuid PRIMARY KEY,
    -- Within the module: the purge deletes the versions first; the cascades are the fallback.
    changeset_id uuid NOT NULL REFERENCES changesets ON DELETE CASCADE,
    node_id uuid NOT NULL REFERENCES nodes ON DELETE CASCADE,
    -- The revision the write was based on; NULL when the changeset created the page.
    base_revision integer,
    -- The revision the write left.
    revision integer NOT NULL,
    content text NOT NULL,
    content_hash bytea NOT NULL CONSTRAINT page_revisions_content_hash_check CHECK (octet_length(content_hash) = 32),
    byte_size integer NOT NULL
        CONSTRAINT page_revisions_byte_size_check CHECK (byte_size = octet_length(content) AND byte_size <= 5242880),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- Set with its node's: history reads it regardless; it marks when the purge may take it.
    deleted_at timestamptz,
    CONSTRAINT page_revisions_changeset_id_node_id_key UNIQUE (changeset_id, node_id),
    CONSTRAINT page_revisions_revision_check CHECK (
        revision >= 1 AND (base_revision IS NULL OR base_revision BETWEEN 1 AND revision - 1))
);
-- A page's versions: its deletion and the purge's look-up.
CREATE INDEX page_revisions_node_id_idx ON page_revisions (node_id);
CREATE INDEX page_revisions_deleted_at_idx ON page_revisions (deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP TABLE page_revisions;
