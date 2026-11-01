-- FORMAL DESIGN REVISION 01. SQL NOT EXECUTED / NOT GENERATED / NOT EXPLAINED.
-- Fixed input: 6992fd042ff06c110a4e59e67fb89bfe5f43fbd9; P3 complete, own CI accepted, formal P4 design released.
-- Proposed query names and module files are not implemented interfaces.
-- Ownership changes at the explicit section headers below; no query crosses module tables.
-- All ingest statements join the caller's write unit and its existing index lock.
-- Adapters validate input arrays, row membership/order, line_ok, live-page has_content,
-- and terminal state. Missing/stale index evidence is distinct from execution errors.

-- PAGE OWNER. First/After raw-node ranges; take is an internal 1..512.
-- max_page_depth is page/domain.MaxDepth (fixed 6992: 10), not user config.
-- agents_key is shared.TitleKey(domain.AgentsTitle), not an ad-hoc case fold.
-- name: IngestHeadsFirst :many
WITH RECURSIVE batch AS MATERIALIZED (
    SELECT id, parent_id, kind, name_key, deleted_at IS NULL AS live
    FROM nodes
    WHERE notebook_id = sqlc.arg(notebook_id)::uuid
    ORDER BY id LIMIT (SELECT sqlc.arg(take)::integer)
), walk AS (
    SELECT b.id AS leaf_id, b.id, b.parent_id, 1 AS depth,
           sqlc.arg(max_page_depth)::integer + CASE WHEN b.kind = 'asset' THEN 1 ELSE 0 END AS depth_limit,
           ARRAY[b.id]::uuid[] AS seen,
           b.id = sqlc.arg(source_root_id)::uuid AS saw_source_root,
           false AS invalid
    FROM batch b WHERE b.live
    UNION ALL
    SELECT w.leaf_id, p.id, p.parent_id, w.depth + 1, w.depth_limit,
           array_append(w.seen, p.id),
           w.saw_source_root OR coalesce(p.id = sqlc.arg(source_root_id)::uuid, false),
           (p.id IS NULL OR p.deleted_at IS NOT NULL OR p.kind <> 'page'
            OR p.id = ANY(w.seen) OR w.depth + 1 > w.depth_limit) AS invalid
    FROM walk w
    LEFT JOIN nodes p
      ON p.notebook_id = sqlc.arg(notebook_id)::uuid AND p.id = w.parent_id
    WHERE w.parent_id IS NOT NULL AND NOT w.invalid
), ancestry AS MATERIALIZED (
    SELECT leaf_id, bool_or(invalid) AS invalid,
           bool_or(parent_id IS NULL AND NOT invalid) AS reached_root,
           bool_or(saw_source_root) AS in_root_line
    FROM walk GROUP BY leaf_id
)
SELECT b.id, b.kind, b.live,
       c.node_id IS NOT NULL AS has_content, c.revision,
       coalesce(c.byte_size, 0)::bigint AS page_bytes,
       b.id = sqlc.arg(source_root_id)::uuid AS is_source_root,
       b.live AND coalesce(a.in_root_line, false)
         AND b.id <> sqlc.arg(source_root_id)::uuid AS in_source,
       b.live AND b.kind = 'page' AND b.parent_id IS NULL
         AND b.name_key = sqlc.arg(agents_key)::text AS is_root_agents,
       b.live AND NOT coalesce(a.invalid, true)
         AND coalesce(a.reached_root, false) AS line_ok
FROM batch b
LEFT JOIN ancestry a ON a.leaf_id = b.id
LEFT JOIN LATERAL (
    SELECT node_id, revision, byte_size FROM page_contents
    WHERE node_id = b.id AND deleted_at IS NULL LIMIT 1
) c ON b.live AND b.kind = 'page'
ORDER BY b.id;

