-- nodes by a path's segments (M9/P1 design 3.9.2): RootByKey reads a root by its title key, ChildByKey a parent's
-- child by its, each a point lookup whatever the statistics (M9/P1 S2 review, A-I1 and C-I2). The siblings' unique
-- index (00012) keeps a key one among siblings, but the planner read neither lookup by it alone. A root's
-- parent_id IS NULL is no equality, so a scan of that index is costed as the key's nodes in the notebook: a common
-- key's root was read by a scan of the table once analyzed, by the titles' index (00019) before, every node of the
-- key. Without statistics, any of the notebook's indexes reads a child as one row, and the planner took the
-- narrowest: the titles' (every node of the key) or the parents' (every child of the parent). Each index here is
-- unique, each of its columns an equality in its lookup, and holds fewer entries than the others: the planner costs it
-- as the one row it reads, with statistics or without, custom plan or generic. Neither refuses a row the siblings'
-- index allows: a root's key is one among roots already, and a parent's children are in its notebook.

-- +goose Up
CREATE UNIQUE INDEX nodes_notebook_id_name_key_root_key ON nodes (notebook_id, name_key)
    WHERE parent_id IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX nodes_parent_id_name_key_key ON nodes (parent_id, name_key)
    WHERE parent_id IS NOT NULL AND deleted_at IS NULL;

-- +goose Down
DROP INDEX nodes_parent_id_name_key_key;
DROP INDEX nodes_notebook_id_name_key_root_key;
