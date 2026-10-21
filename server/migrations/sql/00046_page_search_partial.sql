-- page_search.partial: whether the body's lexemes leave out text of the page's content (M9 design 4.2; M9/P4
-- design 3.3): a run of more than 64 ASCII letters and digits, a data: URL longer than 64 bytes, or what follows
-- MaxSearchLexemeBytes. Such a row's body holds the reserved key !cut behind its notebook's prefix too, so that grep
-- reads a notebook's partial rows by the GIN index as it reads its keys (page_search_hits), never another notebook's.
-- A name's run left out does not count: grep searches contents alone. An attachment's row, without a body, is never
-- partial.

-- +goose Up
ALTER TABLE page_search ADD COLUMN partial boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT page_search_partial_check CHECK (NOT partial OR body IS NOT NULL);

-- The live pages whose content the tokenizer may cut, queued in bulk for its version 5 (M9/P4 design 3.2.2): every
-- page SearchTokens marks partial meets one of the four conditions, so a page not queued has the lexemes of 4 under
-- 5, and its row differs only by its tokenizer, which nervewiki reindex rewrites whenever it runs. The conditions:
--   1. the bound: a byte of name + "\n" + content costs at most 13 + 13/30 bytes of lexemes and prefixes (13 at
--      U+0344, U+0587, U+FB2C, U+FB2D; and Go's NFC writes a CGJ, a word character of 2 bytes, after 30 runes that
--      are no starters, which no fewer bytes give), so MaxSearchLexemeBytes (1,000,000) takes more than 74,441 bytes
--      of them, a name at most 255: a content of at most 74,000 bytes (BoundFreeContentBytes) never reaches it;
--   2. a run of more than 64 ASCII letters and digits, as written;
--   3. a data: URL of more than 64 bytes: its characters are ASCII, so 60 after "data:", in any case;
--   4. a run or a URL made long by folding: the eleven runes whose fold is ASCII letters alone (U+00DF, U+017F,
--      U+1E9E, U+212A, U+FB00-U+FB06) fold to three at most, so 22 of them and of ASCII letters and digits make 65.
-- PostgreSQL's bracket ranges here are of code points; a wider reading of any of them only queues more.
INSERT INTO page_search_queue (node_id, notebook_id, bulk)
SELECT n.id, n.notebook_id, true
FROM nodes n JOIN page_contents c ON c.node_id = n.id
WHERE n.deleted_at IS NULL AND n.kind = 'page' AND c.deleted_at IS NULL
    AND (c.byte_size > 74000
        OR c.content ~ '[0-9A-Za-z]{65}'
        OR c.content ~* 'data:[^\t\n\v\f\r )"''<>\]`]{60}'
        OR c.content ~ '[\u00DF\u017F\u1E9E\u212A\uFB00-\uFB06]'
            AND (c.content ~ '[0-9A-Za-z\u00DF\u017F\u1E9E\u212A\uFB00-\uFB06]{22}' OR c.content ~* 'data:'))
ORDER BY n.id
ON CONFLICT (node_id) DO UPDATE SET version = excluded.version, bulk = page_search_queue.bulk AND excluded.bulk;

-- +goose Down
ALTER TABLE page_search DROP CONSTRAINT page_search_partial_check, DROP COLUMN partial;
