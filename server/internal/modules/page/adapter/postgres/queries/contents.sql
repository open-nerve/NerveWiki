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

-- name: Contents :many
-- The contents not deleted of the pages: an export's batch.
SELECT node_id, content FROM page_contents
WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]) AND deleted_at IS NULL;
