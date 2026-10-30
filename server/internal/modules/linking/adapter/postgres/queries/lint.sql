-- name: LintHeads :many
SELECT node_id, extractor, frontmatter_valid
FROM indexed_pages WHERE notebook_id = sqlc.arg(notebook_id) ORDER BY node_id;

-- The keyset queries keep current-page/edge eligibility and full-set membership
-- inside LIMIT 1 LATERALs so the outer notebook/tuple range can drive the scan.
-- The scalar LIMIT InitPlan avoids a no-statistics custom plan sorting the
-- entire range because it estimates fewer rows than the supplied take. Both
-- shapes are guarded with normal planner settings and actual generic execution.
-- name: LintWantedEdgeHeadsFirst :many
WITH excluded AS MATERIALIZED (SELECT unnest(sqlc.arg(excluded_ids)::uuid[]) AS id)
SELECT l.source_id, l.range_start, octet_length(l.target)::bigint AS target_bytes
FROM page_links l
CROSS JOIN LATERAL (
 SELECT ip.node_id FROM indexed_pages ip
 WHERE ip.node_id=l.source_id AND ip.notebook_id=sqlc.arg(notebook_id)
   AND ip.extractor=sqlc.arg(extractor)
   AND l.source_id NOT IN (SELECT id FROM excluded)
   AND l.resolved_id IS NULL AND l.target_key IS NOT NULL
 LIMIT 1
) current_page
WHERE l.notebook_id=sqlc.arg(notebook_id)
ORDER BY l.source_id,l.range_start
LIMIT (SELECT sqlc.arg(take)::integer);

-- name: LintWantedEdgeHeadsAfter :many
WITH excluded AS MATERIALIZED (SELECT unnest(sqlc.arg(excluded_ids)::uuid[]) AS id)
SELECT l.source_id, l.range_start, octet_length(l.target)::bigint AS target_bytes
FROM page_links l
CROSS JOIN LATERAL (
 SELECT ip.node_id FROM indexed_pages ip
 WHERE ip.node_id=l.source_id AND ip.notebook_id=sqlc.arg(notebook_id)
   AND ip.extractor=sqlc.arg(extractor)
   AND l.source_id NOT IN (SELECT id FROM excluded)
   AND l.resolved_id IS NULL AND l.target_key IS NOT NULL
 LIMIT 1
) current_page
WHERE l.notebook_id=sqlc.arg(notebook_id)
  AND (l.source_id,l.range_start) >
      (sqlc.arg(after_source)::uuid,sqlc.arg(after_start)::integer)
ORDER BY l.source_id,l.range_start
LIMIT (SELECT sqlc.arg(take)::integer);

-- name: LintEdgesByPositions :many
WITH requested AS MATERIALIZED (
 SELECT unnest(sqlc.arg(source_ids)::uuid[]) AS source_id,
        unnest(sqlc.arg(starts)::integer[]) AS range_start
)
SELECT l.source_id,l.range_start,l.kind,l.target,l.target_key,l.target_alt_key,
       l.resolved_id,l.resolved_asset,l.ambiguous
FROM requested r CROSS JOIN LATERAL (
 SELECT l.source_id,l.range_start,l.kind,l.target,l.target_key,l.target_alt_key,
        l.resolved_id,l.resolved_asset,l.ambiguous,l.notebook_id
 FROM page_links l
 WHERE l.source_id=r.source_id AND l.range_start=r.range_start
 LIMIT 1
) l
WHERE l.notebook_id=sqlc.arg(notebook_id)
ORDER BY l.source_id,l.range_start;

-- name: LintFirstSourceHeads :many
WITH requested AS MATERIALIZED (SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS id)
SELECT ip.node_id,p.position,octet_length(p.value::text)::bigint AS value_bytes
FROM requested r JOIN indexed_pages ip ON ip.node_id=r.id
CROSS JOIN LATERAL (
 SELECT p.position,p.value FROM page_properties p
 WHERE p.source_id=ip.node_id AND translate(p.key,'SOURCE','source')='sources'
 ORDER BY p.position LIMIT 1
) p
WHERE ip.notebook_id=sqlc.arg(notebook_id) AND ip.extractor=sqlc.arg(extractor)
 AND ip.frontmatter_valid ORDER BY ip.node_id;

