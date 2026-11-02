-- name: IndexPages :many
-- Read each notebook page's first ASCII-insensitive type key through its partial index.
-- Valid frontmatter and a string nonempty under JS trim qualify it. The 257-character
-- prefix normally decides that; only an all-whitespace long prefix reads the full value.
-- Exact filtering reads the full value only when matching prefixes and lengths need it.
SELECT ip.node_id, coalesce(t.nonempty, false)::boolean AS typed,
       CASE WHEN coalesce(t.nonempty, false) THEN left(t.value_start, 128) ELSE '' END::text AS type_start
FROM indexed_pages ip
LEFT JOIN LATERAL (
    SELECT p.value_start,
      CASE WHEN p.value_start IS NULL THEN false
           WHEN btrim(p.value_start, sqlc.arg(trim_chars)::text) <> '' THEN true
           WHEN p.value_chars <= 257 THEN false
           ELSE btrim(p.value #>> '{}', sqlc.arg(trim_chars)::text) <> '' END AS nonempty,
      CASE WHEN sqlc.narg(type_is)::text IS NULL THEN true
           WHEN p.value_start IS DISTINCT FROM left(sqlc.narg(type_is)::text,257) THEN false
           WHEN char_length(sqlc.narg(type_is)::text) < 257 THEN true
           WHEN p.value_chars <> char_length(sqlc.narg(type_is)::text) THEN false
           ELSE p.value = to_jsonb(sqlc.narg(type_is)::text) END AS of_type
    FROM page_properties p
    WHERE ip.frontmatter_valid AND p.source_id = ip.node_id
      AND translate(p.key,'TYPE','type') = 'type'
    ORDER BY p.position LIMIT 1
) t ON true
WHERE ip.notebook_id = sqlc.arg(notebook_id)::uuid
  AND (sqlc.narg(type_is)::text IS NULL OR (t.nonempty AND t.of_type));

-- name: IndexItems :many
-- Point-read these IDs' first type and summary keys, then the first 21 tags.
-- Type uses IndexPages' qualification; summary keeps its existing string-prefix contract.
SELECT ip.node_id, coalesce(ty.nonempty,false)::boolean AS typed,
       CASE WHEN coalesce(ty.nonempty,false) THEN coalesce(ty.value_start,'') ELSE '' END::text AS type_text,
       (s.value_start IS NOT NULL)::boolean AS summarized,
       coalesce(s.value_start,'')::text AS summary_start,
       ARRAY(SELECT left(t.tag,512) FROM page_tags t WHERE t.source_id=ip.node_id ORDER BY t.tag_key LIMIT 21)::text[] AS tags
FROM indexed_pages ip
LEFT JOIN LATERAL (
    SELECT p.value_start,
      CASE WHEN p.value_start IS NULL THEN false
           WHEN btrim(p.value_start,sqlc.arg(trim_chars)::text)<>'' THEN true
           WHEN p.value_chars<=257 THEN false
           ELSE btrim(p.value #>> '{}',sqlc.arg(trim_chars)::text)<>'' END AS nonempty
    FROM page_properties p
    WHERE ip.frontmatter_valid AND p.source_id=ip.node_id AND translate(p.key,'TYPE','type')='type'
    ORDER BY p.position LIMIT 1
) ty ON true
LEFT JOIN LATERAL (
    SELECT p.value_start FROM page_properties p
    WHERE p.source_id=ip.node_id AND translate(p.key,'SUMARY','sumary')='summary'
    ORDER BY p.position LIMIT 1
) s ON true
WHERE ip.node_id=ANY(sqlc.arg(ids)::uuid[]);

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

-- Complete stored filter values for one finite batch, at most 512 requested IDs.
-- Every requested ID gets a row; a vanished index row is explicitly untyped.
-- Empty text transports NULL through sqlc's nonnullable CASE inference. A qualified
-- type cannot be empty; the adapter restores nil for this private sentinel.
-- name: IndexCatalogueHeads :many
WITH requested AS MATERIALIZED (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id)
SELECT r.id::uuid AS id, coalesce(t.nonempty,false)::boolean AS typed,
       coalesce(CASE WHEN t.nonempty AND octet_length(t.filter_value)<=1024
            THEN t.filter_value ELSE NULL END, '')::text AS filter_type
FROM requested r
LEFT JOIN LATERAL (
    SELECT frontmatter_valid FROM indexed_pages
    WHERE node_id=r.id AND notebook_id=sqlc.arg(notebook_id)::uuid LIMIT 1
) ip ON true
LEFT JOIN LATERAL (
    SELECT
      CASE WHEN p.value_start IS NULL THEN false
           WHEN btrim(p.value_start,sqlc.arg(trim_chars)::text)<>'' THEN true
           WHEN p.value_chars<=257 THEN false
           ELSE btrim(p.value #>> '{}',sqlc.arg(trim_chars)::text)<>'' END AS nonempty,
      CASE WHEN p.value_start IS NULL THEN NULL
           WHEN p.value_chars<=257 THEN p.value_start
           WHEN p.value_chars<=1024 THEN p.value #>> '{}'
           ELSE NULL END AS filter_value
    FROM page_properties p
    WHERE ip.frontmatter_valid AND p.source_id=r.id AND translate(p.key,'TYPE','type')='type'
    ORDER BY p.position LIMIT 1
) t ON true
ORDER BY r.id;
