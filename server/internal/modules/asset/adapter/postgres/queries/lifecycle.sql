-- The attachments' rows follow their nodes and notebooks (M7/P2 design 3.8), in the transaction of the unit or of
-- the deletion that deleted them, at its time.

-- name: DeleteBlobsOfNodes :exec
UPDATE asset_blobs SET deleted_at = sqlc.arg(at)::timestamptz
WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]) AND deleted_at IS NULL;

-- name: DeleteBlobsOfNotebooks :exec
UPDATE asset_blobs SET deleted_at = sqlc.arg(at)::timestamptz
WHERE notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]) AND deleted_at IS NULL;

-- name: NotebookActivities :many
-- The attachments' part in notebooks' activity (M3 handoff 1): the bytes of the rows not deleted, and the latest
-- upload.
SELECT notebook_id, sum(byte_size)::bigint AS bytes, max(created_at)::timestamptz AS last_upload_at
FROM asset_blobs
WHERE notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]) AND deleted_at IS NULL
GROUP BY notebook_id;

-- name: KnownBlobs :many
-- The ids among ids that have a row, deleted or not: the orphan sweep keeps their files.
SELECT id FROM asset_blobs WHERE id = ANY(sqlc.arg(ids)::uuid[]);
