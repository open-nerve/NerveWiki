-- The full-text search (M8/P5 design 3.4, 3.5): the queue the units fill, the rows its job builds from it, and a
-- search's statements, each in the request's read-only snapshot. The search's own run planned with their arguments
-- (postgres.Planned): the keys and the notebooks decide the plan.

-- name: QueueSearch :exec
-- The nodes a unit, a notebook's deletion or nervewiki reindex changed (M8/P5 design 3.4), in ascending id order
-- (v0.1 design 13.1, item 5): queued for the search's job, a node already queued queued again with a new version,
-- so the job's take of an older read misses it; exact unless every queueing of it is bulk.
INSERT INTO page_search_queue (node_id, notebook_id, bulk)
SELECT unnest(sqlc.arg(ids)::uuid[]), sqlc.arg(notebook_id)::uuid, sqlc.arg(bulk)::boolean
ORDER BY 1
ON CONFLICT (node_id) DO UPDATE SET version = excluded.version, bulk = page_search_queue.bulk AND excluded.bulk;

-- name: QueueSearchOfNotebooks :exec
-- The nodes of notebooks being deleted that the deletion deletes, those deleted at its time (M8/P5 design 3.4): queued
-- exact, by id, as QueueSearch; the job then deletes their rows. Those in the trash before are not.
INSERT INTO page_search_queue (node_id, notebook_id, bulk)
SELECT id, notebook_id, false FROM nodes
WHERE notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]) AND deleted_at = sqlc.arg(deleted_at)::timestamptz
ORDER BY id
ON CONFLICT (node_id) DO UPDATE SET version = excluded.version, bulk = page_search_queue.bulk AND excluded.bulk;

-- name: StaleSearchNodes :many
-- The live nodes of a notebook after the id after, by id, at most max_rows, whose row is missing or of another
-- tokenizer than the current: those nervewiki reindex queues (M8/P5 design 3.4).
SELECT n.id FROM nodes n LEFT JOIN page_search s ON s.node_id = n.id
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL AND n.id > sqlc.arg(after)::uuid
    AND (s.node_id IS NULL OR s.tokenizer <> sqlc.arg(tokenizer)::text)
ORDER BY n.id
LIMIT sqlc.arg(max_rows);

-- name: TrySearchLock :one
-- The search's job's lock (M8/P5 design 3.4), on its session, without waiting: one run at a time, whichever process
-- runs it. The key is the page module's search job's alone.
SELECT pg_try_advisory_lock(6107471706471486842);

-- name: ReleaseSearchLock :exec
-- The search's job's lock given back.
SELECT pg_advisory_unlock(6107471706471486842);

-- name: SetSearchWriting :exec
-- The job's writes' settings, for its transaction alone (M8/P5A, measured on 300,000 pages): the GIN indexes' pending
-- list flushed in larger merges, each in memory, write a full index about 3.6 times as fast as at the server's
-- defaults (4 MB and 4 MB); the reads then scan a pending list of 16 MB at most.
SELECT set_config('work_mem', '64MB', true), set_config('gin_pending_list_limit', '16MB', true);

-- name: QueuedSearch :many
-- The queue after the node after, by id, at most max_rows, with the versions queued: the job's pass, unlocked.
SELECT node_id, version FROM page_search_queue WHERE node_id > sqlc.arg(after)::uuid ORDER BY node_id LIMIT sqlc.arg(max_rows);

-- name: SearchSources :many
-- What the job decides on for queued nodes, by their keys: each node as it is, deleted or not there (gone), a page's
-- content's revision and size, and the node's row as it was built.
SELECT i.id::uuid AS id, n.kind, n.name, (n.id IS NULL OR n.deleted_at IS NOT NULL)::boolean AS gone, c.revision, c.byte_size,
    s.revision AS built_revision, s.name AS built_name, s.tokenizer AS built_tokenizer, s.built_at
