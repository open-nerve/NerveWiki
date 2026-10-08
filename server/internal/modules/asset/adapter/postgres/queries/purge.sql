-- The purge of the attachments' rows (M7/P2 design 3.8): a batch's rows are locked, their files deleted, then the
-- rows, in one transaction.

-- name: ExpiredBlobs :many
-- Up to batch rows deleted before before, the oldest deletions first, locked; the rows another transaction holds
-- are skipped.
SELECT id FROM asset_blobs
WHERE deleted_at < sqlc.arg(before)::timestamptz
ORDER BY deleted_at
LIMIT sqlc.arg(batch)
FOR UPDATE SKIP LOCKED;

-- name: DeleteBlobs :execrows
DELETE FROM asset_blobs WHERE id = ANY(sqlc.arg(ids)::uuid[]);
