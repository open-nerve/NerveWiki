-- name: Backlinks :many
-- The pages that link to target, but for target itself, whose id is after the id after, at most size of them by id
-- (M6/P5 design 3): each with the revision and the extractor its rows are of, none for no row; how many of its links
-- lead to target, at most max_count; and where the targets of its first contexts of them start and end, by start.
-- One statement, so one snapshot. The pages are found a step each, the next id after the last on
-- page_links_resolved_id_source_id_idx, not by reading all their links: a page may write a million (review r1-1,
-- r2-M1). So is target, though not counted, and left out at the end: a filter on the step would read each of its
-- links to itself (review c1).
WITH RECURSIVE sources (source_id, n) AS (
    (
        SELECT l.source_id, CASE WHEN l.source_id = sqlc.arg(target)::uuid THEN 0 ELSE 1 END
        FROM page_links l
        WHERE l.resolved_id = sqlc.arg(target)::uuid AND l.source_id > sqlc.arg(after)::uuid
        ORDER BY l.source_id
        LIMIT 1
    )
    UNION ALL
    SELECT x.source_id, s.n + CASE WHEN x.source_id = sqlc.arg(target)::uuid THEN 0 ELSE 1 END
    FROM sources s
    CROSS JOIN LATERAL (
        SELECT l.source_id
        FROM page_links l
        WHERE l.resolved_id = sqlc.arg(target)::uuid AND l.source_id > s.source_id
        ORDER BY l.source_id
        LIMIT 1
    ) x
    WHERE s.n < sqlc.arg(size)::integer
)
SELECT s.source_id::uuid AS source_id, coalesce(ip.revision, 0)::integer AS revision,
    coalesce(ip.extractor, 0)::integer AS extractor, c.links, f.range_start, f.range_end
FROM sources s
LEFT JOIN indexed_pages ip ON ip.node_id = s.source_id
CROSS JOIN LATERAL (
    -- In the index's order, which only the index reads cheaply: unordered, a scan of the table stopped early looks
    -- cheap where one page writes most of the links, and is not (review c1).
    SELECT count(*) AS links
    FROM (
        SELECT 1 FROM page_links
        WHERE resolved_id = sqlc.arg(target)::uuid AND source_id = s.source_id
        ORDER BY range_start
        LIMIT sqlc.arg(max_count)::integer
    ) counted
) c
CROSS JOIN LATERAL (
    SELECT range_start, range_end
    FROM page_links
    WHERE resolved_id = sqlc.arg(target)::uuid AND source_id = s.source_id
    ORDER BY range_start
    LIMIT sqlc.arg(contexts)::integer
) f
WHERE s.source_id <> sqlc.arg(target)::uuid
ORDER BY s.source_id, f.range_start;

-- name: PageProperties :one
-- A page's properties for its right panel (M6/P5 design 4): whether its frontmatter is valid; its properties' keys
-- and values, in the order written; and its property links' paths and where each resolves, the zero id for none, by
-- where they start. One statement, so one snapshot; none for a page the index does not have.
SELECT ip.frontmatter_valid,
    ARRAY(
        SELECT p.key FROM page_properties p WHERE p.source_id = ip.node_id ORDER BY p.position
    )::text[] AS keys,
    ARRAY(
        SELECT p.value FROM page_properties p WHERE p.source_id = ip.node_id ORDER BY p.position
    )::jsonb[] AS property_values,
    ARRAY(
        SELECT l.property_key FROM page_links l
        WHERE l.source_id = ip.node_id AND l.property_key IS NOT NULL ORDER BY l.range_start
    )::text[] AS link_keys,
    ARRAY(
        SELECT coalesce(l.resolved_id, '00000000-0000-0000-0000-000000000000'::uuid) FROM page_links l
        WHERE l.source_id = ip.node_id AND l.property_key IS NOT NULL ORDER BY l.range_start
    )::uuid[] AS link_ids
FROM indexed_pages ip
WHERE ip.node_id = sqlc.arg(node_id);

-- name: Tags :many
-- A notebook's tags by key (M6/P5 design 5): each as most of its pages write it, the first by bytes of those as
-- many, and how many pages have it.
SELECT mode() WITHIN GROUP (ORDER BY t.tag COLLATE "C")::text AS tag, count(*) AS pages
FROM page_tags t
WHERE t.notebook_id = sqlc.arg(notebook_id)
GROUP BY t.tag_key
ORDER BY t.tag_key;

-- name: TagPages :many
-- The pages of a notebook with the tag of key or one under it, key/…, by id (M6/P5 design 5): '0' is the byte
-- after '/', and the keys compare by bytes.
SELECT DISTINCT t.source_id
FROM page_tags t
WHERE t.notebook_id = sqlc.arg(notebook_id) AND (
    t.tag_key = sqlc.arg(key)::text COLLATE "C"
    OR t.tag_key >= (sqlc.arg(key)::text || '/') COLLATE "C" AND t.tag_key < (sqlc.arg(key)::text || '0') COLLATE "C"
)
ORDER BY t.source_id;

-- name: NotebookAliases :many
-- The aliases of a notebook's pages, by page, each page's by key (M6/P5 design 6).
SELECT a.source_id, a.alias FROM page_aliases a WHERE a.notebook_id = sqlc.arg(notebook_id) ORDER BY a.source_id, a.alias_key;
