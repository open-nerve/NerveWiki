-- name: UpgradeProtected :many
-- Those of node_ids that have a row of the upgrade upgrade_id (M11 design 4.6): a delete's guardrail, by the primary
-- key, in the unit's transaction under the notebook's row lock.
SELECT node_id FROM upgrade_nodes
WHERE upgrade_id = sqlc.arg(upgrade_id) AND node_id = ANY(sqlc.arg(node_ids)::uuid[]);

-- name: DeleteUpgradeNodesOfNotebooks :exec
-- The notebooks' deletion (M11/P1 design 3.12): their upgrades' baseline and protected rows go, by
-- upgrade_nodes_notebook_id_idx. The rows have no deleted_at: an upgrade's ending deletes them too.
DELETE FROM upgrade_nodes WHERE notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]);
