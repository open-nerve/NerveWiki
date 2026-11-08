-- name: UpgradeProtected :many
-- Those of node_ids that have a row of the upgrade upgrade_id (M11 design 4.6): a delete's guardrail, by the primary
-- key, in the unit's transaction under the notebook's row lock.
SELECT node_id FROM upgrade_nodes
WHERE upgrade_id = sqlc.arg(upgrade_id) AND node_id = ANY(sqlc.arg(node_ids)::uuid[]);

-- name: DeleteUpgradeNodesOfNotebooks :exec
-- The notebooks' deletion (M11/P1 design 3.12): their upgrades' baseline and protected rows go, by
-- upgrade_nodes_notebook_id_idx. The rows have no deleted_at: an upgrade's ending deletes them too.
DELETE FROM upgrade_nodes WHERE notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]);

-- name: CaptureBaseline :one
-- An upgrade's baseline (M11 design 4.4 step 11): a row for each node alive in the notebook, under the notebook's row
-- lock; a page's latest version, its hash also the one the guardrails accept, its wanted count 0 until step 13.
WITH captured AS (
    INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
        content_hash, byte_size, wanted, accepted_hash)
    SELECT sqlc.arg(upgrade_id), n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision,
        c.content_hash, c.byte_size, CASE WHEN n.kind = 'page' THEN 0 END, c.content_hash
    FROM nodes n
    LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL
    WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL
    RETURNING kind
)
SELECT count(*)::integer AS nodes, (count(*) FILTER (WHERE kind = 'page'))::integer AS pages FROM captured;

-- name: ProtectSourceRoot :exec
-- The source root an upgrade's start created (M11 design 4.4 step 12): kept from a delete, not of the baseline.
INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind)
VALUES (sqlc.arg(upgrade_id), sqlc.arg(node_id), sqlc.arg(notebook_id), false, 'page');

-- name: SetBaselineWanted :exec
-- The distinct wanted targets each baseline page links to as the upgrade starts (M11 design 4.4 step 13), one
-- unnest a list in the select list as SetSourceHashes (sources.sql:46-54): sqlc parses no multi-argument unnest in FROM.
UPDATE upgrade_nodes u SET wanted = w.wanted
FROM (SELECT unnest(sqlc.arg(node_ids)::uuid[]) AS node_id, unnest(sqlc.arg(wanted)::integer[]) AS wanted) w
WHERE u.upgrade_id = sqlc.arg(upgrade_id) AND u.node_id = w.node_id AND u.baseline AND u.kind = 'page';

-- name: AnalyzeUpgradeNodes :exec
-- After an upgrade's start commits (M11 design 4.4 step 16): its baseline, up to 300,000 rows written at once, is
-- read at once by the guardrails and the gates.
ANALYZE upgrade_nodes;

-- name: RootNodesByKey :many
-- The nodes alive at the notebook's root whose title keys are keys, by nodes_notebook_id_name_key_root_key.
SELECT id, kind, name_key FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id IS NULL AND deleted_at IS NULL
    AND name_key = ANY(sqlc.arg(keys)::text[]);
