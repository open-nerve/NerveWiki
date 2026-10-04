-- The pages of a notebook by their title key, which a link's last segment names (M6/P3 design 3.3): the link
-- index looks its candidates up by it.

-- +goose Up
CREATE INDEX nodes_notebook_id_name_key_idx ON nodes (notebook_id, name_key) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX nodes_notebook_id_name_key_idx;
