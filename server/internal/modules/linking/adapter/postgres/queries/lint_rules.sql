-- Rule counts and items have the same eligibility predicates. Counts stop
-- after 10,000 qualifying pages; item cursors never inherit that cap.
-- LIMIT 1 keeps the complete candidate relation driving PK page checks. Within
-- anti-EXISTS, the resolved target range drives links, while a correlated PK
-- LATERAL checks notebook/current/full-set membership. Keeping those predicates
-- inside the LATERAL prevents notebook BitmapAnd rescans or 100k parent probes
-- chosen by the normal no-statistics/generic planner (physical matrix guard).

-- name: LintOrphanCount :one
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id),
 excluded_emitters AS MATERIALIZED (SELECT unnest(sqlc.arg(excluded_emitters)::uuid[]) AS id),
 qualified AS (
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
WHERE true
 AND NOT EXISTS (
  SELECT 1 FROM page_links l CROSS JOIN LATERAL (
   SELECT e.node_id FROM indexed_pages e
   WHERE e.node_id=l.source_id AND e.notebook_id=sqlc.arg(notebook_id)
    AND e.extractor=sqlc.arg(extractor)
    AND l.notebook_id=sqlc.arg(notebook_id) AND NOT l.resolved_asset
    AND l.source_id<>ip.node_id
    AND e.node_id NOT IN (SELECT id FROM excluded_emitters)
   LIMIT 1
  ) current_emitter
  WHERE l.resolved_id=ip.node_id
 )
LIMIT 10000
)
SELECT count(*)::bigint FROM qualified;

-- name: LintOrphanItemsFirst :many
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id ORDER BY id),
 excluded_emitters AS MATERIALIZED (SELECT unnest(sqlc.arg(excluded_emitters)::uuid[]) AS id)
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
WHERE true
 AND NOT EXISTS (
  SELECT 1 FROM page_links l CROSS JOIN LATERAL (
   SELECT e.node_id FROM indexed_pages e
   WHERE e.node_id=l.source_id AND e.notebook_id=sqlc.arg(notebook_id)
    AND e.extractor=sqlc.arg(extractor)
    AND l.notebook_id=sqlc.arg(notebook_id) AND NOT l.resolved_asset
    AND l.source_id<>ip.node_id
    AND e.node_id NOT IN (SELECT id FROM excluded_emitters)
   LIMIT 1
  ) current_emitter
  WHERE l.resolved_id=ip.node_id
 )
ORDER BY c.id LIMIT sqlc.arg(take)::integer;

-- name: LintOrphanItemsAfter :many
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id ORDER BY id),
 excluded_emitters AS MATERIALIZED (SELECT unnest(sqlc.arg(excluded_emitters)::uuid[]) AS id)
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
WHERE true
 AND NOT EXISTS (
  SELECT 1 FROM page_links l CROSS JOIN LATERAL (
   SELECT e.node_id FROM indexed_pages e
   WHERE e.node_id=l.source_id AND e.notebook_id=sqlc.arg(notebook_id)
    AND e.extractor=sqlc.arg(extractor)
    AND l.notebook_id=sqlc.arg(notebook_id) AND NOT l.resolved_asset
    AND l.source_id<>ip.node_id
    AND e.node_id NOT IN (SELECT id FROM excluded_emitters)
   LIMIT 1
  ) current_emitter
  WHERE l.resolved_id=ip.node_id
 )
 AND c.id>sqlc.arg(after_id)::uuid
ORDER BY c.id LIMIT sqlc.arg(take)::integer;

-- name: LintFrontmatterCount :one
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id),
 qualified AS (
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
WHERE true
 AND NOT ip.frontmatter_valid
LIMIT 10000
)
SELECT count(*)::bigint FROM qualified;

-- name: LintFrontmatterItemsFirst :many
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id)
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
WHERE true
 AND NOT ip.frontmatter_valid
ORDER BY ip.node_id LIMIT sqlc.arg(take)::integer;

-- name: LintFrontmatterItemsAfter :many
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id)
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
WHERE true
 AND NOT ip.frontmatter_valid
 AND c.id>sqlc.arg(after_id)::uuid
ORDER BY ip.node_id LIMIT sqlc.arg(take)::integer;

