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

-- name: ListMembers :many
-- The notebook's active members, by when they first joined, then by id.
SELECT id, notebook_id, user_id, role, ended_at, created_at FROM notebook_members
WHERE notebook_id = sqlc.arg(notebook_id) AND ended_at IS NULL AND deleted_at IS NULL
ORDER BY created_at, id;

-- name: FindActiveMember :one
-- The active membership with the id. A notebook's deletion deletes its member rows: the notebook is not deleted.
SELECT id, notebook_id, user_id, role, ended_at, created_at FROM notebook_members
WHERE id = sqlc.arg(id) AND ended_at IS NULL AND deleted_at IS NULL;

-- name: FindMemberOf :one
-- The account's membership of the notebook, active or ended: one row per pair (notebook_members_notebook_id_user_id_key).
SELECT id, notebook_id, user_id, role, ended_at, created_at FROM notebook_members
WHERE notebook_id = sqlc.arg(notebook_id) AND user_id = sqlc.arg(user_id) AND deleted_at IS NULL;

-- name: CountAdmins :one
-- The notebook's active admins, which rule one counts under the notebook's lock.
SELECT count(*) FROM notebook_members
WHERE notebook_id = sqlc.arg(notebook_id) AND role = 'admin' AND ended_at IS NULL AND deleted_at IS NULL;

-- name: UpdateMemberRole :exec
UPDATE notebook_members
SET role = sqlc.arg(role), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: EndMember :exec
UPDATE notebook_members
SET ended_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: RestoreMember :exec
-- An ended membership active again with the role. created_at stays: when the account first joined.
UPDATE notebook_members
SET role = sqlc.arg(role), ended_at = NULL, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);
