-- name: SourceHeads :many
-- Metadata only. Deleted nodes and content remain readable before a restore.
SELECT n.id, n.kind, n.source_hash AS before_source_hash,
    c.content_hash, COALESCE(c.byte_size, 0)::bigint AS byte_size
FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id
WHERE n.notebook_id = sqlc.arg(notebook_id)
    AND n.id = ANY(sqlc.arg(node_ids)::uuid[]);

-- name: ProbeSources :many
-- Each key's cursor is independent. The adapter bounds both the arrays and
-- sum(takes) to 10000; the inner index scan never sorts the whole hash group.
-- As the existing array writes do, the SELECT's set-returning functions zip
-- equal-length arrays. generate_subscripts supplies their one-based ordinal;
-- sqlc does not resolve PostgreSQL's multi-array FROM unnest overload.
-- Its UUID slice cannot represent NULL elements either: has_after distinguishes
-- an absent cursor (after_id is zero) from a continuation after the zero UUID.
SELECT wanted.ord::bigint AS probe, hit.id
FROM (
    SELECT unnest(sqlc.arg(kinds)::text[]) AS kind, unnest(sqlc.arg(hashes)::bytea[]) AS hash,
        unnest(sqlc.arg(after_ids)::uuid[]) AS after_id, unnest(sqlc.arg(takes)::int[]) AS take,
        unnest(sqlc.arg(has_afters)::boolean[]) AS has_after,
        generate_subscripts(sqlc.arg(kinds)::text[], 1) AS ord
) wanted
CROSS JOIN LATERAL (
    SELECT n.id FROM nodes n
    WHERE n.notebook_id = sqlc.arg(notebook_id)
        AND n.kind = wanted.kind AND n.source_hash = wanted.hash
        AND n.source_hash IS NOT NULL AND n.deleted_at IS NULL
        AND n.id >= wanted.after_id
        AND (NOT wanted.has_after OR n.id <> wanted.after_id)
    ORDER BY n.id
    LIMIT wanted.take
) hit
ORDER BY wanted.ord, hit.id;

-- name: SourceBaselines :many
-- Derived columns only, before a Unit deletion clears them in its UPDATE.
SELECT id, kind, source_hash AS before_source_hash FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND id = ANY(sqlc.arg(node_ids)::uuid[])
    AND deleted_at IS NULL;

-- name: SetSourceHashes :many
-- Sibling CTEs share a statement snapshot: previous sees the old column,
-- while RETURNING identifies only rows actually changed by the write.
WITH previous AS MATERIALIZED (
    SELECT old.id, old.kind, old.source_hash AS before_source_hash FROM nodes old
    WHERE old.notebook_id = sqlc.arg(notebook_id) AND old.id = ANY(sqlc.arg(node_ids)::uuid[])
        AND old.deleted_at IS NULL
), changed AS (
    UPDATE nodes n SET source_hash = v.hash
    FROM (SELECT unnest(sqlc.arg(node_ids)::uuid[]) AS id, unnest(sqlc.arg(hashes)::bytea[]) AS hash) v
    WHERE n.id = v.id AND n.notebook_id = sqlc.arg(notebook_id)
        AND n.deleted_at IS NULL AND n.source_hash IS DISTINCT FROM v.hash
    RETURNING n.id
)
SELECT p.id, p.kind, p.before_source_hash FROM previous p JOIN changed c ON c.id = p.id;

-- name: LockSources :exec
SELECT pg_advisory_xact_lock(sqlc.arg(space)::int, sqlc.arg(key)::int);

-- name: ForgetSources :exec
-- Downgrade holds the notebook lock and uses this same transaction before
-- its mode subscribers. Deleted nodes already have NULL from their deletion.
UPDATE nodes SET source_hash = NULL
WHERE notebook_id = sqlc.arg(notebook_id) AND source_hash IS NOT NULL AND deleted_at IS NULL;
