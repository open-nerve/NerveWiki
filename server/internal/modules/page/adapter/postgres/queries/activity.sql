-- name: NotebookActivities :many
-- The pages' part in their notebooks' activity (M3 design 4; M4/P4 design 3.10): the bytes of the pages not
-- deleted, and the latest updated_at of the changesets, which an edit session's later writes move on. A
-- notebook without a changeset has never had a page: it is not in the answer.
SELECT cs.notebook_id, max(cs.updated_at)::timestamptz AS last_write_at,
    (SELECT coalesce(sum(c.byte_size), 0) FROM nodes n JOIN page_contents c ON c.node_id = n.id
        WHERE n.notebook_id = cs.notebook_id AND n.deleted_at IS NULL AND c.deleted_at IS NULL)::bigint AS bytes
FROM changesets cs
WHERE cs.notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]) AND cs.deleted_at IS NULL
GROUP BY cs.notebook_id;