FROM unnest(sqlc.arg(ids)::uuid[]) AS i(id)
LEFT JOIN nodes n ON n.id = i.id
LEFT JOIN page_contents c ON c.node_id = i.id
LEFT JOIN page_search s ON s.node_id = i.id
ORDER BY i.id;

-- name: SearchContents :many
-- The contents of live pages by their keys, with their revisions: those the job builds rows of.
SELECT node_id, content, revision FROM page_contents
WHERE node_id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL
ORDER BY node_id;

-- name: WriteSearchRow :batchexec
-- A node's row as the job built it: its lexemes as tsvectors, without positions.
INSERT INTO page_search (node_id, revision, name, tokenizer, built_at, title, body)
VALUES (sqlc.arg(node_id), sqlc.narg(revision), sqlc.arg(name), sqlc.arg(tokenizer), sqlc.arg(built_at)::timestamptz,
    array_to_tsvector(sqlc.arg(title)::text[]), array_to_tsvector(sqlc.narg(body)::text[]))
ON CONFLICT (node_id) DO UPDATE SET revision = excluded.revision, name = excluded.name, tokenizer = excluded.tokenizer,
    built_at = excluded.built_at, title = excluded.title, body = excluded.body;

-- name: DeleteSearchRows :exec
-- The rows of nodes deleted or gone.
DELETE FROM page_search WHERE node_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DequeueSearch :exec
-- The queued nodes the job read at these versions, taken off unless a unit holds or requeued one since: a skipped
-- row is read again by the next run. The job never waits.
DELETE FROM page_search_queue q
USING (SELECT unnest(sqlc.arg(ids)::uuid[]) AS node_id, unnest(sqlc.arg(versions)::bigint[]) AS version) r
WHERE q.node_id = r.node_id AND q.version = r.version
    AND q.node_id IN (SELECT x.node_id FROM page_search_queue x WHERE x.node_id = ANY(sqlc.arg(ids)::uuid[])
                      ORDER BY x.node_id FOR UPDATE SKIP LOCKED);

-- name: AnalyzeSearch :exec
-- The statistics of the rows the job writes: @@'s selectivity reads them.
ANALYZE page_search;

-- name: SearchTitles :many
-- The title tier of a search (M8/P5 design 3.5): the live nodes, pages and attachments, of the notebooks given whose
-- title holds the keys, or that wait in the queue as a user wrote them, below the cursor, newest first, whose name
-- holds every term, at most max_rows; then a page's content read by key for the answered rows alone, and the first
-- hit's place in it, its character from 1, with the excerpt around it and the content's characters: 0 and none when
-- the content holds no hit, or the node is an attachment. The candidates are joined to their nodes and contents with
-- the notebooks' condition alone, behind OFFSET 0, and the other checks run above it, on the joined rows. Without
-- statistics the planner sizes the notebooks' nodes by that one condition's default selectivity; with the deleted
-- check's or the patterns' multiplied in, it took a notebook's live nodes for one row and compared each with every
-- candidate. With statistics it joins from the smaller side, a few candidates by key or the notebooks' nodes.
SELECT w.id, w.notebook_id, w.kind, w.name, w.sort_at,
    coalesce(e.at, 0)::integer AS at,
    coalesce(CASE WHEN e.at > 0 THEN substr(b.doc, greatest(e.at - 41, 1), 162) END, '')::text AS excerpt,
    coalesce(length(b.doc), 0)::integer AS chars
