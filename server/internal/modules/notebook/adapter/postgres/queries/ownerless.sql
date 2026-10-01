-- name: ListOwnerless :many
-- The ownerless notebooks not deleted of the workspace, the earliest to become so first, then by id; with the
-- count of active members.
SELECT n.id, n.workspace_id, n.name, n.workspace_access, n.created_at, n.updated_at, n.ownerless_since, n.former_owner_id,
    (SELECT count(*) FROM notebook_members c
     WHERE c.notebook_id = n.id AND c.ended_at IS NULL AND c.deleted_at IS NULL) AS member_count
FROM notebooks n
WHERE n.workspace_id = sqlc.arg(workspace_id) AND n.deleted_at IS NULL AND n.ownerless_since IS NOT NULL
ORDER BY n.ownerless_since, n.id;