-- PAGE OWNER. First/After raw-node ranges; take is an internal 1..512.
-- max_page_depth is page/domain.MaxDepth (fixed 6992: 10), not user config.
-- agents_key is shared.TitleKey(domain.AgentsTitle), not an ad-hoc case fold.
-- name: IngestHeadsAfter :many
WITH RECURSIVE batch AS MATERIALIZED (
    SELECT id, parent_id, kind, name_key, deleted_at IS NULL AS live
    FROM nodes
    WHERE notebook_id = sqlc.arg(notebook_id)::uuid AND id > sqlc.arg(after_id)::uuid
    ORDER BY id LIMIT (SELECT sqlc.arg(take)::integer)
), walk AS (
    SELECT b.id AS leaf_id, b.id, b.parent_id, 1 AS depth,
           sqlc.arg(max_page_depth)::integer + CASE WHEN b.kind = 'asset' THEN 1 ELSE 0 END AS depth_limit,
           ARRAY[b.id]::uuid[] AS seen,
           b.id = sqlc.arg(source_root_id)::uuid AS saw_source_root,
           false AS invalid
    FROM batch b WHERE b.live
    UNION ALL
    SELECT w.leaf_id, p.id, p.parent_id, w.depth + 1, w.depth_limit,
           array_append(w.seen, p.id),
           w.saw_source_root OR coalesce(p.id = sqlc.arg(source_root_id)::uuid, false),
           (p.id IS NULL OR p.deleted_at IS NOT NULL OR p.kind <> 'page'
            OR p.id = ANY(w.seen) OR w.depth + 1 > w.depth_limit) AS invalid
    FROM walk w
    LEFT JOIN nodes p
      ON p.notebook_id = sqlc.arg(notebook_id)::uuid AND p.id = w.parent_id
    WHERE w.parent_id IS NOT NULL AND NOT w.invalid
), ancestry AS MATERIALIZED (
    SELECT leaf_id, bool_or(invalid) AS invalid,
           bool_or(parent_id IS NULL AND NOT invalid) AS reached_root,
           bool_or(saw_source_root) AS in_root_line
    FROM walk GROUP BY leaf_id
)
SELECT b.id, b.kind, b.live,
       c.node_id IS NOT NULL AS has_content, c.revision,
       coalesce(c.byte_size, 0)::bigint AS page_bytes,
       b.id = sqlc.arg(source_root_id)::uuid AS is_source_root,
       b.live AND coalesce(a.in_root_line, false)
         AND b.id <> sqlc.arg(source_root_id)::uuid AS in_source,
       b.live AND b.kind = 'page' AND b.parent_id IS NULL
         AND b.name_key = sqlc.arg(agents_key)::text AS is_root_agents,
       b.live AND NOT coalesce(a.invalid, true)
         AND coalesce(a.reached_root, false) AS line_ok
FROM batch b
LEFT JOIN ancestry a ON a.leaf_id = b.id
LEFT JOIN LATERAL (
    SELECT node_id, revision, byte_size FROM page_contents
    WHERE node_id = b.id AND deleted_at IS NULL LIMIT 1
) c ON b.live AND b.kind = 'page'
ORDER BY b.id;

-- LINKING OWNER. Complete one row for each requested Before W/target, Now target, or coverage wiki/source page.
-- Equal-length, sorted unique arrays, <=512. A missing row is Present=false, not absence of references.
-- name: IngestIndexedHeads :many
WITH requested AS MATERIALIZED (
    SELECT unnest(sqlc.arg(page_ids)::uuid[]) AS id,
           unnest(sqlc.arg(page_revisions)::integer[]) AS revision
)
SELECT r.id, r.revision AS page_revision,
       ip.node_id IS NOT NULL AS present,
       ip.revision AS indexed_revision, ip.extractor, ip.frontmatter_valid
FROM requested r LEFT JOIN indexed_pages ip
  ON ip.node_id = r.id AND ip.notebook_id = sqlc.arg(notebook_id)::uuid
ORDER BY r.id;

