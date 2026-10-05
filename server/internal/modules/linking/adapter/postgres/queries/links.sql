-- name: InsertLinks :exec
-- A page's links, resolved to none. An empty property path, anchor, display text or key is none.
INSERT INTO page_links (
    source_id, range_start, range_end, notebook_id, kind, property_key, target, anchor, display, target_key,
    target_alt_key, resolved_id, ambiguous
)
SELECT sqlc.arg(source_id), u.range_start, u.range_end, sqlc.arg(notebook_id), u.kind, NULLIF(u.property_key, ''),
    u.target, NULLIF(u.anchor, ''), NULLIF(u.display, ''), NULLIF(u.target_key, ''), NULLIF(u.target_alt_key, ''),
    NULL, false
FROM (
    SELECT unnest(sqlc.arg(range_starts)::integer[]) AS range_start, unnest(sqlc.arg(range_ends)::integer[]) AS range_end,
        unnest(sqlc.arg(kinds)::text[]) AS kind, unnest(sqlc.arg(property_keys)::text[]) AS property_key,
        unnest(sqlc.arg(targets)::text[]) AS target, unnest(sqlc.arg(anchors)::text[]) AS anchor,
        unnest(sqlc.arg(displays)::text[]) AS display, unnest(sqlc.arg(target_keys)::text[]) AS target_key,
        unnest(sqlc.arg(target_alt_keys)::text[]) AS target_alt_key
) AS u;

-- name: DeleteLinksOf :many
-- The pages the links deleted resolved to, each once.
WITH deleted AS (
    DELETE FROM page_links WHERE source_id = ANY(sqlc.arg(ids)::uuid[]) RETURNING resolved_id
)
SELECT DISTINCT resolved_id::uuid AS resolved_id FROM deleted WHERE resolved_id IS NOT NULL ORDER BY resolved_id;

-- name: DeleteNotebooksLinks :exec
DELETE FROM page_links WHERE notebook_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: LinksReached :many
-- The links of a notebook whose target's keys meet keys, that resolve to one of targets, or that are written in
-- one of sources (M6/P3 design 3.4, step 6). Each tells whether it is a value of its page's aliases, as the
-- extraction tells it (linking/adapter/markdown, keyOf and valueOf): its property is the first key that is
-- "aliases" but for ASCII case, or one of that key's list (M6/P4 design 2).
SELECT l.source_id, l.range_start, l.target, l.resolved_id, l.ambiguous,
    coalesce(l.property_key = a.key OR (left(l.property_key, length(a.key) + 1) = a.key || '.'
        AND substr(l.property_key, length(a.key) + 2) ~ '^[0-9]+$'), false)::boolean AS aliases
FROM page_links l
LEFT JOIN LATERAL (
    SELECT p.key FROM page_properties p
    WHERE p.source_id = l.source_id AND translate(p.key, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz') = 'aliases'
    ORDER BY p.position LIMIT 1
) a ON l.property_key IS NOT NULL
WHERE l.notebook_id = sqlc.arg(notebook_id) AND (
    l.target_key = ANY(sqlc.arg(keys)::text[]) OR l.target_alt_key = ANY(sqlc.arg(keys)::text[])
    OR l.resolved_id = ANY(sqlc.arg(targets)::uuid[]) OR l.source_id = ANY(sqlc.arg(sources)::uuid[])
)
ORDER BY l.source_id, l.range_start;

-- name: SetResolutions :execrows
-- Each link, by its page and start, resolves to the page given, the zero id none.
UPDATE page_links l
SET resolved_id = NULLIF(u.resolved_id, '00000000-0000-0000-0000-000000000000'::uuid), ambiguous = u.ambiguous
FROM (
    SELECT unnest(sqlc.arg(source_ids)::uuid[]) AS source_id, unnest(sqlc.arg(range_starts)::integer[]) AS range_start,
        unnest(sqlc.arg(resolved_ids)::uuid[]) AS resolved_id, unnest(sqlc.arg(ambiguous)::boolean[]) AS ambiguous
) AS u
WHERE l.source_id = u.source_id AND l.range_start = u.range_start;

-- name: PageView :many
-- A page's index for its reading view (M6/P3 design 6.5): the revision and the extractor its rows are of, with where
-- each of its links resolves to; one row without a link for a page without links, none for a page the index does not
-- have. Both tables are read by their primary keys.
SELECT ip.revision, ip.extractor, l.range_start, l.resolved_id, l.ambiguous
FROM indexed_pages ip LEFT JOIN page_links l ON l.source_id = ip.node_id
WHERE ip.node_id = sqlc.arg(node_id);