-- name: LintMissingTypeCount :one
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id),
 qualified AS (
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
LEFT JOIN LATERAL (
 SELECT CASE
  WHEN p.value_start IS NULL THEN true
  WHEN btrim(p.value_start,sqlc.arg(trim_chars)::text)<>'' THEN false
  WHEN p.value_chars<=257 THEN true
  ELSE btrim(p.value #>> '{}',sqlc.arg(trim_chars)::text)=''
 END AS missing
 FROM page_properties p
 WHERE p.source_id=ip.node_id AND translate(p.key,'TYPE','type')='type'
 ORDER BY p.position LIMIT 1
) prop ON true
WHERE true
 AND ip.frontmatter_valid AND COALESCE(prop.missing,true)
LIMIT 10000
)
SELECT count(*)::bigint FROM qualified;

-- name: LintMissingTypeItemsFirst :many
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id)
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
LEFT JOIN LATERAL (
 SELECT CASE
  WHEN p.value_start IS NULL THEN true
  WHEN btrim(p.value_start,sqlc.arg(trim_chars)::text)<>'' THEN false
  WHEN p.value_chars<=257 THEN true
  ELSE btrim(p.value #>> '{}',sqlc.arg(trim_chars)::text)=''
 END AS missing
 FROM page_properties p
 WHERE p.source_id=ip.node_id AND translate(p.key,'TYPE','type')='type'
 ORDER BY p.position LIMIT 1
) prop ON true
WHERE true
 AND ip.frontmatter_valid AND COALESCE(prop.missing,true)
ORDER BY ip.node_id LIMIT sqlc.arg(take)::integer;

-- name: LintMissingTypeItemsAfter :many
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id)
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
LEFT JOIN LATERAL (
 SELECT CASE
  WHEN p.value_start IS NULL THEN true
  WHEN btrim(p.value_start,sqlc.arg(trim_chars)::text)<>'' THEN false
  WHEN p.value_chars<=257 THEN true
  ELSE btrim(p.value #>> '{}',sqlc.arg(trim_chars)::text)=''
 END AS missing
 FROM page_properties p
 WHERE p.source_id=ip.node_id AND translate(p.key,'TYPE','type')='type'
 ORDER BY p.position LIMIT 1
) prop ON true
WHERE true
 AND ip.frontmatter_valid AND COALESCE(prop.missing,true)
 AND c.id>sqlc.arg(after_id)::uuid
ORDER BY ip.node_id LIMIT sqlc.arg(take)::integer;

-- name: LintMissingSummaryCount :one
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id),
 qualified AS (
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
LEFT JOIN LATERAL (
 SELECT CASE
  WHEN p.value_start IS NULL THEN true
  WHEN btrim(p.value_start,sqlc.arg(trim_chars)::text)<>'' THEN false
  WHEN p.value_chars<=257 THEN true
  ELSE btrim(p.value #>> '{}',sqlc.arg(trim_chars)::text)=''
 END AS missing
 FROM page_properties p
 WHERE p.source_id=ip.node_id AND translate(p.key,'SUMARY','sumary')='summary'
 ORDER BY p.position LIMIT 1
) prop ON true
WHERE true
 AND ip.frontmatter_valid AND COALESCE(prop.missing,true)
LIMIT 10000
)
SELECT count(*)::bigint FROM qualified;

-- name: LintMissingSummaryItemsFirst :many
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id)
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
LEFT JOIN LATERAL (
 SELECT CASE
  WHEN p.value_start IS NULL THEN true
  WHEN btrim(p.value_start,sqlc.arg(trim_chars)::text)<>'' THEN false
  WHEN p.value_chars<=257 THEN true
  ELSE btrim(p.value #>> '{}',sqlc.arg(trim_chars)::text)=''
 END AS missing
 FROM page_properties p
 WHERE p.source_id=ip.node_id AND translate(p.key,'SUMARY','sumary')='summary'
 ORDER BY p.position LIMIT 1
) prop ON true
WHERE true
 AND ip.frontmatter_valid AND COALESCE(prop.missing,true)
ORDER BY ip.node_id LIMIT sqlc.arg(take)::integer;

-- name: LintMissingSummaryItemsAfter :many
WITH candidates AS MATERIALIZED (SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id)
SELECT ip.node_id
FROM candidates c CROSS JOIN LATERAL (
 SELECT ip.node_id,ip.frontmatter_valid FROM indexed_pages ip
 WHERE ip.node_id=c.id AND ip.notebook_id=sqlc.arg(notebook_id)
  AND ip.extractor=sqlc.arg(extractor) LIMIT 1
) ip
LEFT JOIN LATERAL (
 SELECT CASE
  WHEN p.value_start IS NULL THEN true
  WHEN btrim(p.value_start,sqlc.arg(trim_chars)::text)<>'' THEN false
  WHEN p.value_chars<=257 THEN true
  ELSE btrim(p.value #>> '{}',sqlc.arg(trim_chars)::text)=''
 END AS missing
 FROM page_properties p
 WHERE p.source_id=ip.node_id AND translate(p.key,'SUMARY','sumary')='summary'
 ORDER BY p.position LIMIT 1
) prop ON true
WHERE true
 AND ip.frontmatter_valid AND COALESCE(prop.missing,true)
 AND c.id>sqlc.arg(after_id)::uuid
ORDER BY ip.node_id LIMIT sqlc.arg(take)::integer;

-- Parents is the full nonempty source-page relation, including stale pages.
-- The indexed_pages join establishes current P here; it is not redundant with
-- an app-side current filter. Only SQL NULL denotes a body reference in F.
-- The inner limit applies after all F qualifications and keeps EXISTS from
-- flattening into a whole-link hash anti join when assets has 512 rows.
-- IS TRUE keeps membership a once-built hashed SubPlan rather than a repeated
-- semi join; the scalar LIMIT InitPlan avoids the no-statistics startup plan
-- choosing a whole-table scan for each asset. Both are actual-plan guarded.
-- name: LintPendingAssets :many
WITH assets AS MATERIALIZED (SELECT unnest(sqlc.arg(asset_ids)::uuid[]) AS id),
 parents AS MATERIALIZED (SELECT unnest(sqlc.arg(parent_ids)::uuid[]) AS id)
SELECT a.id::uuid FROM assets a
WHERE NOT EXISTS (
 SELECT 1 FROM (
 SELECT 1 FROM page_links l CROSS JOIN LATERAL (
  SELECT p.node_id FROM indexed_pages p
  WHERE p.node_id=l.source_id AND p.notebook_id=sqlc.arg(notebook_id)
   AND p.extractor=sqlc.arg(extractor)
   AND l.notebook_id=sqlc.arg(notebook_id)
   AND l.resolved_asset AND l.property_key IS NULL
   AND (l.source_id IN (SELECT id FROM parents)) IS TRUE
  LIMIT 1
 ) current_parent
 WHERE l.resolved_id=a.id LIMIT (SELECT 1)
 ) followed
)
ORDER BY a.id;