-- LINKING OWNER. One final batch statement after Capture, updates, resolution and complete coverage.
-- R distinct old-F pairs stay in app; SQL receives only distinct parent_ids.
-- possible_old_direct_ids contains ALL known and possible D_W; not the inherited closure.
-- current_source_ids/revisions is a complete qualified live source relation, including W.
-- untouched_source_ids is its untouched subset; eligible_emitter_ids is untouched current wiki.
-- App validates proof binding, aligned sorted unique arrays, <=512 HEAD point reads upstream.
-- Query arrays are complete total sets: 512 is NOT their total limit.
-- SQL returns all candidates and requested parents; app projects old F only after ALL rows/errors.
-- name: IngestNovelSources :many
WITH candidates AS MATERIALIZED (
    SELECT unnest(sqlc.arg(candidate_ids)::uuid[]) AS id,
           unnest(sqlc.arg(candidate_assets)::boolean[]) AS asset,
           unnest(sqlc.arg(candidate_revisions)::integer[]) AS revision
), source_proofs AS MATERIALIZED (
    SELECT unnest(sqlc.arg(current_source_ids)::uuid[]) AS id,
           unnest(sqlc.arg(current_source_revisions)::integer[]) AS revision
), current_sources AS MATERIALIZED (
    SELECT s.id, s.revision FROM source_proofs s JOIN indexed_pages ip
      ON ip.node_id = s.id AND ip.notebook_id = sqlc.arg(notebook_id)::uuid
      AND ip.extractor = sqlc.arg(extractor)::integer AND ip.revision = s.revision
), parents AS MATERIALIZED (
    SELECT unnest(sqlc.arg(parent_ids)::uuid[]) AS id
), eligible AS MATERIALIZED (
    SELECT unnest(sqlc.arg(eligible_emitter_ids)::uuid[]) AS id
), untouched_sources AS MATERIALIZED (
    SELECT unnest(sqlc.arg(untouched_source_ids)::uuid[]) AS id
), followed_untouched AS MATERIALIZED (
    SELECT DISTINCT l.source_id, c.id AS asset_id
    FROM candidates c JOIN page_links l
      ON c.asset AND l.resolved_id = c.id AND l.resolved_asset
    JOIN untouched_sources u ON u.id = l.source_id
    JOIN current_sources s ON s.id = l.source_id
    WHERE l.notebook_id = sqlc.arg(notebook_id)::uuid AND l.property_key IS NULL
), wanted AS MATERIALIZED (
    SELECT id, asset FROM candidates
    UNION SELECT source_id, false FROM followed_untouched
    UNION SELECT id, false FROM parents
), relevant AS MATERIALIZED (
    -- Reverse hits discover whole source pages only, never prefilter same-slot peers.
    SELECT DISTINCT l.source_id, ip.frontmatter_valid
    FROM wanted w JOIN page_links l ON l.resolved_id = w.id AND l.resolved_asset = w.asset
    JOIN eligible e ON e.id = l.source_id
    JOIN indexed_pages ip ON ip.node_id = l.source_id
      AND ip.notebook_id = sqlc.arg(notebook_id)::uuid
      AND ip.extractor = sqlc.arg(extractor)::integer
    WHERE l.notebook_id = sqlc.arg(notebook_id)::uuid
), digested AS MATERIALIZED (
    SELECT j.id, j.asset, bool_or(j.invalid_source_value) AS invalid_source_value
    FROM relevant r CROSS JOIN LATERAL (
        WITH first_json AS MATERIALIZED (
            SELECT p.key, p.value
            FROM page_properties p
            WHERE r.frontmatter_valid AND p.source_id = r.source_id
              AND translate(p.key, 'SOURCE', 'source') = 'sources'
            ORDER BY p.position LIMIT 1
        ), shape AS MATERIALIZED (
            SELECT key, value, jsonb_typeof(value) AS value_kind,
                   (CASE WHEN jsonb_typeof(value) = 'array'
                     THEN jsonb_array_length(value) > sqlc.arg(max_slot_items)::integer
                     ELSE false END
                    OR octet_length(value::text)::bigint > sqlc.arg(max_json_bytes)::bigint)
                    AS invalid_source_value
            FROM first_json
        ), slots AS MATERIALIZED (
            SELECT CASE WHEN s.value_kind = 'array'
                   THEN s.key || '.' || (a.ordinality - 1)::text
                   ELSE s.key END AS property_key
            FROM shape s
            CROSS JOIN LATERAL jsonb_array_elements(
                CASE WHEN s.invalid_source_value THEN '[]'::jsonb
                     WHEN s.value_kind = 'array' THEN s.value
                     WHEN s.value_kind = 'string' THEN jsonb_build_array(s.value)
                     ELSE '[]'::jsonb END
            ) WITH ORDINALITY a(value, ordinality)
            WHERE jsonb_typeof(a.value) = 'string'
        ), peers AS MATERIALIZED (
            -- No resolved_id, resolved_asset or candidate filter in this relation.
            -- Different range_start rows count separately, even when their target text/ID is equal.
            SELECT l.property_key, l.resolved_id, l.resolved_asset,
                   (l.target = '' OR (l.resolved_asset AND l.resolved_id IS NULL)) AS invalid_peer,
                   count(*) OVER (PARTITION BY l.property_key) AS matching_edges
            FROM page_links l JOIN slots s ON s.property_key = l.property_key
            WHERE l.source_id = r.source_id
              AND l.notebook_id = sqlc.arg(notebook_id)::uuid
        ), qualified AS (
            SELECT l.resolved_id AS id, l.resolved_asset AS asset
            FROM page_links l
            WHERE l.source_id = r.source_id
              AND l.notebook_id = sqlc.arg(notebook_id)::uuid
              AND l.property_key IS NULL AND l.resolved_id IS NOT NULL
            UNION ALL
            SELECT resolved_id, resolved_asset FROM peers
            WHERE matching_edges = 1 AND resolved_id IS NOT NULL
        )
        SELECT c.id, c.asset, false AS invalid_source_value
        FROM qualified q JOIN wanted c ON c.id = q.id AND c.asset = q.asset
        WHERE c.asset OR EXISTS (SELECT 1 FROM current_sources s WHERE s.id = c.id)
        UNION ALL
        SELECT NULL::uuid, false, true
        WHERE EXISTS (SELECT 1 FROM shape WHERE invalid_source_value)
           OR EXISTS (SELECT 1 FROM peers WHERE invalid_peer)
    ) j
    GROUP BY j.id, j.asset
), possible_direct AS MATERIALIZED (
    SELECT unnest(sqlc.arg(possible_old_direct_ids)::uuid[]) AS id
    UNION SELECT id FROM digested WHERE id IS NOT NULL
), flags AS MATERIALIZED (
    SELECT EXISTS (SELECT 1 FROM digested WHERE invalid_source_value) AS invalid_source_value,
      (EXISTS (SELECT 1 FROM source_proofs s WHERE NOT EXISTS
          (SELECT 1 FROM current_sources c WHERE c.id = s.id AND c.revision = s.revision))
       OR EXISTS (SELECT 1 FROM candidates c WHERE NOT c.asset AND NOT EXISTS
          (SELECT 1 FROM current_sources s WHERE s.id = c.id AND s.revision = c.revision))) AS invalid_proof
), output AS (
    SELECT 'candidate'::text AS row_kind, c.id, c.asset,
      EXISTS (SELECT 1 FROM digested d WHERE d.id = c.id AND d.asset = c.asset) AS directly_digested,
      EXISTS (SELECT 1 FROM followed_untouched f JOIN possible_direct d ON d.id = f.source_id
              WHERE c.asset AND f.asset_id = c.id) AS followed_by_untouched
    FROM candidates c
    UNION ALL
    SELECT 'parent'::text, p.id, false,
      EXISTS (SELECT 1 FROM digested d WHERE d.id = p.id AND NOT d.asset), false
    FROM parents p
)
SELECT o.row_kind, o.id, o.asset, o.directly_digested, o.followed_by_untouched,
       f.invalid_source_value, f.invalid_proof
