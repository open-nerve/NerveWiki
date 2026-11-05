-- name: InsertRebuiltLinks :exec
-- Private Rebuild writes each parsed link once after all aliases and heads exist
-- and every input has been freshly resolved. Empty pages still execute this query.
INSERT INTO page_links (
    source_id, range_start, range_end, notebook_id, kind, property_key, target, anchor, display, target_key,
    target_alt_key, resolved_id, ambiguous, aliases, resolved_asset
)
SELECT sqlc.arg(source_id), u.range_start, u.range_end, sqlc.arg(notebook_id), u.kind, CASE WHEN u.in_frontmatter THEN u.property_key ELSE NULL::text END,
    u.target, NULLIF(u.anchor, ''), NULLIF(u.display, ''), NULLIF(u.target_key, ''), NULLIF(u.target_alt_key, ''),
    NULLIF(u.resolved_id, '00000000-0000-0000-0000-000000000000'::uuid), u.ambiguous, u.aliases, u.resolved_asset
FROM (
    SELECT unnest(sqlc.arg(range_starts)::integer[]) AS range_start, unnest(sqlc.arg(range_ends)::integer[]) AS range_end,
        unnest(sqlc.arg(kinds)::text[]) AS kind, unnest(sqlc.arg(property_keys)::text[]) AS property_key,
        unnest(sqlc.arg(in_frontmatter)::boolean[]) AS in_frontmatter,
        unnest(sqlc.arg(targets)::text[]) AS target, unnest(sqlc.arg(anchors)::text[]) AS anchor,
        unnest(sqlc.arg(displays)::text[]) AS display, unnest(sqlc.arg(target_keys)::text[]) AS target_key,
        unnest(sqlc.arg(target_alt_keys)::text[]) AS target_alt_key, unnest(sqlc.arg(aliases)::boolean[]) AS aliases,
        unnest(sqlc.arg(resolved_ids)::uuid[]) AS resolved_id, unnest(sqlc.arg(ambiguous)::boolean[]) AS ambiguous,
        unnest(sqlc.arg(resolved_assets)::boolean[]) AS resolved_asset
) AS u;
