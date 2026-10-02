-- page_contents: a page's Markdown, byte for byte as written, and its revision (v0.1 design 3.6; M4/P1
-- design 3.4). One row per page.

-- +goose Up
CREATE TABLE page_contents (
    -- The purge deletes it before its node; the cascade is the fallback within the module.
    node_id uuid PRIMARY KEY REFERENCES nodes ON DELETE CASCADE,
    content text NOT NULL,
    -- Each write adds one: what a write's base_revision is compared with.
    revision integer NOT NULL CONSTRAINT page_contents_revision_check CHECK (revision >= 1),
    -- SHA-256 of content.
    content_hash bytea NOT NULL CONSTRAINT page_contents_content_hash_check CHECK (octet_length(content_hash) = 32),
    -- A page holds at most 5 MB (v0.1 design 3.6).
    byte_size integer NOT NULL
        CONSTRAINT page_contents_byte_size_check CHECK (byte_size = octet_length(content) AND byte_size <= 5242880),
    updated_by_id uuid NOT NULL REFERENCES users,
    updated_at timestamptz NOT NULL,
    -- Set with its node's, in the same transaction.
    deleted_at timestamptz
);
CREATE INDEX page_contents_deleted_at_idx ON page_contents (deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP TABLE page_contents;