FROM output o CROSS JOIN flags f ORDER BY o.row_kind, o.id;

-- PAGE OWNER. Proposed changeset_sources/ingest_count DDL is not present at the fixed commit.
-- One insert/count statement; caller already owns its changeset row through Unit creation/Touch/lockInto.
-- source_ids is a sorted unique complete proven-novel input; max_sources is the code constant 10000.
-- No FK to nodes: source deletion does not remove history. No ON CONFLICT masks invariant failure.
-- Adapter requires exactly one updated row; a missing/wrong-kind/wrong-book changeset is an internal error.
-- name: AddIngestMarks :one
WITH owned AS MATERIALIZED (
    SELECT id, ingest_count FROM changesets
    WHERE id = sqlc.arg(changeset_id)::uuid
      AND notebook_id = sqlc.arg(notebook_id)::uuid AND kind = 'edit' AND deleted_at IS NULL
    FOR NO KEY UPDATE
), wanted AS MATERIALIZED (
    SELECT DISTINCT unnest(sqlc.arg(source_ids)::uuid[]) AS node_id
), chosen AS MATERIALIZED (
    SELECT w.node_id FROM wanted w
    WHERE NOT EXISTS (
        SELECT 1 FROM changeset_sources s
        WHERE s.changeset_id = sqlc.arg(changeset_id)::uuid AND s.node_id = w.node_id
    )
    ORDER BY w.node_id
    LIMIT coalesce((SELECT greatest(0, sqlc.arg(max_sources)::integer - ingest_count) FROM owned), 0)
), inserted AS (
    INSERT INTO changeset_sources (changeset_id, seq, node_id)
    SELECT o.id, o.ingest_count + row_number() OVER (ORDER BY c.node_id)::integer, c.node_id
    FROM owned o CROSS JOIN chosen c
    RETURNING changeset_id
), updated AS (
    UPDATE changesets c
    SET ingest_count = c.ingest_count + (SELECT count(*)::integer FROM inserted)
    FROM owned o WHERE c.id = o.id
    RETURNING c.id, c.ingest_count
)
SELECT u.id, u.ingest_count, (SELECT count(*)::integer FROM inserted) AS added
FROM updated u;

