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

-- name: QueuedSearch :many
-- The nodes queued exact, or in bulk, after the node after, by id, at most max_rows, with the versions queued: the
-- job's pass, unlocked, the exact nodes' first.
SELECT node_id, version FROM page_search_queue
WHERE bulk = sqlc.arg(bulk)::boolean AND node_id > sqlc.arg(after)::uuid
ORDER BY node_id
LIMIT sqlc.arg(max_rows);

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

-- name: TagSearchNotebooks :exec
-- The notebooks of the nodes given that have no prefix yet get the next (M8 Codex review R1): the job's, before it
-- writes their rows, in its transaction. One number of the sequence a notebook, not a node: the prefixes stay as
-- short as the notebooks are few (without the DISTINCT, ON CONFLICT would keep the answers, a batch of 100 nodes of a
-- new notebook spending 99 numbers).
INSERT INTO page_search_tags (notebook_id, prefix)
SELECT x.notebook_id, page_search_prefix(nextval('page_search_tags_seq'))
FROM (SELECT DISTINCT n.notebook_id FROM nodes n
      WHERE n.id = ANY(sqlc.arg(ids)::uuid[])
          AND NOT EXISTS (SELECT 1 FROM page_search_tags t WHERE t.notebook_id = n.notebook_id)
      ORDER BY 1) x
ON CONFLICT (notebook_id) DO NOTHING;

-- name: WriteSearchRow :batchexec
-- A node's row as the job built it: its lexemes as tsvectors, without positions, each after its notebook's prefix
-- (M8 Codex review R1). A node purged since it was read has no notebook and gets no row.
INSERT INTO page_search (node_id, revision, name, tokenizer, built_at, title, body)
SELECT n.id, sqlc.narg(revision)::integer, sqlc.arg(name)::text, sqlc.arg(tokenizer)::text, sqlc.arg(built_at)::timestamptz,
    array_to_tsvector(ARRAY(SELECT t.prefix || l FROM unnest(sqlc.arg(title)::text[]) l)),
    CASE WHEN sqlc.narg(body)::text[] IS NOT NULL
        THEN array_to_tsvector(ARRAY(SELECT t.prefix || l FROM unnest(sqlc.narg(body)::text[]) l)) END
FROM nodes n JOIN page_search_tags t ON t.notebook_id = n.notebook_id
WHERE n.id = sqlc.arg(node_id)::uuid
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
-- The size of page_search as the job writes it, which the candidate reads' plans are made for (planned for any
-- arguments, M8 Codex review R1): a change of it makes them again. Their @@ reads no statistics of the lexemes, an
-- ANY of queries no plan knows.
ANALYZE page_search;

-- name: SearchNFC :many
-- Texts in NFC as the search's statements read a content in it, PostgreSQL's, in their order: the terms of a query
-- that Go's NFC changed, whose patterns of contents are built of these. Go's NFC can differ from PostgreSQL's: a CGJ in
-- a run of more than 30 non-starters, and their Unicode versions (M8 Codex review R2; fix check 1, C1-1).
SELECT normalize(x.t, NFC)::text AS nfc FROM unnest(sqlc.arg(texts)::text[]) WITH ORDINALITY AS x(t, i) ORDER BY x.i;

-- name: SearchTitles :many
-- The title tier of a search (M8/P5 design 3.5): the live nodes, pages and attachments, of the notebooks given whose
-- title holds the keys, or that wait in the queue as a user wrote them, below the cursor, newest first, whose name
-- holds every term, at most max_rows; then a page's content read by key for the answered rows alone, in NFC as the
-- patterns of contents are (in_contents), and the first hit's place in it, its character from 1, with the excerpt
-- around it and the content's characters: 0 and none when the content holds no hit, or the node is an attachment. The
-- names are matched by the terms' patterns of names (terms), Go's NFC as the names are, the contents by those of
-- contents, PostgreSQL's NFC as the contents are read: the two differ for a run of more than 30 non-starters, which
-- Go's NFC parts with a CGJ, and where their Unicode versions differ (M8 Codex review R2). A content not in NFC is put
-- in it for the read alone, as the job's lexemes are of it (M8/P5A review A-M4); one in NFC, the common case, costs the
-- check alone, some 20 µs a content of 4 KB, five times the read's (measured on the guard's corpus). The candidates are
-- joined to their nodes and contents with the notebooks' condition alone, behind OFFSET 0, and the other checks run
-- above it, on the joined rows. Without statistics the planner sizes the notebooks' nodes by that one condition's
-- default selectivity; with the deleted check's or the patterns' multiplied in, it took a notebook's live nodes for one
-- row and compared each with every candidate. With statistics it joins from the smaller side, a few candidates by key
-- or the notebooks' nodes.
SELECT w.id, w.notebook_id, w.kind, w.name, w.sort_at,
    coalesce(e.at, 0)::integer AS at,
    coalesce(CASE WHEN e.at > 0 THEN substr(b.doc, greatest(e.at - 41, 1), 162) END, '')::text AS excerpt,
    coalesce(length(b.doc), 0)::integer AS chars
