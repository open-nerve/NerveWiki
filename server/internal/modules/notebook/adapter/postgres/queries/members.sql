-- name: AddMember :exec
INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(notebook_id), sqlc.arg(user_id), sqlc.arg(role), sqlc.arg(by), sqlc.arg(by),
    sqlc.arg(now), sqlc.arg(now));

-- name: CountMembers :one
-- The notebook's active members.
SELECT count(*) FROM notebook_members
WHERE notebook_id = sqlc.arg(notebook_id) AND ended_at IS NULL AND deleted_at IS NULL;

-- name: DeleteMembersOf :exec
-- Every member row of the notebook not deleted, ended ones included, with the notebook's time: its deletion.
UPDATE notebook_members
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE notebook_id = sqlc.arg(notebook_id) AND deleted_at IS NULL;

-- name: DeleteMembersOfNotebooks :exec
-- DeleteMembersOf for the notebooks a workspace's deletion deleted.
UPDATE notebook_members
SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]) AND deleted_at IS NULL;