-- PAGE OWNER. Planned migration: server/migrations/sql/00053_page_ingest.sql; no SQL executed.
-- +goose Up
ALTER TABLE changesets ADD COLUMN ingest_count integer NOT NULL DEFAULT 0
    CONSTRAINT changesets_ingest_count_check CHECK (ingest_count >= 0);
CREATE TABLE changeset_sources (
    changeset_id uuid NOT NULL, seq integer NOT NULL, node_id uuid NOT NULL,
    CONSTRAINT changeset_sources_pkey PRIMARY KEY (changeset_id, seq),
    CONSTRAINT changeset_sources_changeset_id_node_id_key UNIQUE (changeset_id, node_id)
);
CREATE INDEX changesets_notebook_id_created_at_ingest_idx
    ON changesets (notebook_id, created_at DESC, id DESC) WHERE ingest_count > 0;
CREATE INDEX changesets_notebook_id_created_by_id_created_at_ingest_idx
    ON changesets (notebook_id, created_by_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL AND ingest_count > 0;
-- Down candidate (separate actual goose section in final migration):
-- DROP INDEX changesets_notebook_id_created_at_ingest_idx,
--   changesets_notebook_id_created_by_id_created_at_ingest_idx;
-- DROP TABLE changeset_sources;
-- ALTER TABLE changesets DROP COLUMN ingest_count;
-- Runtime grants/sqlc schema in the SAME commit; no FK/no deleted_at/no MAINTAIN/no backfill.

-- PAGE OWNER. Nullable preview retains the first n seq positions even after deletion/purge.
-- Caller already authorized every requested changeset; n is one of 3/5/20.
-- name: FirstIngestSources :many
SELECT r.id::uuid AS changeset_id, s.seq,
       CASE WHEN n.id IS NULL THEN NULL::uuid ELSE s.node_id END AS node_id
FROM (SELECT unnest(sqlc.arg(changeset_ids)::uuid[]) AS id) r
JOIN changesets c ON c.id = r.id
CROSS JOIN LATERAL (
    SELECT seq, node_id FROM changeset_sources
    WHERE changeset_id = r.id ORDER BY seq LIMIT sqlc.arg(preview_count)::integer
) s
LEFT JOIN nodes n ON n.id = s.node_id AND n.notebook_id = c.notebook_id AND n.deleted_at IS NULL
ORDER BY r.id, s.seq;

-- PAGE OWNER. One bounded leaf delete; outer Store.PurgeChangesets repeats until a short batch,
-- adds leaf rows to its returned count, then runs BOTH guarded parent branches.
-- A skipped held changeset is intentionally retained for a later run; no wait/deadlock.
-- name: PurgeChangesetSources :execrows
WITH doomed AS (
    SELECT s.changeset_id, s.seq
    FROM changeset_sources s JOIN changesets c ON c.id = s.changeset_id
    WHERE (c.deleted_at < sqlc.arg(before)::timestamptz
        OR (c.trimmed AND c.deleted_at IS NULL))
      AND NOT EXISTS (SELECT 1 FROM changeset_items i WHERE i.changeset_id = c.id)
      AND NOT EXISTS (SELECT 1 FROM page_revisions v WHERE v.changeset_id = c.id)
    LIMIT sqlc.arg(batch)::integer
    FOR UPDATE OF s SKIP LOCKED FOR NO KEY UPDATE OF c SKIP LOCKED
)
DELETE FROM changeset_sources s USING doomed d
WHERE s.changeset_id = d.changeset_id AND s.seq = d.seq;

-- name: PurgeChangesetsWithIngestGuard :execrows
DELETE FROM changesets WHERE id IN (
    SELECT c.id FROM changesets c
    WHERE c.deleted_at < sqlc.arg(before)::timestamptz
      AND NOT EXISTS (SELECT 1 FROM changeset_items i WHERE i.changeset_id = c.id)
      AND NOT EXISTS (SELECT 1 FROM page_revisions v WHERE v.changeset_id = c.id)
      AND NOT EXISTS (SELECT 1 FROM changeset_sources s WHERE s.changeset_id = c.id)
    LIMIT sqlc.arg(batch)::integer FOR UPDATE SKIP LOCKED
);

-- name: PurgeTrimmedChangesetsWithIngestGuard :execrows
DELETE FROM changesets WHERE id IN (
    SELECT c.id FROM changesets c
    WHERE c.trimmed AND c.deleted_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM changeset_items i WHERE i.changeset_id = c.id)
      AND NOT EXISTS (SELECT 1 FROM page_revisions v WHERE v.changeset_id = c.id)
      AND NOT EXISTS (SELECT 1 FROM changeset_sources s WHERE s.changeset_id = c.id)
    LIMIT sqlc.arg(batch)::integer FOR UPDATE SKIP LOCKED
);
-- Production keeps the existing PurgeChangesets/PurgeTrimmedChangesets query names;
-- ...WithIngestGuard above distinguishes candidate replacements within this annex only.