-- name: LintFirstSourceValues :many
WITH requested AS MATERIALIZED (
 SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS id,
        unnest(sqlc.arg(positions)::integer[]) AS position
)
SELECT r.id::uuid AS node_id,p.position,p.key,p.value::text AS value
FROM requested r JOIN indexed_pages ip ON ip.node_id=r.id
CROSS JOIN LATERAL (
 SELECT p.position,p.key,p.value FROM page_properties p
 WHERE p.source_id=r.id AND p.position=r.position LIMIT 1
) p
WHERE ip.notebook_id=sqlc.arg(notebook_id) AND ip.extractor=sqlc.arg(extractor)
 AND ip.frontmatter_valid AND translate(p.key,'SOURCE','source')='sources'
ORDER BY r.id;

-- name: LintSourceSlotHeadsFirst :many
WITH spec AS MATERIALIZED (
 SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS id,
        unnest(sqlc.arg(first_keys)::text[]) AS key,
        unnest(sqlc.arg(is_array)::boolean[]) AS is_array,
        unnest(sqlc.arg(item_counts)::integer[]) AS items
)
SELECT l.source_id,l.range_start,l.property_key,
       octet_length(l.target)::bigint AS target_bytes
FROM page_links l
CROSS JOIN LATERAL (
 SELECT ip.node_id FROM indexed_pages ip JOIN spec s ON s.id=ip.node_id
 WHERE ip.node_id=l.source_id AND ip.notebook_id=sqlc.arg(notebook_id)
   AND ip.extractor=sqlc.arg(extractor)
 AND CASE WHEN NOT s.is_array THEN l.property_key=s.key
     WHEN octet_length(l.property_key) BETWEEN 9 AND 12
       AND left(l.property_key,8)=s.key||'.'
       AND substring(l.property_key FROM 9) ~ '^(0|[1-9][0-9]{0,3})$'
       THEN substring(l.property_key FROM 9)::integer<s.items
     ELSE false END
 LIMIT 1
) current_slot
WHERE l.notebook_id=sqlc.arg(notebook_id)
 AND l.source_id>=sqlc.arg(batch_first_source)::uuid
 AND l.source_id<=sqlc.arg(batch_last_source)::uuid

ORDER BY l.source_id,l.range_start LIMIT (SELECT sqlc.arg(take)::integer);

-- name: LintSourceSlotHeadsAfter :many
WITH spec AS MATERIALIZED (
 SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS id,
        unnest(sqlc.arg(first_keys)::text[]) AS key,
        unnest(sqlc.arg(is_array)::boolean[]) AS is_array,
        unnest(sqlc.arg(item_counts)::integer[]) AS items
)
SELECT l.source_id,l.range_start,l.property_key,
       octet_length(l.target)::bigint AS target_bytes
FROM page_links l
CROSS JOIN LATERAL (
 SELECT ip.node_id FROM indexed_pages ip JOIN spec s ON s.id=ip.node_id
 WHERE ip.node_id=l.source_id AND ip.notebook_id=sqlc.arg(notebook_id)
   AND ip.extractor=sqlc.arg(extractor)
 AND CASE WHEN NOT s.is_array THEN l.property_key=s.key
     WHEN octet_length(l.property_key) BETWEEN 9 AND 12
       AND left(l.property_key,8)=s.key||'.'
       AND substring(l.property_key FROM 9) ~ '^(0|[1-9][0-9]{0,3})$'
       THEN substring(l.property_key FROM 9)::integer<s.items
     ELSE false END
 LIMIT 1
) current_slot
WHERE l.notebook_id=sqlc.arg(notebook_id)
 AND l.source_id>=sqlc.arg(batch_first_source)::uuid
 AND l.source_id<=sqlc.arg(batch_last_source)::uuid
 AND (l.source_id,l.range_start)>(sqlc.arg(after_source)::uuid,sqlc.arg(after_start)::integer)

ORDER BY l.source_id,l.range_start LIMIT (SELECT sqlc.arg(take)::integer);

-- name: LintSourceSlotEdges :many
WITH requested AS MATERIALIZED (
 SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS source_id,
        unnest(sqlc.arg(starts)::integer[]) AS range_start
)
SELECT l.source_id,l.range_start,l.property_key,l.target,l.kind,
       l.resolved_id,l.resolved_asset,l.ambiguous
FROM requested r CROSS JOIN LATERAL (
 SELECT l.source_id,l.range_start,l.property_key,l.target,l.kind,
        l.resolved_id,l.resolved_asset,l.ambiguous,l.notebook_id
 FROM page_links l WHERE l.source_id=r.source_id AND l.range_start=r.range_start LIMIT 1
) l WHERE l.notebook_id=sqlc.arg(notebook_id)
ORDER BY l.source_id,l.range_start;

