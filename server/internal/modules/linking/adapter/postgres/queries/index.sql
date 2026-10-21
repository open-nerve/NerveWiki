-- name: IndexPages :many
-- The pages of a notebook the index holds (M9/P4 design 3.9.3), each with the start of its type: the first of its
-- properties whose key is "type" but for ASCII case (translate leaves any other key unequal), when that property's
-- value is a string, its first 128 characters, which the order's key of 64 bytes needs. With type_is, only the pages
-- whose type is that string; typed tells an empty type from none. Each page's properties are read by
-- page_properties_type_idx (00047), one probe a page whatever properties it has: never the notebook's others, nor
-- another notebook's. The pages are read whole: the order is Go's title keys.
SELECT ip.node_id, (t.type_start IS NOT NULL)::boolean AS typed, coalesce(t.type_start, '')::text AS type_start
FROM indexed_pages ip
LEFT JOIN LATERAL (
    SELECT (CASE WHEN jsonb_typeof(p.value) = 'string' THEN left(p.value #>> '{}', 128) END)::text AS type_start, p.value
    FROM page_properties p
    WHERE p.source_id = ip.node_id AND translate(p.key, 'TYPE', 'type') = 'type'
    ORDER BY p.position
    LIMIT 1
) t ON true
WHERE ip.notebook_id = sqlc.arg(notebook_id)
    AND (sqlc.narg(type_is)::text IS NULL OR t.value = to_jsonb(sqlc.narg(type_is)::text));

-- name: IndexItems :many
-- The types, summaries and tags of the index's pages ids (M9/P4 design 3.9.3): the first property whose key is "type",
-- and the first whose key is "summary", but for ASCII case, each's first 512 characters when its value is a string, by
-- their partial indexes (00047), typed and summarized telling an empty one from none; and the first 21 tags by key,
-- each as the page first writes it, its first 512 characters, the 21st telling that more follow, by the primary key.
SELECT ip.node_id,
    (ty.type_text IS NOT NULL)::boolean AS typed,
    coalesce(ty.type_text, '')::text AS type_text,
    (s.summary_start IS NOT NULL)::boolean AS summarized,
    coalesce(s.summary_start, '')::text AS summary_start,
    ARRAY(
        SELECT left(t.tag, 512) FROM page_tags t WHERE t.source_id = ip.node_id ORDER BY t.tag_key LIMIT 21
    )::text[] AS tags
FROM indexed_pages ip
LEFT JOIN LATERAL (
    SELECT (CASE WHEN jsonb_typeof(p.value) = 'string' THEN left(p.value #>> '{}', 512) END)::text AS type_text
    FROM page_properties p
    WHERE p.source_id = ip.node_id AND translate(p.key, 'TYPE', 'type') = 'type'
    ORDER BY p.position
    LIMIT 1
) ty ON true
LEFT JOIN LATERAL (
    SELECT (CASE WHEN jsonb_typeof(p.value) = 'string' THEN left(p.value #>> '{}', 512) END)::text AS summary_start
    FROM page_properties p
    WHERE p.source_id = ip.node_id AND translate(p.key, 'SUMARY', 'sumary') = 'summary'
    ORDER BY p.position
    LIMIT 1
) s ON true
WHERE ip.node_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: InboundCounts :many
-- How many pages but itself link to each of the pages ids, at most max_count (M9/P4 design 3.9.3): the sources found
-- a step each, the next after the last on page_links_resolved_id_source_id_idx, as Backlinks finds them, never all of
-- a page's links: a page may write a million (M6/P5 review r1-1). A page no other links to has no row, or a row of 0
-- when it links to itself alone.
WITH RECURSIVE steps (target, source_id, n) AS (
    SELECT t.id, x.source_id, CASE WHEN x.source_id = t.id THEN 0 ELSE 1 END
    FROM unnest(sqlc.arg(ids)::uuid[]) t (id)
    CROSS JOIN LATERAL (
        SELECT l.source_id FROM page_links l
        WHERE l.resolved_id = t.id
        ORDER BY l.source_id
        LIMIT 1
    ) x
    UNION ALL
    SELECT s.target, x.source_id, s.n + CASE WHEN x.source_id = s.target THEN 0 ELSE 1 END
    FROM steps s
    CROSS JOIN LATERAL (
        SELECT l.source_id FROM page_links l
        WHERE l.resolved_id = s.target AND l.source_id > s.source_id
        ORDER BY l.source_id
        LIMIT 1
    ) x
    WHERE s.n < sqlc.arg(max_count)::integer
)
SELECT target::uuid AS page_id, max(n)::integer AS inbound FROM steps GROUP BY target;
