-- nodes.source_hash is the current hash of a nonempty page or any attachment
-- strictly inside a live LLM Wiki's source area (M10/P2 design 3.7, 3.9).
-- System rewrites may make existing sources equal, so the index is not unique.

-- +goose Up
ALTER TABLE nodes ADD COLUMN source_hash bytea,
    ADD CONSTRAINT nodes_source_hash_check CHECK (octet_length(source_hash) = 32);
CREATE INDEX nodes_notebook_id_kind_source_hash_idx
    ON nodes (notebook_id, kind, source_hash, id)
    WHERE source_hash IS NOT NULL AND deleted_at IS NULL;

-- Migration-only cross-module read exception: notebooks identifies the live
-- LLM roots and asset_blobs supplies attachment hashes. Only page.nodes is
-- written. Runtime access keeps the notebook and attachment ports instead.
-- Missing roots or metadata belong to an already invalid database; do not
-- invent their hashes. Traverse live children only, including every level's
-- notebook bound, and exclude the source root itself.
-- +goose StatementBegin
DO $$
DECLARE wiki record;
BEGIN
    FOR wiki IN SELECT id, source_root_id FROM notebooks
                WHERE mode = 'llm_wiki' AND deleted_at IS NULL LOOP
        WITH RECURSIVE area(id) AS (
            SELECT id FROM nodes WHERE id = wiki.source_root_id
                AND notebook_id = wiki.id AND deleted_at IS NULL
            UNION ALL
            SELECT n.id FROM nodes n JOIN area a ON n.parent_id = a.id
                WHERE n.notebook_id = wiki.id AND n.deleted_at IS NULL
        ), values_to_set AS (
            SELECT n.id, CASE
                WHEN n.kind = 'page' AND c.byte_size > 0 THEN c.content_hash
                WHEN n.kind = 'asset' THEN b.sha256
                ELSE NULL::bytea END AS hash
            FROM area a JOIN nodes n ON n.id = a.id
            LEFT JOIN page_contents c ON c.node_id = n.id
            LEFT JOIN asset_blobs b ON b.node_id = n.id AND b.notebook_id = wiki.id
            WHERE n.id <> wiki.source_root_id
        )
        UPDATE nodes n SET source_hash = v.hash
            FROM values_to_set v WHERE n.id = v.id AND n.notebook_id = wiki.id
                AND v.hash IS NOT NULL;
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- The column owns its check and index; another Up derives the values again.
ALTER TABLE nodes DROP COLUMN source_hash;