FROM (
    SELECT j.id, j.notebook_id, j.kind, j.name, j.sort_at
    FROM (SELECT n.id, n.notebook_id, n.kind, n.name, n.deleted_at,
              greatest(n.updated_at, c.updated_at)::timestamptz AS sort_at
          FROM (SELECT s.node_id FROM page_search_titles(sqlc.arg(notebooks)::uuid[], sqlc.arg(keys)::text[]) s (node_id)
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
LEFT JOIN LATERAL (SELECT CASE WHEN r.raw IS NFC NORMALIZED THEN r.raw ELSE normalize(r.raw, NFC) END AS doc
                   FROM (SELECT c2.content || '' AS raw FROM page_contents c2
                         WHERE c2.node_id = w.id OFFSET 0) r OFFSET 0) b ON true
LEFT JOIN LATERAL (SELECT min(nullif(regexp_instr(b.doc, p, 1, 1, 0, 'c', 1), 0)) AS at
                   FROM unnest(sqlc.arg(in_contents)::text[]) p WHERE b.doc IS NOT NULL OFFSET 0) e ON true
ORDER BY w.sort_at DESC, w.id DESC;

-- name: SearchHitCount :one
-- The body tier's candidates by their lexemes, counted: the pages whose body holds body_keys and whose title
-- titled_keys, or, without body keys, whose title holds titled_keys (page_search_hits, which reads them by the GIN
-- indexes alone). The statements that read them take the count as their LIMIT, in the same snapshot: the planner's
-- estimate of their number, which it joins them by.
SELECT count(*) FROM page_search_hits(sqlc.arg(notebooks)::uuid[], sqlc.narg(body_keys)::text[], sqlc.narg(titled_keys)::text[]);

-- name: SearchBodies :many
-- The body tier of a search (M8/P5 design 3.5): the live pages of the notebooks given whose lexemes hold the keys,
-- hits of them (SearchHitCount's, all of them), or that wait in the queue as a user wrote them, below the cursor,
-- newest first, whose name misses some term and holds each titles-only one; walk of them at most have their content
-- read, by key, in NFC as SearchTitles reads it, and checked as they are now, until max_rows pass: each term found in
-- contents too in the name by its pattern of names (bodied) or in the content by its pattern of contents
-- (bodied_in_contents), as SearchTitles matches each. The candidates are
-- joined to their nodes and contents with the notebooks' condition alone, behind OFFSET 0, as in SearchTitles; the
-- other checks, the patterns among them, run above it on the joined rows, never in a scan of nodes.
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
                     FROM page_search_hits(sqlc.arg(notebooks)::uuid[], sqlc.narg(body_keys)::text[], sqlc.narg(titled_keys)::text[]) h (node_id)
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
LEFT JOIN LATERAL (SELECT CASE WHEN r.raw IS NFC NORMALIZED THEN r.raw ELSE normalize(r.raw, NFC) END AS doc
                   FROM (SELECT c2.content || '' AS raw FROM page_contents c2
                         WHERE c2.node_id = w.id AND w.seq <= sqlc.arg(walk)::integer OFFSET 0) r OFFSET 0) b ON true
LEFT JOIN LATERAL (SELECT NOT EXISTS (SELECT 1 FROM generate_subscripts(sqlc.arg(bodied)::text[], 1) i
                                      WHERE NOT (w.name ~ (sqlc.arg(bodied)::text[])[i]
                                                 OR b.doc ~ (sqlc.arg(bodied_in_contents)::text[])[i])) AS passed
                   WHERE b.doc IS NOT NULL OFFSET 0) x ON true
LEFT JOIN LATERAL (SELECT min(nullif(regexp_instr(b.doc, p, 1, 1, 0, 'c', 1), 0)) AS at
                   FROM unnest(sqlc.arg(in_contents)::text[]) p WHERE x.passed) e ON true
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
            FROM (SELECT s.node_id FROM page_search_titles(sqlc.arg(notebooks)::uuid[], sqlc.arg(keys)::text[]) s (node_id)
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
                   FROM page_search_hits(sqlc.arg(notebooks)::uuid[], sqlc.narg(body_keys)::text[], sqlc.narg(titled_keys)::text[]) h (node_id)
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
