-- name: CreateContent :exec
INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
VALUES (sqlc.arg(node_id), sqlc.arg(content), sqlc.arg(revision), sqlc.arg(content_hash), sqlc.arg(byte_size),
    sqlc.arg(by), sqlc.arg(now));

-- name: ContentMeta :one
-- What a page tells of its content besides the content itself.
SELECT revision, byte_size, updated_by_id, updated_at FROM page_contents
WHERE node_id = sqlc.arg(node_id) AND deleted_at IS NULL;

-- name: PageContent :one
-- A page's content, its version and its hash, read in one statement: a deleted node's content went to the bin
-- with it.
SELECT content, revision, content_hash FROM page_contents
WHERE node_id = sqlc.arg(node_id) AND deleted_at IS NULL;

-- name: LockContent :one
-- The content row of a page not deleted of the notebook, FOR NO KEY UPDATE: the page's gate (M4 design 4,
-- "row locks within a page"). A content write compares its base_revision under it, and an edit session opens
-- under it; the node's row is not locked: the notebook's row keeps the tree still.
SELECT c.revision, c.content_hash, c.byte_size FROM page_contents c
JOIN nodes n ON n.id = c.node_id
WHERE c.node_id = sqlc.arg(node_id) AND n.notebook_id = sqlc.arg(notebook_id) AND n.kind = 'page'
    AND n.deleted_at IS NULL AND c.deleted_at IS NULL
FOR NO KEY UPDATE OF c;

-- name: WriteContent :exec
-- A page's content as a write leaves it, under LockContent's lock.
UPDATE page_contents
SET content = sqlc.arg(content), revision = sqlc.arg(revision), content_hash = sqlc.arg(content_hash),
    byte_size = sqlc.arg(byte_size), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE node_id = sqlc.arg(node_id) AND deleted_at IS NULL;

-- name: ContentSizes :many
-- The size and the time of the last write of each content not deleted of the pages: an export tells the pages
-- without content (M7/P5 design 3.8). The array grows with the notebook: the store plans it with its arguments.
SELECT node_id, byte_size, updated_at FROM page_contents
WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]) AND deleted_at IS NULL;

-- name: PageStates :many
-- The revision, size, last write and hash of each content not deleted of the pages (M9/P1 design 3.10.3): list's
-- pages, and a batch's (M9/P3 design 3.4.4), which tells a write of the bytes a page holds by its size and hash.
-- The array is a page of list, at most 1,000: the store plans it with its argument.
SELECT node_id, revision, byte_size, updated_at, content_hash FROM page_contents
WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]) AND deleted_at IS NULL;

-- name: Contents :many
-- The contents not deleted of the pages: an export's batch.
SELECT node_id, content FROM page_contents
WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]) AND deleted_at IS NULL;

-- name: GrepContents :many
-- The contents not deleted of the pages, in the order of node_ids, while their sizes sum to max_bytes at most; the first
-- page past it with its size alone, and no page after it: grep's batch (M9/P4 design 3.4.3), its bound held on what
-- the statement reads, not on the sizes read before it (M9 Codex review D2). The rows are read once, by their key, and
-- only the contents within the bound are returned: the content of one past it, a TOAST pointer in its row while the
-- window and the CASE carry it, is never fetched, as TestTheContentsPastTheBoundAreNotFetched checks.
SELECT s.node_id, s.byte_size, (s.total <= sqlc.arg(max_bytes)::bigint)::boolean AS within,
    coalesce(CASE WHEN s.total <= sqlc.arg(max_bytes)::bigint THEN s.content END, '')::text AS content
FROM (
    SELECT c.node_id, c.byte_size, c.content, sum(c.byte_size) OVER (ORDER BY a.ord) AS total
    FROM unnest(sqlc.arg(node_ids)::uuid[]) WITH ORDINALITY AS a(id, ord)
    JOIN page_contents c ON c.node_id = a.id AND c.deleted_at IS NULL
) s
WHERE s.total - s.byte_size <= sqlc.arg(max_bytes)::bigint;

-- name: PageContentWithin :one
-- A page's content, its version and its hash, as PageContent reads them, while its size is max_bytes at most; past it,
-- its size alone, the content never read (M9 Codex review D2): a batch's plan reads the contents whose sizes it bounded
-- first, which may have grown since. As GrepContents, the row is read once.
SELECT c.byte_size, c.revision, c.content_hash, (c.byte_size <= sqlc.arg(max_bytes)::integer)::boolean AS within,
    coalesce(CASE WHEN c.byte_size <= sqlc.arg(max_bytes)::integer THEN c.content END, '')::text AS content
FROM page_contents c
WHERE c.node_id = sqlc.arg(node_id) AND c.deleted_at IS NULL;
