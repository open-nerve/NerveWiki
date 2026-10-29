-- name: AttachmentInfos :many
-- The type, size and SHA256 of attachments, deleted or not: trash displays their metadata and source restore
-- needs their stored hash (M8/P2 design 3.4; M10/P2 design 3.9). One row a node, by its unique key.
SELECT node_id, mime, byte_size, sha256 FROM asset_blobs WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]);