-- PAGE OWNER. Dedicated TRUE branch; false/omitted keep original query dispatch.
-- name: ListIngestChangesets :many
-- A notebook's log newer first below the cursor (M8 design 4.7): a range of its index backward, whatever plan is kept;
-- the client a filter on it, its values few. The brief of the changeset a revert reverts by its key.
SELECT c.id, c.notebook_id, c.kind, c.reverts_id, c.client, c.message, c.created_by_id, c.created_at, c.updated_at,
    c.trimmed, c.ingest_count, v.kind AS reverts_kind, v.created_by_id AS reverts_by, v.created_at AS reverts_at
FROM changesets c LEFT JOIN changesets v ON v.id = c.reverts_id
WHERE c.notebook_id = sqlc.arg(notebook_id) AND c.deleted_at IS NULL AND c.ingest_count > 0
    AND (c.created_at, c.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
    AND (sqlc.narg(client)::text IS NULL OR c.client = sqlc.narg(client)::text)
    AND (NOT sqlc.arg(all_mcp)::boolean OR c.client LIKE 'mcp:%')
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);

-- PAGE OWNER. Dedicated TRUE branch; false/omitted keep original query dispatch.
-- name: ListIngestChangesetsBy :many
-- ListChangesets of one actor: a range of the actor's index backward. A statement of its own: a cached statement's
-- plan for any arguments serves one of the filters' paths.
SELECT c.id, c.notebook_id, c.kind, c.reverts_id, c.client, c.message, c.created_by_id, c.created_at, c.updated_at,
    c.trimmed, c.ingest_count, v.kind AS reverts_kind, v.created_by_id AS reverts_by, v.created_at AS reverts_at
