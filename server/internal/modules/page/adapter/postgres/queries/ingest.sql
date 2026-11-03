-- Derived ingestion history; page owns these statements.

-- name: AddIngestMarks :one
WITH owned AS MATERIALIZED (
    SELECT id, ingest_count FROM changesets
    WHERE id = sqlc.arg(changeset_id)::uuid
      AND notebook_id = sqlc.arg(notebook_id)::uuid AND kind = 'edit' AND deleted_at IS NULL
    FOR NO KEY UPDATE
), wanted AS MATERIALIZED (
    SELECT DISTINCT unnest(sqlc.arg(source_ids)::uuid[]) AS node_id
), chosen AS MATERIALIZED (
    SELECT w.node_id FROM wanted w
    WHERE NOT EXISTS (
        SELECT 1 FROM changeset_sources s
        WHERE s.changeset_id = sqlc.arg(changeset_id)::uuid AND s.node_id = w.node_id
    )
    ORDER BY w.node_id
    LIMIT coalesce((SELECT greatest(0, sqlc.arg(max_sources)::integer - ingest_count) FROM owned), 0)
), inserted AS (
    INSERT INTO changeset_sources (changeset_id, seq, node_id)
    SELECT o.id, o.ingest_count + row_number() OVER (ORDER BY c.node_id)::integer, c.node_id
    FROM owned o CROSS JOIN chosen c
    RETURNING changeset_id
), updated AS (
    UPDATE changesets c
    SET ingest_count = c.ingest_count + (SELECT count(*)::integer FROM inserted)
    FROM owned o WHERE c.id = o.id
    RETURNING c.id, c.ingest_count
)
SELECT u.id, u.ingest_count, (SELECT count(*)::integer FROM inserted) AS added
FROM updated u;

-- name: FirstIngestSources :many
SELECT r.id::uuid AS changeset_id, s.seq,
       n.id AS node_id
FROM (SELECT unnest(sqlc.arg(changeset_ids)::uuid[]) AS id) r
CROSS JOIN LATERAL (
    SELECT notebook_id FROM changesets WHERE id = r.id LIMIT 1
) c
CROSS JOIN LATERAL (
    SELECT seq, node_id FROM changeset_sources
    WHERE changeset_id = r.id ORDER BY seq LIMIT (SELECT sqlc.arg(preview_count)::integer)
) s
-- Bind each persistent source to one node before nullable projection.
-- The inner LEFT JOIN keeps the generated UUID nullable for missing nodes.
LEFT JOIN LATERAL (
    SELECT n.id
    FROM (SELECT 1) wanted LEFT JOIN nodes n
        ON n.id = s.node_id AND n.notebook_id = c.notebook_id AND n.deleted_at IS NULL
    LIMIT 1
) n ON true
ORDER BY r.id, s.seq;
