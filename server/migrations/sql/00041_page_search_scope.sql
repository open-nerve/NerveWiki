-- page_search's lexemes by notebook (M8 Codex review R1): each lexeme of a row is stored behind its notebook's prefix,
-- so that a key of the GIN indexes is one notebook's lexeme, and its posting list that notebook's rows alone. A search
-- reads the keys of the notebooks it searches, and no other notebook's rows are in a posting list it reads: what a
-- notebook its reader does not see holds changes neither what the search reads nor how long it takes. 00040's reads
-- took every notebook's posting list and kept the reader's notebooks after: 60,000 hidden pages of a word made an
-- empty search for it 30 ms, not 3.

-- +goose Up
-- page_search_tags: each notebook's prefix, given by the search's job the first time it writes a row of one of the
-- notebook's nodes, from the sequence (page_search_prefix). A derived table as page_search: no foreign key, no
-- deleted_at, no purger; a deleted notebook's prefix stays, a few bytes, and is never another's, so that no row left
-- behind is ever another notebook's.
CREATE SEQUENCE page_search_tags_seq;
CREATE TABLE page_search_tags (
    notebook_id uuid PRIMARY KEY,
    prefix text NOT NULL CONSTRAINT page_search_tags_prefix_key UNIQUE
);

-- page_search_prefix: the prefix of the number n, its digits in base 62 and '#'. No lexeme holds a '#'
-- (domain.SearchTokens keeps letters, digits and marks alone), so that a prefixed lexeme is one notebook's, whatever
-- the prefixes' lengths.
-- +goose StatementBegin
CREATE FUNCTION page_search_prefix(n bigint) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
BEGIN ATOMIC
    SELECT string_agg(substr('0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz', (n / p % 62)::integer + 1, 1), ''
                      ORDER BY p DESC) || '#'
    FROM unnest(ARRAY[1, 62, 3844, 238328, 14776336, 916132832]::bigint[]) p
    WHERE p = 1 OR n >= p;
END;
-- +goose StatementEnd

-- page_search_queries: the query of each notebook given that has a prefix, its prefix before each of the keys, the
-- keys ANDed, each quoted as a tsquery's operand ('' and \\ doubled); none for a notebook without one, which has no
-- row. NULL notebooks or keys give NULL, an empty list no query: a read without them finds nothing. Its COST says what
-- a call costs, a query built for each notebook (0.5 ms for one, 4 to 25 ms for 5,000): at the default 100, a plan
-- that called it on each row of a small page_search, an empty one after this migration among them, looked cheaper
-- than a second bitmap, and a search over 5,000 notebooks took 10 s, not 12 ms (M8 Codex review R1, fix round 2).
-- +goose StatementBegin
CREATE FUNCTION page_search_queries(notebooks uuid[], keys text[]) RETURNS tsquery[]
    LANGUAGE sql STABLE STRICT PARALLEL SAFE COST 10000
