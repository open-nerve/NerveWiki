-- name: LockHoldings :many
-- The notebooks not deleted of the workspaces that the account is an active member of, locked FOR NO KEY UPDATE
-- by id: the end of its workspace memberships takes them under the workspaces' rows (M3 design 4). With its
-- role in each, the active admins and the active members; the workspaces' locks keep every other notebook write
-- out, so the counts hold until the transaction ends.
WITH locked AS (
    SELECT l.id, l.workspace_id FROM notebooks l
    WHERE l.workspace_id = ANY(sqlc.arg(workspace_ids)::uuid[]) AND l.deleted_at IS NULL
        AND EXISTS (SELECT 1 FROM notebook_members e WHERE e.notebook_id = l.id AND e.user_id = sqlc.arg(user_id)
            AND e.ended_at IS NULL AND e.deleted_at IS NULL)
    ORDER BY l.id
    FOR NO KEY UPDATE
)
SELECT locked.id, locked.workspace_id, m.role,
    (SELECT count(*) FROM notebook_members a WHERE a.notebook_id = locked.id AND a.role = 'admin'
        AND a.ended_at IS NULL AND a.deleted_at IS NULL) AS admins,
    (SELECT count(*) FROM notebook_members c WHERE c.notebook_id = locked.id
        AND c.ended_at IS NULL AND c.deleted_at IS NULL) AS members
FROM locked
JOIN notebook_members m ON m.notebook_id = locked.id AND m.user_id = sqlc.arg(user_id) AND m.deleted_at IS NULL
ORDER BY locked.id;

-- name: EndMembershipsOf :exec
-- The account's active memberships of the notebooks, ended at the workspace membership's end, by its ender.
UPDATE notebook_members
SET ended_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE user_id = sqlc.arg(user_id) AND notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[])
    AND ended_at IS NULL AND deleted_at IS NULL;

-- name: SetOwnerless :exec
-- The notebooks ownerless since the time, the account their former owner. updated_at stays: it is a notebook's
-- last activity, which the ownerless list shows (M3 design 4), and the owner leaving is none.
UPDATE notebooks
SET ownerless_since = sqlc.arg(since)::timestamptz, former_owner_id = sqlc.arg(former_owner_id)
WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: LockOwnerlessOf :many
-- The ownerless notebooks not deleted of the workspace whose former owner the account is, locked FOR NO KEY
-- UPDATE by id: its restore returns them (M3 design 4).
SELECT id, workspace_id, name, workspace_access, created_at, updated_at
FROM notebooks
WHERE workspace_id = sqlc.arg(workspace_id) AND former_owner_id = sqlc.arg(former_owner_id) AND deleted_at IS NULL
ORDER BY id
FOR NO KEY UPDATE;

-- name: RestoreAdmins :execrows
-- The account's ended memberships of the notebooks active again as their admin; created_at stays.
UPDATE notebook_members
SET role = 'admin', ended_at = NULL, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE user_id = sqlc.arg(user_id) AND notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[])
    AND ended_at IS NOT NULL AND deleted_at IS NULL;

-- name: ClearOwnerless :exec
-- The notebooks owned again; updated_at stays, as SetOwnerless's.
UPDATE notebooks
SET ownerless_since = NULL, former_owner_id = NULL
WHERE id = ANY(sqlc.arg(ids)::uuid[]);
