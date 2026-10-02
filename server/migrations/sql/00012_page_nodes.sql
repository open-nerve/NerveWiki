-- nodes: the tree of a notebook's pages, and from M7 its attachments (v0.1 design 3.5; M4/P1 design 3.4). A
-- node's path is its ancestors' names, never stored; its parent is in the same notebook.

-- +goose Up
CREATE TABLE nodes (
    id uuid PRIMARY KEY,
    -- Another module's table: the purge deletes a deleted notebook's nodes before it (v0.1 design 13.1, item 6).
    notebook_id uuid NOT NULL REFERENCES notebooks ON DELETE RESTRICT,
    -- NULL at the notebook's root. The composite key below keeps the parent in the same notebook; deleting a
    -- parent never takes its children along: the purge deletes leaves first.
    parent_id uuid,
    kind text NOT NULL CONSTRAINT nodes_kind_check CHECK (kind IN ('page', 'asset')),
    -- A page's title or an attachment's file name, as shared.CheckTitle gives it.
    name text NOT NULL CONSTRAINT nodes_name_check CHECK (octet_length(name) BETWEEN 1 AND 255),
    -- shared.TitleKey(name), which names compare by: computed by the application, byte-wise here.
    name_key text COLLATE "C" NOT NULL CONSTRAINT nodes_name_key_check CHECK (name_key <> ''),
    -- The order among siblings. NaN sorts above Infinity, so the upper bound keeps it out too.
    sort_order double precision NOT NULL
        CONSTRAINT nodes_sort_order_check CHECK (sort_order > '-Infinity' AND sort_order < 'Infinity'),
    created_by_id uuid NOT NULL REFERENCES users,
    updated_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- A subtree's deletion sets one time on all of it, and so does the notebook's.
    deleted_at timestamptz,
    CONSTRAINT nodes_parent_check CHECK (parent_id <> id),
    CONSTRAINT nodes_notebook_id_id_key UNIQUE (notebook_id, id),
    CONSTRAINT nodes_notebook_id_parent_id_fkey FOREIGN KEY (notebook_id, parent_id)
        REFERENCES nodes (notebook_id, id) ON DELETE RESTRICT
);
-- Siblings' names differ by their keys; two roots are siblings too, hence NULLS NOT DISTINCT.
CREATE UNIQUE INDEX nodes_notebook_id_parent_id_name_key_idx ON nodes (notebook_id, parent_id, name_key)
    NULLS NOT DISTINCT WHERE deleted_at IS NULL;
-- A notebook's nodes and a parent's children, deleted ones included: the tree, the purge's look-up of
-- children, and the composite key's checks.
CREATE INDEX nodes_notebook_id_parent_id_idx ON nodes (notebook_id, parent_id);
-- The purge's batches.
CREATE INDEX nodes_deleted_at_idx ON nodes (deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP TABLE nodes;
