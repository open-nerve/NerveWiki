-- changeset_items: the nodes a changeset changed in the tree, each with where it was before and after
-- (v0.1 design 3.8; M4/P1 design 3.4). One row per changeset and node: several changes of a node in one
-- changeset keep the first before and the last after.

-- +goose Up
CREATE TABLE changeset_items (
    id uuid PRIMARY KEY,
    -- Within the module: the purge deletes the items first; the cascades are the fallback.
    changeset_id uuid NOT NULL REFERENCES changesets ON DELETE CASCADE,
    node_id uuid NOT NULL REFERENCES nodes ON DELETE CASCADE,
    -- Before: NULL when the changeset created the node. No key on the parents: history outlives them.
    before_parent_id uuid,
    before_name text,
    before_sort_order double precision,
    -- After: NULL when the changeset deleted the node.
    after_parent_id uuid,
    after_name text,
    after_sort_order double precision,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- Set with its node's: history reads it regardless; it marks when the purge may take it.
    deleted_at timestamptz,
    CONSTRAINT changeset_items_changeset_id_node_id_key UNIQUE (changeset_id, node_id),
    -- A state is a name and an order, and a parent only with them; an item has one state at least.
    CONSTRAINT changeset_items_state_check CHECK (
        (before_name IS NULL) = (before_sort_order IS NULL) AND (after_name IS NULL) = (after_sort_order IS NULL)
        AND (before_name IS NOT NULL OR before_parent_id IS NULL) AND (after_name IS NOT NULL OR after_parent_id IS NULL)
        AND (before_name IS NOT NULL OR after_name IS NOT NULL))
);
-- A node's items: its deletion and the purge's look-up.
CREATE INDEX changeset_items_node_id_idx ON changeset_items (node_id);
CREATE INDEX changeset_items_deleted_at_idx ON changeset_items (deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP TABLE changeset_items;
