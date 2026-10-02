-- name: CreateContent :exec
INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
VALUES (sqlc.arg(node_id), sqlc.arg(content), sqlc.arg(revision), sqlc.arg(content_hash), sqlc.arg(byte_size),
    sqlc.arg(by), sqlc.arg(now));

-- name: ContentMeta :one
-- What a page tells of its content besides the content itself.
SELECT revision, byte_size, updated_by_id, updated_at FROM page_contents
WHERE node_id = sqlc.arg(node_id) AND deleted_at IS NULL;

-- name: PageContent :one
-- A page's content and its version, read in one statement for the reading
-- view: a deleted node's content went to the bin with it.
SELECT content, revision FROM page_contents
WHERE node_id = sqlc.arg(node_id) AND deleted_at IS NULL;