BEGIN ATOMIC
    SELECT coalesce(array_agg(q.query), '{}')
    FROM (SELECT string_agg('''' || replace(replace(t.prefix || k.key, '\', '\\'), '''', '''''') || '''', ' & ')::tsquery
                     AS query
          FROM page_search_tags t CROSS JOIN unnest(keys) k (key)
          WHERE t.notebook_id = ANY(notebooks)
          GROUP BY t.prefix) q;
END;
-- +goose StatementEnd

-- Every row is built again with the prefixes: the rows go, and the live nodes are queued in bulk as 00040 queued them
-- (a node queued exact stays exact); a search says the index is being built until the job has written them. The
-- indexes keep no pending list (fastupdate off): every GIN scan reads the whole list, the latest rows of every
-- notebook, and a search scans the index once for each of its notebooks.
TRUNCATE page_search;
ALTER INDEX page_search_title_idx SET (fastupdate = off);
ALTER INDEX page_search_body_idx SET (fastupdate = off);
INSERT INTO page_search_queue (node_id, notebook_id, bulk)
SELECT id, notebook_id, true FROM nodes WHERE deleted_at IS NULL ORDER BY id
ON CONFLICT (node_id) DO UPDATE SET version = excluded.version, bulk = page_search_queue.bulk AND excluded.bulk;

-- The job's pass over the nodes queued in bulk goes notebook by notebook (QueuedSearchBulk): a notebook's rows written
-- together touch its keys alone in the GIN indexes, where rows in the order of their ids touch every notebook's, each
-- page of them again at each checkpoint (this migration's rebuild of 299,200 pages of 5,000 notebooks: 16 minutes and
-- 439 KB of WAL a page, against 29 minutes and 1.1 MB by id; R1 fix round 2, B-3); and a notebook says it is being
-- built until its own rows are.
CREATE INDEX page_search_queue_bulk_idx ON page_search_queue (notebook_id, node_id) WHERE bulk;

-- page_search_hits: the body tier's candidates in the notebooks given by their keys, 00040's three branches, each
-- condition an ANY over the notebooks' queries (page_search_queries). GIN takes no array: the executor scans the index
-- once for each notebook's query, into one bitmap, and no key it reads is another notebook's, whatever the plan. A
-- branch's arguments are checked before its scans, which a NULL list of keys would leave in a plan otherwise. The
-- scans' settings and the ROWS are 00040's, for its reasons.
--
-- Its statements are planned once for any arguments (plan_cache_mode), and without parallel workers. Planned for its
-- arguments, a statement builds every notebook's query to estimate its conditions (4 to 25 ms over 5,000 notebooks,
-- each time), and, an ANY of hundreds of queries estimated to hold most rows, may check the second branch's title or
-- body row by row, every notebook's query on every row: 的 数据库 over 200 notebooks took 3.5 s, not 10 ms (M8 Codex
-- review R1, 299,200 pages). Planned for any arguments, each branch reads the indexes' bitmaps, the second their
-- intersection (BitmapAnd), at any size of the table, page_search_queries priced as it costs. Such a plan takes
-- workers for any key, 2 ms a read to start them, which only a common key over hundreds of notebooks gives back, 3 ms
-- of 30. No JIT: at that price a plan's cost would compile its statements (some 30 ms a call), which the server's
-- connections never do (jit off) and no other's need.
DROP FUNCTION page_search_hits(tsquery, tsquery);
-- +goose StatementBegin
CREATE FUNCTION page_search_hits(notebooks uuid[], body_keys text[], title_keys text[]) RETURNS SETOF uuid
    LANGUAGE sql STABLE PARALLEL RESTRICTED ROWS 1000000
    SET enable_seqscan = off SET enable_indexscan = off SET enable_indexonlyscan = off SET work_mem = '64MB'
    SET plan_cache_mode = force_generic_plan SET max_parallel_workers_per_gather = 0 SET jit = off
BEGIN ATOMIC
    SELECT node_id FROM page_search WHERE title_keys IS NULL AND body_keys IS NOT NULL
        AND body @@ ANY (page_search_queries(notebooks, body_keys))
    UNION ALL
    SELECT node_id FROM page_search WHERE title_keys IS NOT NULL AND body_keys IS NOT NULL
        AND body @@ ANY (page_search_queries(notebooks, body_keys)) AND title @@ ANY (page_search_queries(notebooks, title_keys))
    UNION ALL
    SELECT node_id FROM page_search WHERE body_keys IS NULL AND title_keys IS NOT NULL
        AND title @@ ANY (page_search_queries(notebooks, title_keys));
END;
-- +goose StatementEnd

-- page_search_titles: the title tier's candidates in the notebooks given by their keys, read as page_search_hits reads
-- the body tier's, with its settings: the title statements no longer read the table themselves, which the planner could
-- scan whole.
-- +goose StatementBegin
CREATE FUNCTION page_search_titles(notebooks uuid[], keys text[]) RETURNS SETOF uuid
    LANGUAGE sql STABLE PARALLEL RESTRICTED ROWS 1000
    SET enable_seqscan = off SET enable_indexscan = off SET enable_indexonlyscan = off SET work_mem = '64MB'
    SET plan_cache_mode = force_generic_plan SET max_parallel_workers_per_gather = 0 SET jit = off
BEGIN ATOMIC
    SELECT node_id FROM page_search WHERE title @@ ANY (page_search_queries(notebooks, keys));
END;
-- +goose StatementEnd

-- +goose Down
DROP INDEX page_search_queue_bulk_idx;
DROP FUNCTION page_search_titles(uuid[], text[]);
DROP FUNCTION page_search_hits(uuid[], text[], text[]);
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
TRUNCATE page_search;
ALTER INDEX page_search_title_idx RESET (fastupdate);
ALTER INDEX page_search_body_idx RESET (fastupdate);
INSERT INTO page_search_queue (node_id, notebook_id, bulk)
SELECT id, notebook_id, true FROM nodes WHERE deleted_at IS NULL ORDER BY id
ON CONFLICT (node_id) DO UPDATE SET version = excluded.version, bulk = page_search_queue.bulk AND excluded.bulk;
DROP FUNCTION page_search_queries(uuid[], text[]);
DROP FUNCTION page_search_prefix(bigint);
DROP TABLE page_search_tags;
DROP SEQUENCE page_search_tags_seq;
