-- asset_blobs: the file of each attachment (M7 design 4.2; M7/P2 design 3.4). An attachment is a node of kind
-- 'asset' and exactly one row here, written in the node's transaction; its bytes are a file of the store at
-- blobs/<id>. The row follows its node into the trash, and the purge deletes the file, then the row, before the
-- node: the key to nodes keeps the order. The key to nodes holds the notebook too, so a row's notebook is its
-- node's, which a key to notebooks would only repeat.

-- +goose Up
CREATE TABLE asset_blobs (
    -- The blob's id (UUIDv7), the name of its file.
    id uuid PRIMARY KEY,
    node_id uuid NOT NULL CONSTRAINT asset_blobs_node_id_key UNIQUE,
    notebook_id uuid NOT NULL,
    -- The type the server determined from the name's extension and the first bytes (M7/P2 design 3.5).
    mime text NOT NULL CONSTRAINT asset_blobs_mime_check CHECK (mime <> ''),
    byte_size bigint NOT NULL CONSTRAINT asset_blobs_byte_size_check CHECK (byte_size >= 0),
    sha256 bytea NOT NULL CONSTRAINT asset_blobs_sha256_check CHECK (octet_length(sha256) = 32),
    -- An image's size in pixels, when the server read it; both or neither.
    width integer CONSTRAINT asset_blobs_width_check CHECK (width > 0),
    height integer CONSTRAINT asset_blobs_height_check CHECK (height > 0),
    created_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    deleted_at timestamptz,
    CONSTRAINT asset_blobs_notebook_id_node_id_fkey FOREIGN KEY (notebook_id, node_id) REFERENCES nodes (notebook_id, id) ON DELETE RESTRICT,
    CONSTRAINT asset_blobs_size_check CHECK ((width IS NULL) = (height IS NULL))
);
-- A notebook's attachments not deleted: its activity, and a notebook's deletion.
CREATE INDEX asset_blobs_notebook_id_idx ON asset_blobs (notebook_id) WHERE deleted_at IS NULL;
-- The purge's batches.
CREATE INDEX asset_blobs_deleted_at_idx ON asset_blobs (deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP TABLE asset_blobs;
