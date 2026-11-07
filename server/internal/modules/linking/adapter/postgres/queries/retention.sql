-- name: InspectPageLinksForRetention :many
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
    COALESCE(l.aliases, false)::boolean AS aliases, l.resolved_id
FROM eligibility e
LEFT JOIN page_links l ON l.source_id = sqlc.arg(source_id)::uuid
ORDER BY l.range_start;

-- name: DeleteMissingPageLinks :exec
-- These are the actual old keys missing from the complete incoming facts.
-- Still called for an empty set, at the original delete stage.
DELETE FROM page_links
WHERE source_id = sqlc.arg(source_id)::uuid
  AND range_start = ANY(sqlc.arg(missing_range_starts)::integer[])
  AND COALESCE(cardinality(sqlc.arg(missing_range_starts)::integer[]), 0) > 0;

-- name: WritePageLinksRetainingEqualPrefiltered :exec
WITH incoming AS MATERIALIZED (
    SELECT u.range_start, u.range_end, sqlc.arg(notebook_id)::uuid AS notebook_id,
        u.kind, CASE WHEN u.in_frontmatter THEN u.property_key ELSE NULL::text END AS property_key,
        u.target, NULLIF(u.anchor, '') AS anchor, NULLIF(u.display, '') AS display,
        NULLIF(u.target_key, '') AS target_key, NULLIF(u.target_alt_key, '') AS target_alt_key,
        u.aliases
    FROM (
        SELECT unnest(sqlc.arg(range_starts)::integer[]) AS range_start,
            unnest(sqlc.arg(range_ends)::integer[]) AS range_end,
            unnest(sqlc.arg(kinds)::text[]) AS kind,
            unnest(sqlc.arg(property_keys)::text[]) AS property_key,
            unnest(sqlc.arg(in_frontmatter)::boolean[]) AS in_frontmatter,
            unnest(sqlc.arg(targets)::text[]) AS target,
            unnest(sqlc.arg(anchors)::text[]) AS anchor,
            unnest(sqlc.arg(displays)::text[]) AS display,
            unnest(sqlc.arg(target_keys)::text[]) AS target_key,
            unnest(sqlc.arg(target_alt_keys)::text[]) AS target_alt_key,
            unnest(sqlc.arg(aliases)::boolean[]) AS aliases
    ) AS u
)
INSERT INTO page_links AS l (
    source_id, range_start, range_end, notebook_id, kind, property_key, target,
    anchor, display, target_key, target_alt_key, resolved_id, ambiguous, aliases,
    resolved_asset
)
SELECT sqlc.arg(source_id)::uuid, n.range_start, n.range_end, n.notebook_id,
    n.kind, n.property_key, n.target, n.anchor, n.display, n.target_key,
    n.target_alt_key, NULL, false, n.aliases, false
FROM incoming n
ON CONFLICT (source_id, range_start) DO UPDATE
SET range_end = EXCLUDED.range_end,
    notebook_id = EXCLUDED.notebook_id,
    kind = EXCLUDED.kind,
    property_key = EXCLUDED.property_key,
    target = EXCLUDED.target,
    anchor = EXCLUDED.anchor,
    display = EXCLUDED.display,
    target_key = EXCLUDED.target_key,
    target_alt_key = EXCLUDED.target_alt_key,
    aliases = EXCLUDED.aliases,
    resolved_id = NULL,
    ambiguous = false,
    resolved_asset = false
WHERE ROW(
    l.range_end, l.notebook_id, l.kind COLLATE "C", l.property_key COLLATE "C",
    l.target COLLATE "C", l.anchor COLLATE "C", l.display COLLATE "C",
    l.target_key COLLATE "C", l.target_alt_key COLLATE "C", l.aliases
) IS DISTINCT FROM ROW(
    EXCLUDED.range_end, EXCLUDED.notebook_id, EXCLUDED.kind COLLATE "C",
    EXCLUDED.property_key COLLATE "C", EXCLUDED.target COLLATE "C",
    EXCLUDED.anchor COLLATE "C", EXCLUDED.display COLLATE "C",
    EXCLUDED.target_key COLLATE "C", EXCLUDED.target_alt_key COLLATE "C",
    EXCLUDED.aliases
);