FROM (
    SELECT j.id, j.notebook_id, j.kind, j.name, j.sort_at
    FROM (SELECT n.id, n.notebook_id, n.kind, n.name, n.deleted_at,
              greatest(n.updated_at, c.updated_at)::timestamptz AS sort_at
          FROM (SELECT s.node_id FROM page_search s WHERE s.title @@ sqlc.arg(keys)::text::tsquery
                UNION
                SELECT q.node_id FROM page_search_queue q
                WHERE NOT q.bulk AND q.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])) k
          JOIN nodes n ON n.id = k.node_id
          LEFT JOIN page_contents c ON c.node_id = n.id
          WHERE n.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])
          OFFSET 0) j
    WHERE j.deleted_at IS NULL AND (j.sort_at, j.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
        AND j.name ~ ALL(sqlc.arg(terms)::text[])
    ORDER BY j.sort_at DESC, j.id DESC
    LIMIT sqlc.arg(max_rows)
) w
LEFT JOIN LATERAL (SELECT c2.content || '' AS doc FROM page_contents c2 WHERE c2.node_id = w.id OFFSET 0) b ON true
LEFT JOIN LATERAL (SELECT min(nullif(regexp_instr(b.doc, p, 1, 1, 0, 'c', 1), 0)) AS at
                   FROM unnest(sqlc.arg(terms)::text[]) p WHERE b.doc IS NOT NULL OFFSET 0) e ON true
ORDER BY w.sort_at DESC, w.id DESC;

-- name: SearchHitCount :one
-- The body tier's candidates by their lexemes, counted: the pages whose body holds body_keys and whose title
-- titled_keys, or, without body keys, whose title holds titled_keys (page_search_hits, which reads them by the GIN
-- indexes alone). The statements that read them take the count as their LIMIT, in the same snapshot: the planner's
-- estimate of their number, which it joins them by.
SELECT count(*) FROM page_search_hits(sqlc.narg(body_keys)::text::tsquery, sqlc.narg(titled_keys)::text::tsquery);

-- name: SearchBodies :many
-- The body tier of a search (M8/P5 design 3.5): the live pages of the notebooks given whose lexemes hold the keys,
-- hits of them (SearchHitCount's, all of them), or that wait in the queue as a user wrote them, below the cursor,
-- newest first, whose name misses some term and holds each titles-only one; walk of them at most have their content
-- read, by key, and checked as they are now, until max_rows pass. The candidates are joined to their nodes and
-- contents with the notebooks' condition alone, behind OFFSET 0, as in SearchTitles; the other checks, the patterns
-- among them, run above it on the joined rows, never in a scan of nodes.
-- The sort carries no content; OFFSET 0 keeps each lateral that reads or checks it one subquery too, so the
-- concatenation and the check run once a row. The last row walked comes back passing or not, with walked: more than
-- walk is more to walk. A row that passes has its first hit's place and excerpt as SearchTitles'.
SELECT w.id, w.notebook_id, w.name, w.sort_at, w.seq::integer AS seq, w.walked::integer AS walked,
    coalesce(x.passed, false)::boolean AS passed, coalesce(e.at, 0)::integer AS at,
    coalesce(CASE WHEN e.at > 0 THEN substr(b.doc, greatest(e.at - 41, 1), 162) END, '')::text AS excerpt,
    coalesce(length(b.doc), 0)::integer AS chars
FROM (
    SELECT v.*, row_number() OVER (ORDER BY v.sort_at DESC, v.id DESC) AS seq, count(*) OVER () AS walked
    FROM (
        SELECT j.id, j.notebook_id, j.name, j.sort_at
        FROM (SELECT n.id, n.notebook_id, n.name, n.deleted_at, c.deleted_at AS content_deleted_at,
                  greatest(n.updated_at, c.updated_at)::timestamptz AS sort_at
              FROM ((SELECT h.node_id
                     FROM page_search_hits(sqlc.narg(body_keys)::text::tsquery, sqlc.narg(titled_keys)::text::tsquery) h (node_id)
                     LIMIT sqlc.arg(hits)::bigint)
                    UNION
                    SELECT q.node_id FROM page_search_queue q
                    WHERE NOT q.bulk AND q.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])) k
              JOIN nodes n ON n.id = k.node_id
              JOIN page_contents c ON c.node_id = n.id
              WHERE n.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])
              OFFSET 0) j
        WHERE j.deleted_at IS NULL AND j.content_deleted_at IS NULL
            AND (j.sort_at, j.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
            AND NOT j.name ~ ALL(sqlc.arg(terms)::text[]) AND j.name ~ ALL(sqlc.arg(titled)::text[])
        ORDER BY j.sort_at DESC, j.id DESC
        LIMIT sqlc.arg(walk)::integer + 1
    ) v
) w
LEFT JOIN LATERAL (SELECT c2.content || '' AS doc FROM page_contents c2
                   WHERE c2.node_id = w.id AND w.seq <= sqlc.arg(walk)::integer OFFSET 0) b ON true