-- name: LintBodyReferencesFirst :many
WITH emitters AS MATERIALIZED (SELECT unnest(sqlc.arg(wiki_page_ids)::uuid[]) AS id)
SELECT l.source_id,l.range_start,l.resolved_id,l.resolved_asset
FROM page_links l
CROSS JOIN LATERAL (
 SELECT ip.node_id FROM indexed_pages ip
 WHERE ip.node_id=l.source_id AND ip.notebook_id=sqlc.arg(notebook_id)
   AND ip.extractor=sqlc.arg(extractor)
   AND l.source_id IN (SELECT id FROM emitters)
   AND l.property_key IS NULL AND l.resolved_id IS NOT NULL
 LIMIT 1
) current_page
WHERE l.notebook_id=sqlc.arg(notebook_id)
ORDER BY l.source_id,l.range_start LIMIT (SELECT sqlc.arg(take)::integer);

-- name: LintBodyReferencesAfter :many
WITH emitters AS MATERIALIZED (SELECT unnest(sqlc.arg(wiki_page_ids)::uuid[]) AS id)
SELECT l.source_id,l.range_start,l.resolved_id,l.resolved_asset
FROM page_links l
CROSS JOIN LATERAL (
 SELECT ip.node_id FROM indexed_pages ip
 WHERE ip.node_id=l.source_id AND ip.notebook_id=sqlc.arg(notebook_id)
   AND ip.extractor=sqlc.arg(extractor)
   AND l.source_id IN (SELECT id FROM emitters)
   AND l.property_key IS NULL AND l.resolved_id IS NOT NULL
 LIMIT 1
) current_page
WHERE l.notebook_id=sqlc.arg(notebook_id)
 AND (l.source_id,l.range_start)>(sqlc.arg(after_source)::uuid,sqlc.arg(after_start)::integer)
ORDER BY l.source_id,l.range_start LIMIT (SELECT sqlc.arg(take)::integer);

-- name: LintAmbiguousEdgeHeadsFirst :many
WITH excluded AS MATERIALIZED (SELECT unnest(sqlc.arg(excluded_ids)::uuid[]) AS id)
SELECT l.source_id, l.range_start, octet_length(l.target)::bigint AS target_bytes
FROM page_links l
CROSS JOIN LATERAL (
 SELECT ip.node_id FROM indexed_pages ip
 WHERE ip.node_id=l.source_id AND ip.notebook_id=sqlc.arg(notebook_id)
   AND ip.extractor=sqlc.arg(extractor)
   AND l.source_id NOT IN (SELECT id FROM excluded)
  AND l.ambiguous
  AND (l.resolved_asset OR EXISTS (
    SELECT 1 FROM indexed_pages target_page
    WHERE target_page.node_id=l.resolved_id
      AND target_page.notebook_id=sqlc.arg(notebook_id)
      AND target_page.extractor=sqlc.arg(extractor)
  ))
 LIMIT 1
) current_page
WHERE l.notebook_id=sqlc.arg(notebook_id)

ORDER BY l.source_id,l.range_start
LIMIT (SELECT sqlc.arg(take)::integer);

-- name: LintAmbiguousEdgeHeadsAfter :many
WITH excluded AS MATERIALIZED (SELECT unnest(sqlc.arg(excluded_ids)::uuid[]) AS id)
SELECT l.source_id, l.range_start, octet_length(l.target)::bigint AS target_bytes
FROM page_links l
CROSS JOIN LATERAL (
 SELECT ip.node_id FROM indexed_pages ip
 WHERE ip.node_id=l.source_id AND ip.notebook_id=sqlc.arg(notebook_id)
   AND ip.extractor=sqlc.arg(extractor)
   AND l.source_id NOT IN (SELECT id FROM excluded)
  AND l.ambiguous
  AND (l.resolved_asset OR EXISTS (
    SELECT 1 FROM indexed_pages target_page
    WHERE target_page.node_id=l.resolved_id
      AND target_page.notebook_id=sqlc.arg(notebook_id)
      AND target_page.extractor=sqlc.arg(extractor)
  ))
 LIMIT 1
) current_page
WHERE l.notebook_id=sqlc.arg(notebook_id)

  AND (l.source_id,l.range_start) >
      (sqlc.arg(after_source)::uuid,sqlc.arg(after_start)::integer)
ORDER BY l.source_id,l.range_start
LIMIT (SELECT sqlc.arg(take)::integer);
