-- Private pre-Replace old-F evidence under the notebook index lock.
-- Emitters have complete current Before heads; public Capture/Lint use lint.sql.

-- name: IngestOldBodyReferencesFirst :many
WITH emitters AS MATERIALIZED (SELECT unnest(sqlc.arg(wiki_page_ids)::uuid[]) AS id)
SELECT l.source_id,l.range_start,l.resolved_id,l.resolved_asset
FROM page_links l
CROSS JOIN LATERAL (
 SELECT 1
 WHERE l.source_id IN (SELECT id FROM emitters)
   AND l.property_key IS NULL AND l.resolved_id IS NOT NULL
 LIMIT 1
) qualified_body
WHERE l.notebook_id=sqlc.arg(notebook_id)
 AND l.source_id >= sqlc.arg(first_source)::uuid
 AND l.source_id <= sqlc.arg(last_source)::uuid
ORDER BY l.source_id,l.range_start LIMIT (SELECT sqlc.arg(take)::integer);

-- name: IngestOldBodyReferencesAfter :many
WITH emitters AS MATERIALIZED (SELECT unnest(sqlc.arg(wiki_page_ids)::uuid[]) AS id)
SELECT l.source_id,l.range_start,l.resolved_id,l.resolved_asset
FROM page_links l
CROSS JOIN LATERAL (
 SELECT 1
 WHERE l.source_id IN (SELECT id FROM emitters)
   AND l.property_key IS NULL AND l.resolved_id IS NOT NULL
 LIMIT 1
) qualified_body
WHERE l.notebook_id=sqlc.arg(notebook_id)
 AND l.source_id >= sqlc.arg(first_source)::uuid
 AND l.source_id <= sqlc.arg(last_source)::uuid
 AND (l.source_id,l.range_start)>(sqlc.arg(after_source)::uuid,sqlc.arg(after_start)::integer)
ORDER BY l.source_id,l.range_start LIMIT (SELECT sqlc.arg(take)::integer);
