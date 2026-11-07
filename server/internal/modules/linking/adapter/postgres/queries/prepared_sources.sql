-- name: InspectPageLinksForPreparation :many
-- One complete source read. The materialized predicate runs once, including
-- for an empty source. Its NULL range_start is metadata, never a stored key.
-- The caller holds the same notebook/index maintenance locks until commit;
-- this query does not acquire them or validate Before.
-- Keep source selection separate from the notebook/eligibility predicates.
-- These four small producers preserve the original source-bounded lookups.
WITH old_head AS MATERIALIZED (
    SELECT node_id, notebook_id, extractor FROM indexed_pages
    WHERE node_id = sqlc.arg(source_id)::uuid
), old_tags AS MATERIALIZED (
    SELECT notebook_id FROM page_tags
    WHERE source_id = sqlc.arg(source_id)::uuid
), old_properties AS MATERIALIZED (
    SELECT notebook_id FROM page_properties
    WHERE source_id = sqlc.arg(source_id)::uuid
), old_aliases AS MATERIALIZED (
    SELECT notebook_id FROM page_aliases
    WHERE source_id = sqlc.arg(source_id)::uuid
), eligibility AS MATERIALIZED (
    SELECT EXISTS (
        SELECT 1 FROM old_head i
        WHERE i.node_id = sqlc.arg(source_id)::uuid
          AND i.notebook_id = sqlc.arg(notebook_id)::uuid
          AND i.extractor = sqlc.arg(extractor)::integer
    ) AND NOT EXISTS (
        SELECT 1 FROM old_tags t
        WHERE t.notebook_id <> sqlc.arg(notebook_id)::uuid
    ) AND NOT EXISTS (
        SELECT 1 FROM old_properties p
        WHERE p.notebook_id <> sqlc.arg(notebook_id)::uuid
    ) AND NOT EXISTS (
        SELECT 1 FROM old_aliases a
        WHERE a.notebook_id <> sqlc.arg(notebook_id)::uuid
    ) AS can_retain
)
SELECT COALESCE(e.can_retain, false)::boolean AS can_retain, l.range_start,
    COALESCE(l.range_end, 0)::integer AS range_end,
    COALESCE(l.notebook_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS notebook_id,
    COALESCE(l.kind, '')::text AS kind, l.property_key,
    COALESCE(l.target, '')::text AS target,
    l.anchor, l.display, l.target_key, l.target_alt_key,
    CASE WHEN l.range_start IS NULL THEN false ELSE l.aliases END::boolean AS aliases, l.resolved_id,
    CASE WHEN l.range_start IS NULL THEN false ELSE l.ambiguous END::boolean AS ambiguous,
    CASE WHEN l.range_start IS NULL THEN false ELSE l.resolved_asset END::boolean AS resolved_asset
FROM eligibility e
LEFT JOIN page_links l ON l.source_id = sqlc.arg(source_id)::uuid
ORDER BY l.range_start;

-- name: LinksReachedExcept :many
-- Complete residual relation; all rows of supplied sources come from the
-- same unit's successful Apply, even if their keys/targets also reach here.
-- Probe distinct C-collated keys separately, without inheriting UNION's
-- ordering. Keep the resolved-id range separate from notebook qualification
-- so it cannot intersect a notebook-only bitmap. Exclude supplied sources
-- inside both boundaries and again outside the complete relation.
-- UNION preserves one row when several arms reach it.
WITH reached AS (
    SELECT l.source_id, l.range_start, l.target, l.resolved_id, l.ambiguous, l.resolved_asset, l.aliases
    FROM (
        SELECT DISTINCT k.value COLLATE "C" AS value
        FROM unnest(sqlc.arg(keys)::text[]) AS k(value)
        WHERE k.value IS NOT NULL
    ) AS key_input
    CROSS JOIN LATERAL (
        SELECT l.source_id, l.range_start, l.target, l.resolved_id, l.ambiguous, l.resolved_asset, l.aliases
        FROM page_links l
        WHERE l.notebook_id = sqlc.arg(notebook_id) AND l.target_key = key_input.value
          AND NOT (l.source_id = ANY(COALESCE(sqlc.arg(excluded_sources)::uuid[], '{}'::uuid[])))
        OFFSET 0
    ) l
    UNION
    SELECT l.source_id, l.range_start, l.target, l.resolved_id, l.ambiguous, l.resolved_asset, l.aliases
    FROM page_links l
    WHERE l.notebook_id = sqlc.arg(notebook_id) AND l.target_alt_key = ANY(sqlc.arg(keys)::text[])
    UNION
    SELECT l.source_id, l.range_start, l.target, l.resolved_id, l.ambiguous, l.resolved_asset, l.aliases
    FROM (
        SELECT l.notebook_id, l.source_id, l.range_start, l.target, l.resolved_id, l.ambiguous, l.resolved_asset, l.aliases
        FROM page_links l
        WHERE l.resolved_id = ANY(sqlc.arg(targets)::uuid[])
          AND NOT (l.source_id = ANY(COALESCE(sqlc.arg(excluded_sources)::uuid[], '{}'::uuid[])))
        OFFSET 0
    ) l
    WHERE l.notebook_id = sqlc.arg(notebook_id)
    UNION
    SELECT l.source_id, l.range_start, l.target, l.resolved_id, l.ambiguous, l.resolved_asset, l.aliases
    FROM page_links l
    WHERE l.notebook_id = sqlc.arg(notebook_id) AND l.source_id = ANY(sqlc.arg(sources)::uuid[])
)
SELECT l.source_id, l.range_start, l.target, l.resolved_id, l.ambiguous, l.resolved_asset, l.aliases
FROM reached l
WHERE NOT (l.source_id = ANY(COALESCE(sqlc.arg(excluded_sources)::uuid[], '{}'::uuid[])))
ORDER BY l.source_id,l.range_start;
