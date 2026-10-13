-- name: AttachmentInfos :many
-- The type and size of the attachments of nodes, deleted or not (M8/P2 design 3.4): the page module's trash shows an
-- attachment's in the trash. One row a node, by its unique key.
SELECT node_id, mime, byte_size FROM asset_blobs WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]);