LEFT JOIN LATERAL (SELECT NOT EXISTS (SELECT 1 FROM unnest(sqlc.arg(bodied)::text[]) p
                                      WHERE NOT (w.name ~ p OR b.doc ~ p)) AS passed
                   WHERE b.doc IS NOT NULL OFFSET 0) x ON true
LEFT JOIN LATERAL (SELECT min(nullif(regexp_instr(b.doc, p, 1, 1, 0, 'c', 1), 0)) AS at
                   FROM unnest(sqlc.arg(terms)::text[]) p WHERE x.passed) e ON true
WHERE x.passed OR w.seq = least(w.walked, sqlc.arg(walk)::integer)
ORDER BY w.sort_at DESC, w.id DESC
LIMIT sqlc.arg(max_rows);

-- name: SearchTitleCount :one
-- The first page's count of a search's title tier (M8/P5 design 3.5): its hits, their names checked, counted to cap,
-- and the pages among them. The candidates are joined to their nodes with the notebooks' condition alone, behind
-- OFFSET 0, as in SearchTitles.
SELECT count(*)::integer AS titles, (count(*) FILTER (WHERE a.kind = 'page'))::integer AS title_pages
FROM (SELECT j.kind
      FROM (SELECT n.kind, n.name, n.notebook_id, n.deleted_at
            FROM (SELECT s.node_id FROM page_search s WHERE s.title @@ sqlc.arg(keys)::text::tsquery
                  UNION
                  SELECT q.node_id FROM page_search_queue q
                  WHERE NOT q.bulk AND q.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])) k
            JOIN nodes n ON n.id = k.node_id
            WHERE n.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])
            OFFSET 0) j
      WHERE j.deleted_at IS NULL AND j.name ~ ALL(sqlc.arg(terms)::text[])
      LIMIT sqlc.arg(cap)::integer) a;

-- name: SearchBodyCount :one
-- The first page's count of a search's body tier (M8/P5 design 3.5): its candidates, SearchBodies' hits and queue,
-- live pages of the notebooks given, their names unchecked, counted to cap. The candidates are joined to their nodes
-- with the notebooks' condition alone, behind OFFSET 0, as in SearchBodies.
SELECT count(*)::integer AS bodies
FROM (SELECT 1
      FROM (SELECT n.kind, n.notebook_id, n.deleted_at
            FROM ((SELECT h.node_id
                   FROM page_search_hits(sqlc.narg(body_keys)::text::tsquery, sqlc.narg(titled_keys)::text::tsquery) h (node_id)
                   LIMIT sqlc.arg(hits)::bigint)
                  UNION
                  SELECT q.node_id FROM page_search_queue q
                  WHERE NOT q.bulk AND q.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])) k
            JOIN nodes n ON n.id = k.node_id
            WHERE n.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])
            OFFSET 0) j
      WHERE j.deleted_at IS NULL AND j.kind = 'page'
      LIMIT sqlc.arg(cap)::integer) a;

-- name: SearchBuilding :one
-- Whether the notebooks given hold nodes queued in bulk, which no search finds until the job builds their rows.
SELECT EXISTS (SELECT 1 FROM page_search_queue WHERE bulk AND notebook_id = ANY(sqlc.arg(notebooks)::uuid[]))::boolean;
