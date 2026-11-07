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

-- name: IngestNodesNow :many
WITH RECURSIVE batch AS MATERIALIZED (
    SELECT r.id, n.parent_id, n.kind, n.name_key,
           n.id IS NOT NULL AS node_exists,
           n.id IS NOT NULL AND n.deleted_at IS NULL AS live
    FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id) r
    LEFT JOIN LATERAL (
        SELECT id, parent_id, kind, name_key, deleted_at FROM nodes
        WHERE id = r.id AND notebook_id = sqlc.arg(notebook_id)::uuid
        LIMIT 1
    ) n ON true
), walk AS (
    SELECT b.id AS leaf_id, b.id, b.parent_id, 1 AS depth,
           sqlc.arg(max_page_depth)::integer + CASE WHEN b.kind = 'asset' THEN 1 ELSE 0 END AS depth_limit,
           ARRAY[b.id]::uuid[] AS seen,
           b.id = sqlc.arg(source_root_id)::uuid AS saw_source_root,
           false AS invalid
    FROM batch b WHERE b.live
    UNION ALL
    SELECT w.leaf_id, p.id, p.parent_id, w.depth + 1, w.depth_limit,
           array_append(w.seen, p.id),
           w.saw_source_root OR coalesce(p.id = sqlc.arg(source_root_id)::uuid, false),
           (p.id IS NULL OR p.deleted_at IS NOT NULL OR p.kind <> 'page'
            OR p.id = ANY(w.seen) OR w.depth + 1 > w.depth_limit) AS invalid
    FROM walk w
    LEFT JOIN LATERAL (
        SELECT id, parent_id, kind, deleted_at FROM nodes
        WHERE id = w.parent_id AND notebook_id = sqlc.arg(notebook_id)::uuid
        LIMIT 1
    ) p ON true
    WHERE w.parent_id IS NOT NULL AND NOT w.invalid
), ancestry AS MATERIALIZED (
    SELECT leaf_id, bool_or(invalid) AS invalid,
           bool_or(parent_id IS NULL AND NOT invalid) AS reached_root,
           bool_or(saw_source_root) AS in_root_line
    FROM walk GROUP BY leaf_id
)
SELECT b.id::uuid AS id, b.node_exists::boolean AS node_exists, coalesce(b.kind, '')::text AS kind, b.live::boolean AS live,
       (c.node_id IS NOT NULL)::boolean AS has_content, coalesce(c.revision, 0)::integer AS revision,
       coalesce(c.byte_size, 0)::bigint AS page_bytes,
       b.id = sqlc.arg(source_root_id)::uuid AS is_source_root,
       b.live AND coalesce(a.in_root_line, false)
         AND b.id <> sqlc.arg(source_root_id)::uuid AS in_source,
       b.live AND b.kind = 'page' AND b.parent_id IS NULL
         AND b.name_key = sqlc.arg(agents_key)::text AS is_root_agents,
       b.live AND NOT coalesce(a.invalid, true)
         AND coalesce(a.reached_root, false) AS line_ok
FROM batch b
LEFT JOIN ancestry a ON a.leaf_id = b.id
LEFT JOIN LATERAL (
    SELECT node_id, revision, byte_size FROM page_contents
    WHERE node_id = b.id AND deleted_at IS NULL LIMIT 1
) c ON b.live AND b.kind = 'page'
ORDER BY b.id;

-- name: IngestHeadsFirst :many
WITH RECURSIVE batch AS MATERIALIZED (
    SELECT id, parent_id, kind, name_key, deleted_at IS NULL AS live
    FROM nodes
    WHERE notebook_id = sqlc.arg(notebook_id)::uuid
    ORDER BY id LIMIT (SELECT sqlc.arg(take)::integer)
), walk AS (
    SELECT b.id AS leaf_id, b.id, b.parent_id, 1 AS depth,
           sqlc.arg(max_page_depth)::integer + CASE WHEN b.kind = 'asset' THEN 1 ELSE 0 END AS depth_limit,
           ARRAY[b.id]::uuid[] AS seen,
           b.id = sqlc.arg(source_root_id)::uuid AS saw_source_root,
           false AS invalid
    FROM batch b WHERE b.live
    UNION ALL
    SELECT w.leaf_id, p.id, p.parent_id, w.depth + 1, w.depth_limit,
           array_append(w.seen, p.id),
           w.saw_source_root OR coalesce(p.id = sqlc.arg(source_root_id)::uuid, false),
           (p.id IS NULL OR p.deleted_at IS NOT NULL OR p.kind <> 'page'
            OR p.id = ANY(w.seen) OR w.depth + 1 > w.depth_limit) AS invalid
    FROM walk w
    LEFT JOIN LATERAL (
        SELECT id, parent_id, kind, deleted_at FROM nodes
        WHERE id = w.parent_id AND notebook_id = sqlc.arg(notebook_id)::uuid
        LIMIT 1
    ) p ON true
    WHERE w.parent_id IS NOT NULL AND NOT w.invalid
), ancestry AS MATERIALIZED (
    SELECT leaf_id, bool_or(invalid) AS invalid,
           bool_or(parent_id IS NULL AND NOT invalid) AS reached_root,
           bool_or(saw_source_root) AS in_root_line
    FROM walk GROUP BY leaf_id
)
SELECT b.id, b.kind, b.live::boolean AS live,
       (c.node_id IS NOT NULL)::boolean AS has_content, coalesce(c.revision, 0)::integer AS revision,
       coalesce(c.byte_size, 0)::bigint AS page_bytes,
       b.id = sqlc.arg(source_root_id)::uuid AS is_source_root,
       b.live AND coalesce(a.in_root_line, false)
         AND b.id <> sqlc.arg(source_root_id)::uuid AS in_source,
       b.live AND b.kind = 'page' AND b.parent_id IS NULL
         AND b.name_key = sqlc.arg(agents_key)::text AS is_root_agents,
       b.live AND NOT coalesce(a.invalid, true)
         AND coalesce(a.reached_root, false) AS line_ok
