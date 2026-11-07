-- name: SetOrderedSourceResolutions :execrows
-- Private strict ordered runs contain one source and 257..4096 distinct starts.
-- Keep the pair equality authoritative and bound the requested starts.
-- A single-source point range avoids adding the parameter to the source equality class.
UPDATE page_links l
SET resolved_id = NULLIF(u.resolved_id, '00000000-0000-0000-0000-000000000000'::uuid), ambiguous = u.ambiguous,
    resolved_asset = u.resolved_asset
FROM (
    SELECT (sqlc.arg(source_ids)::uuid[])[g.i] AS source_id,
        (sqlc.arg(range_starts)::integer[])[g.i] AS range_start,
        (sqlc.arg(resolved_ids)::uuid[])[g.i] AS resolved_id,
        (sqlc.arg(ambiguous)::boolean[])[g.i] AS ambiguous,
        (sqlc.arg(resolved_assets)::boolean[])[g.i] AS resolved_asset
    FROM generate_subscripts(sqlc.arg(source_ids)::uuid[], 1) AS g(i)
) AS u
WHERE l.source_id = u.source_id AND l.range_start = u.range_start
    AND l.source_id BETWEEN (sqlc.arg(source_ids)::uuid[])[1]
                        AND (sqlc.arg(source_ids)::uuid[])[1]
    AND l.range_start = ANY(sqlc.arg(range_starts)::integer[]);
