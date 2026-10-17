-- nodes by a path's segments (M9/P1 design 3.9.2), and a name one among its siblings by two unique indexes in place
-- of the siblings' one (00012), dropped here (M9/P1 S2 reviews, A-I1 and C-I2; fix check A).
-- RootByKey reads a root by the roots' index, its two columns equalities: a point lookup with statistics or without,
-- custom plan or generic. Through the siblings' index a root's parent_id IS NULL is no equality, so a scan of it was
-- costed as the key's nodes in the notebook: a common key's root was read by a scan of the table once analyzed.
-- ChildByKey reads a child by the children's index, its two columns equalities, once the table is analyzed. Before,
-- the planner may take the titles' index (00019) instead and read the notebook's nodes of the key: deduplicated, it
-- is cheaper to descend than any unique index of the children, which no definition measured changed (fix check A,
-- A2-M1). Autovacuum's first ANALYZE of the table ends that.
-- Together the two keep the siblings' index's rule, row by row: a key one among a notebook's roots, and one among a
-- parent's children, which are in the parent's notebook (the composite key nodes_notebook_id_parent_id_fkey).

-- +goose Up
CREATE UNIQUE INDEX nodes_notebook_id_name_key_root_key ON nodes (notebook_id, name_key)
    WHERE parent_id IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX nodes_parent_id_name_key_key ON nodes (parent_id, name_key)
    WHERE parent_id IS NOT NULL AND deleted_at IS NULL;
DROP INDEX nodes_notebook_id_parent_id_name_key_idx;

-- +goose Down
CREATE UNIQUE INDEX nodes_notebook_id_parent_id_name_key_idx ON nodes (notebook_id, parent_id, name_key)
    NULLS NOT DISTINCT WHERE deleted_at IS NULL;
DROP INDEX nodes_parent_id_name_key_key;
DROP INDEX nodes_notebook_id_name_key_root_key;