FROM batch b
LEFT JOIN ancestry a ON a.leaf_id = b.id
LEFT JOIN LATERAL (
    SELECT node_id, revision, byte_size FROM page_contents
    WHERE node_id = b.id AND deleted_at IS NULL LIMIT 1
) c ON b.live AND b.kind = 'page'
ORDER BY b.id;

-- name: IngestHeadsAfter :many
WITH RECURSIVE batch AS MATERIALIZED (
    SELECT id, parent_id, kind, name_key, deleted_at IS NULL AS live
    FROM nodes
    WHERE notebook_id = sqlc.arg(notebook_id)::uuid AND id > sqlc.arg(after_id)::uuid
    ORDER BY id LIMIT (SELECT sqlc.arg(take)::integer)
), walk AS (
    SELECT b.id AS leaf_id, b.id, b.parent_id, 1 AS depth,
           sqlc.arg(max_page_depth)::integer + CASE WHEN b.kind = 'asset' THEN 1 ELSE 0 END AS depth_limit,
           ARRAY[b.id]::uuid[] AS seen,
           b.id = sqlc.arg(source_root_id)::uuid AS saw_source_root,
           false AS invalid
    FROM batch b WHERE b.live
    UNION ALL
    SELECT w.leaf_id, p.id, p.parent_id, w.depth + 1, w.depth_limit,
           array_append(w.seen, p.id),
           w.saw_source_root OR coalesce(p.id = sqlc.arg(source_root_id)::uuid, false),
           (p.id IS NULL OR p.deleted_at IS NOT NULL OR p.kind <> 'page'
            OR p.id = ANY(w.seen) OR w.depth + 1 > w.depth_limit) AS invalid
    FROM walk w
    LEFT JOIN LATERAL (
        SELECT id, parent_id, kind, deleted_at FROM nodes
        WHERE id = w.parent_id AND notebook_id = sqlc.arg(notebook_id)::uuid
        LIMIT 1
    ) p ON true
    WHERE w.parent_id IS NOT NULL AND NOT w.invalid
), ancestry AS MATERIALIZED (
    SELECT leaf_id, bool_or(invalid) AS invalid,
           bool_or(parent_id IS NULL AND NOT invalid) AS reached_root,
           bool_or(saw_source_root) AS in_root_line
    FROM walk GROUP BY leaf_id
)
SELECT b.id, b.kind, b.live::boolean AS live,
       (c.node_id IS NOT NULL)::boolean AS has_content, coalesce(c.revision, 0)::integer AS revision,
       coalesce(c.byte_size, 0)::bigint AS page_bytes,
       b.id = sqlc.arg(source_root_id)::uuid AS is_source_root,
       b.live AND coalesce(a.in_root_line, false)
         AND b.id <> sqlc.arg(source_root_id)::uuid AS in_source,
       b.live AND b.kind = 'page' AND b.parent_id IS NULL
         AND b.name_key = sqlc.arg(agents_key)::text AS is_root_agents,
       b.live AND NOT coalesce(a.invalid, true)
         AND coalesce(a.reached_root, false) AS line_ok
FROM batch b
LEFT JOIN ancestry a ON a.leaf_id = b.id
LEFT JOIN LATERAL (
    SELECT node_id, revision, byte_size FROM page_contents
    WHERE node_id = b.id AND deleted_at IS NULL LIMIT 1
) c ON b.live AND b.kind = 'page'
ORDER BY b.id;