FROM changesets c LEFT JOIN changesets v ON v.id = c.reverts_id
WHERE c.notebook_id = sqlc.arg(notebook_id) AND c.created_by_id = sqlc.arg(actor) AND c.deleted_at IS NULL AND c.ingest_count > 0
    AND (c.created_at, c.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
    AND (sqlc.narg(client)::text IS NULL OR c.client = sqlc.narg(client)::text)
    AND (NOT sqlc.arg(all_mcp)::boolean OR c.client LIKE 'mcp:%')
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);

-- PAGE OWNER. Dedicated TRUE branch; false/omitted keep original query dispatch.
-- name: ListIngestChangesetsOfPage :many
-- ListChangesets of the changesets that changed a node, of one actor when it is set: the node's items' and versions'
-- changesets through their indexes on node_id, deleted or not (a node in the trash has its history), then each by its
-- key alone, as TrashLines reads a parent, sorted. Its cost is the node's history's: joined as the planner pleased,
-- without statistics a page of a 40,000 changesets' history walked the log's index and compared each with them all,
-- 7 s a page.
WITH touched AS (
    SELECT i.changeset_id FROM changeset_items i WHERE i.node_id = sqlc.arg(page_id)
    UNION
    SELECT r.changeset_id FROM page_revisions r WHERE r.node_id = sqlc.arg(page_id)
), changed AS (
    SELECT x.id, x.notebook_id, x.kind, x.reverts_id, x.client, x.message, x.created_by_id, x.created_at, x.updated_at,
        x.trimmed, x.ingest_count, x.deleted_at
    FROM touched t CROSS JOIN LATERAL (SELECT * FROM changesets WHERE id = t.changeset_id LIMIT 1) x
)
SELECT c.id, c.notebook_id, c.kind, c.reverts_id, c.client, c.message, c.created_by_id, c.created_at, c.updated_at,
    c.trimmed, c.ingest_count, v.kind AS reverts_kind, v.created_by_id AS reverts_by, v.created_at AS reverts_at
FROM changed c LEFT JOIN changesets v ON v.id = c.reverts_id
WHERE c.notebook_id = sqlc.arg(notebook_id)::uuid AND c.deleted_at IS NULL AND c.ingest_count > 0
    AND (c.created_at, c.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
    AND (sqlc.narg(actor)::uuid IS NULL OR c.created_by_id = sqlc.narg(actor)::uuid)
    AND (sqlc.narg(client)::text IS NULL OR c.client = sqlc.narg(client)::text)
    AND (NOT sqlc.arg(all_mcp)::boolean OR c.client LIKE 'mcp:%')
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);

-- LINKING OWNER. Replacement IndexPages: typed means current stored valid FM + first ASCII type
-- property string is nonempty under shared jsTrim. Classification is independent of filter.
-- Caller supplies the exact established JS whitespace set as trim_chars, never a new approximation.
-- type_is remains EXACT text equality and max 1024B at the public boundary.
-- name: IndexPagesP4 :many
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

-- LINKING OWNER. IndexItems type branch; summary retains its existing representation.
-- This replacement's columns match original IndexItems plus its corrected typed Boolean.
-- name: IndexItemsP4 :many
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

-- PAGE OWNER. ChildPagesAt makes root/null distinct from a zero UUID parent.
-- name: IndexChildPagesAt :many
SELECT id FROM nodes WHERE notebook_id=sqlc.arg(notebook_id)::uuid AND deleted_at IS NULL
  AND kind='page' AND parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)::uuid;

