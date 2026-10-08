-- name: CreateBlob :exec
-- An attachment's row, in its node's unit: the node's time and uploader.
INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, width, height, created_by_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(node_id), sqlc.arg(notebook_id), sqlc.arg(mime), sqlc.arg(byte_size), sqlc.arg(sha256),
    sqlc.narg(width), sqlc.narg(height), sqlc.arg(created_by_id), sqlc.arg(created_at));

-- name: BlobOfNode :one
-- The row not deleted of an attachment's node.
SELECT id, node_id, notebook_id, mime, byte_size, sha256, width, height, created_by_id, created_at
FROM asset_blobs
WHERE node_id = sqlc.arg(node_id) AND deleted_at IS NULL;

-- name: BlobsOfNodes :many
-- The rows not deleted of the attachments' nodes.
SELECT id, node_id, notebook_id, mime, byte_size, sha256, width, height, created_by_id, created_at
FROM asset_blobs
WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]) AND deleted_at IS NULL;
