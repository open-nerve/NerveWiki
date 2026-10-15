-- page_search: a node's search lexemes (M8/P5 design 3.2), computed by the app (domain.SearchTokens) from the node's
-- name and a page's content as the search's job last read them. A derived table as linking's are: no foreign key, no
-- deleted_at, no purger (v0.1 design 13.1, item 6). The job keeps a row for each live node and removes a node's row
-- once the node is deleted or gone; reads join the live nodes, so a stale row is never an answer.

-- +goose Up
CREATE TABLE page_search (
    node_id uuid PRIMARY KEY,
    -- The page's content revision the body was built from; null for an attachment, which has no body.
    revision integer CONSTRAINT page_search_revision_check CHECK (revision >= 1),
    -- The name the lexemes were built from.
    name text NOT NULL,
    -- The tokenizer the lexemes are of (domain.SearchTokenizer: its own version and Go's Unicode version): a row of
    -- another is rebuilt once nervewiki reindex queues it, as linking's extractor.
    tokenizer text NOT NULL,
    -- When the job last built the row: it builds it again no sooner than domain.MinSearchRebuild after.
    built_at timestamptz NOT NULL,
    -- The name's: CJK unigrams and bigrams, the word runs' trigrams, the short words.
    title tsvector NOT NULL,
    -- A page's name and content's: CJK bigrams, the word runs' trigrams, the short words.
    body tsvector,
    CONSTRAINT page_search_body_check CHECK ((revision IS NULL) = (body IS NULL))
);
CREATE INDEX page_search_title_idx ON page_search USING gin (title);
CREATE INDEX page_search_body_idx ON page_search USING gin (body);

-- page_search_hits: the body tier's candidates by their lexemes (M8/P5 design 3.5), the pages whose body holds
-- body_keys and whose title title_keys, or, without body_keys, whose title holds title_keys: read through the GIN
-- indexes alone (M8/P5A, measured on 300,000 pages). Any other read of the table, a sequential scan or a walk of its
-- key, reads every row's body, out of line in TOAST (3.7 GB), which the planner does not count: once the index's
-- pending list or a rebuild's dead entries raised its cost a little, it took the scan for a common key, a search
-- 10-28 s. With the scans of the table disabled here, bitmaps of its indexes are its only reads left, and work_mem
-- keeps their heap pages exact: past it a bitmap turns pages lossy, whose every row's body the recheck reads again, a
-- common key's past about 2M rows at the default 4 MB (some 64 kB of pages a MB, 30 rows a page); at 64 MB, past
-- about 30M. The bitmap takes the memory as it grows, 0.6 MB at 300,000 pages (M8/P5A review C-M3). The settings end
-- with the function, the statement around it planned as it would be. A statement reads it with the count of its rows
-- as its LIMIT (SearchHitCount, in the same snapshot), which is its estimate: ROWS is above any count, and the LIMIT
-- never cuts. PARALLEL RESTRICTED: a worker cannot take its settings.
-- +goose StatementBegin
CREATE FUNCTION page_search_hits(body_keys tsquery, title_keys tsquery) RETURNS SETOF uuid
    LANGUAGE sql STABLE PARALLEL RESTRICTED ROWS 1000000
    SET enable_seqscan = off SET enable_indexscan = off SET enable_indexonlyscan = off SET work_mem = '64MB'
BEGIN ATOMIC
    SELECT node_id FROM page_search WHERE title_keys IS NULL AND body @@ body_keys
    UNION ALL
    SELECT node_id FROM page_search WHERE title_keys IS NOT NULL AND body @@ body_keys AND title @@ title_keys
    UNION ALL
    SELECT node_id FROM page_search WHERE body_keys IS NULL AND title @@ title_keys;
END;
-- +goose StatementEnd

-- The nodes whose row may be stale (M8/P5 design 3.4): a unit queues each node it creates, restores, renames, deletes
-- or writes the content of, in its transaction; the job takes them off. Each queueing takes a new version from the
-- sequence, so the job takes a node off only as it read it. A bulk node (an import's, the backfill's, nervewiki
-- reindex's) is no candidate of a search, which says the index is being built; a user's write makes it exact.
CREATE SEQUENCE page_search_queue_version_seq;
CREATE TABLE page_search_queue (
    node_id uuid PRIMARY KEY,
    -- The node's notebook, which never changes: a search reads its notebooks' queue only.
    notebook_id uuid NOT NULL,
    version bigint NOT NULL DEFAULT nextval('page_search_queue_version_seq'),
    bulk boolean NOT NULL
);
CREATE INDEX page_search_queue_notebook_id_bulk_idx ON page_search_queue (notebook_id, bulk);
-- The live nodes so far, in bulk: the job builds their rows once the server runs.
INSERT INTO page_search_queue (node_id, notebook_id, bulk)
SELECT id, notebook_id, true FROM nodes WHERE deleted_at IS NULL ORDER BY id;

-- +goose Down
DROP FUNCTION page_search_hits(tsquery, tsquery);
DROP TABLE page_search_queue, page_search;
DROP SEQUENCE page_search_queue_version_seq;