-- LINKING OWNER. Proposed bounded catalogue headers, <=512 requested IDs; not a public new mode.
-- Caller owns a FINITE complete first-pass ID array. No DISTINCT-value unbounded materialization.
-- A typed NULL filter_type is explicitly an unfilterable complete stored value >1024 UTF-8 bytes.
-- app visits all batches, keeps bounded Top-K counts and emits explicit completion fields.
-- name: IndexCatalogueHeads :many
WITH requested AS MATERIALIZED (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id)
SELECT r.id, coalesce(t.nonempty,false)::boolean AS typed,
       CASE WHEN t.nonempty AND octet_length(t.filter_value)<=1024
            THEN t.filter_value ELSE NULL END::text AS filter_type
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

-- PAGE OWNER. Local current qualification, <=512 requested IDs, including missing as explicit false.
-- Metadata alone is insufficient for source pages: app also verifies matching IngestIndexedHeads.
-- Point qualification precedes full coverage; assets do not require a page head.
-- Caller validates sorted unique rows and applies line_ok only to live nodes.
-- name: IngestNodesNow :many
WITH RECURSIVE batch AS MATERIALIZED (
    SELECT r.id, n.parent_id, n.kind, n.name_key,
           n.id IS NOT NULL AS node_exists,
           n.id IS NOT NULL AND n.deleted_at IS NULL AS live
    FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id) r
    LEFT JOIN nodes n ON n.id=r.id AND n.notebook_id=sqlc.arg(notebook_id)::uuid
), walk AS (
    SELECT b.id AS leaf_id, b.id, b.parent_id, 1 AS depth,
           sqlc.arg(max_page_depth)::integer + CASE WHEN b.kind = 'asset' THEN 1 ELSE 0 END AS depth_limit,
           ARRAY[b.id]::uuid[] AS seen,
           b.id = sqlc.arg(source_root_id)::uuid AS saw_source_root,
           false AS invalid
    FROM batch b WHERE b.live
    UNION ALL
    SELECT w.leaf_id, p.id, p.parent_id, w.depth + 1, w.depth_limit,
           array_append(w.seen, p.id),
           w.saw_source_root OR coalesce(p.id = sqlc.arg(source_root_id)::uuid, false),
           (p.id IS NULL OR p.deleted_at IS NOT NULL OR p.kind <> 'page'
            OR p.id = ANY(w.seen) OR w.depth + 1 > w.depth_limit) AS invalid
    FROM walk w
    LEFT JOIN nodes p
      ON p.notebook_id = sqlc.arg(notebook_id)::uuid AND p.id = w.parent_id
    WHERE w.parent_id IS NOT NULL AND NOT w.invalid
), ancestry AS MATERIALIZED (
    SELECT leaf_id, bool_or(invalid) AS invalid,
           bool_or(parent_id IS NULL AND NOT invalid) AS reached_root,
           bool_or(saw_source_root) AS in_root_line
    FROM walk GROUP BY leaf_id
)
SELECT b.id, b.node_exists, b.kind, b.live,
       c.node_id IS NOT NULL AS has_content, c.revision,
       coalesce(c.byte_size, 0)::bigint AS page_bytes,
       b.id = sqlc.arg(source_root_id)::uuid AS is_source_root,
       b.live AND coalesce(a.in_root_line, false)
         AND b.id <> sqlc.arg(source_root_id)::uuid AS in_source,
       b.live AND b.kind = 'page' AND b.parent_id IS NULL
         AND b.name_key = sqlc.arg(agents_key)::text AS is_root_agents,
       b.live AND NOT coalesce(a.invalid, true)
         AND coalesce(a.reached_root, false) AS line_ok
FROM batch b
LEFT JOIN ancestry a ON a.leaf_id = b.id
LEFT JOIN LATERAL (
    SELECT node_id, revision, byte_size FROM page_contents
    WHERE node_id = b.id AND deleted_at IS NULL LIMIT 1
) c ON b.live AND b.kind = 'page'
ORDER BY b.id;
