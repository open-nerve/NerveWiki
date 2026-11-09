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
-- lock; a page's latest version, its hash also the one the guardrails accept, its wanted count 0 until step 13. The
-- notebook's nodes drive page_contents' primary key, one node at a time, so that the start reads this notebook's
-- rows and not the instance's (decision 240): the LIMIT 1 keeps the LATERAL from being pulled up into a join, which
-- may hash the whole table, and changes nothing, a node having one row of page_contents at most.
WITH captured AS (
    INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
        content_hash, byte_size, wanted, accepted_hash)
    SELECT sqlc.arg(upgrade_id), n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision,
        c.content_hash, c.byte_size, CASE WHEN n.kind = 'page' THEN 0 END, c.content_hash
    FROM nodes n
    LEFT JOIN LATERAL (
        SELECT c.revision, c.content_hash, c.byte_size FROM page_contents c
        WHERE c.node_id = n.id AND c.deleted_at IS NULL
        LIMIT 1
    ) c ON true
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

-- name: RollbackFreeForest :many
-- The new nodes alive in the notebook that hold no baseline node, with their parents (M11 design 4.10): the
-- holders are the ancestors of the baseline nodes alive; a free node is a new one that is no holder. The roots of
-- the free subtrees are those whose parent is none, a holder or a baseline node. Depth is at most MaxDepth: the
-- holders climb as many levels. Each node is read by its key, each upgrade's row by upgrade_nodes_pkey, a row at a
-- time (the LATERALs' LIMIT 1 keeps them from being pulled up into joins, which may read a table whole), so that the
-- read costs as the notebook and not as the instance (decision 240; its cost guard). The holders are left out by an
-- anti-join, NOT EXISTS: a NOT IN of them is, once the tables are analyzed, a SubPlan run for each node alive, which
-- reads the holders whole each time, as many nodes times as many holders (decision 244); a holder's id is never null,
-- so the two say the same.
WITH RECURSIVE holders AS (
    SELECT n.parent_id AS id FROM upgrade_nodes b
    JOIN LATERAL (SELECT n.parent_id FROM nodes n WHERE n.id = b.node_id AND n.deleted_at IS NULL LIMIT 1) n ON true
    WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.baseline AND n.parent_id IS NOT NULL
  UNION
    SELECT n.parent_id FROM holders h
    JOIN LATERAL (SELECT n.parent_id FROM nodes n WHERE n.id = h.id LIMIT 1) n ON true
    WHERE n.parent_id IS NOT NULL
)
SELECT n.id, n.parent_id FROM nodes n
LEFT JOIN LATERAL (
    SELECT true AS baseline FROM upgrade_nodes b WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.node_id = n.id AND b.baseline
    LIMIT 1
) b ON true
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL AND b.baseline IS NULL
    AND NOT EXISTS (SELECT 1 FROM holders h WHERE h.id = n.id);

-- name: RollbackHolders :many
-- The new nodes alive holding baseline nodes alive, their ancestors with no baseline row, as they are now (M11
-- design 4.10): what a rollback's plan deletes once the baseline nodes under them are put back. Read as
-- RollbackFreeForest reads them, each holder's node by its key.
WITH RECURSIVE holders AS (
    SELECT n.parent_id AS id FROM upgrade_nodes b
    JOIN LATERAL (SELECT n.parent_id FROM nodes n WHERE n.id = b.node_id AND n.deleted_at IS NULL LIMIT 1) n ON true
    WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.baseline AND n.parent_id IS NOT NULL
  UNION
    SELECT n.parent_id FROM holders h
    JOIN LATERAL (SELECT n.parent_id FROM nodes n WHERE n.id = h.id LIMIT 1) n ON true
    WHERE n.parent_id IS NOT NULL
)
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at
FROM holders h
JOIN LATERAL (
    SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
        n.created_at, n.updated_at
    FROM nodes n WHERE n.id = h.id AND n.deleted_at IS NULL LIMIT 1
) n ON true
LEFT JOIN LATERAL (
    SELECT true AS baseline FROM upgrade_nodes b WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.node_id = h.id AND b.baseline
    LIMIT 1
) b ON true
WHERE b.baseline IS NULL;

-- name: RollbackBaseline :many
-- An upgrade's baseline rows, every one, by upgrade_nodes_pkey's prefix (M11 design 4.10): what a rollback's plan
-- puts back.
SELECT node_id, kind, parent_id, name, sort_order, revision, content_hash, byte_size FROM upgrade_nodes
WHERE upgrade_id = sqlc.arg(upgrade_id) AND baseline;

-- name: RollbackNodesNow :many
-- The upgrade's baseline nodes alive as they are now (M11 design 4.10), each by its key, as RollbackFreeForest
-- reads them: what a rollback's plan compares with the baseline.
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at
FROM upgrade_nodes b
JOIN LATERAL (
    SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
        n.created_at, n.updated_at
    FROM nodes n WHERE n.id = b.node_id AND n.deleted_at IS NULL LIMIT 1
) n ON true
WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.baseline;

-- name: RollbackContentsNow :many
-- The contents now of the upgrade's baseline pages (M11 design 4.10), their revisions, hashes and byte sizes, each
-- by page_contents_pkey, as RollbackFreeForest reads its rows, without reading the bodies.
SELECT b.node_id, c.revision, c.content_hash, c.byte_size
FROM upgrade_nodes b
JOIN LATERAL (
    SELECT c.revision, c.content_hash, c.byte_size FROM page_contents c
    WHERE c.node_id = b.node_id AND c.deleted_at IS NULL LIMIT 1
) c ON true
WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.baseline AND b.kind = 'page';

-- name: RollbackLeft :one
-- What a rollback has left to do (M11 design 4.10, decision 226): the baseline nodes not in place (parent, name or
-- order not the baseline's, or not alive), the baseline pages whose content is not the baseline's, and the live
-- nodes with no baseline row. getUpgrade, endUpgradeRollback and the rollback's end read it alike. Read as
-- RollbackFreeForest reads its rows: a baseline row's node and content each by its key, a live node's baseline
-- row by upgrade_nodes_pkey.
SELECT
    (SELECT count(*) FILTER (WHERE n.alive IS NULL OR n.parent_id IS DISTINCT FROM b.parent_id OR n.name <> b.name
         OR n.sort_order <> b.sort_order)
     FROM upgrade_nodes b
     LEFT JOIN LATERAL (
         SELECT true AS alive, n.parent_id, n.name, n.sort_order FROM nodes n WHERE n.id = b.node_id AND n.deleted_at IS NULL
         LIMIT 1
     ) n ON true
     WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.baseline)::integer AS nodes,
    (SELECT count(*) FROM upgrade_nodes b
     JOIN LATERAL (
         SELECT c.content_hash FROM page_contents c WHERE c.node_id = b.node_id AND c.deleted_at IS NULL LIMIT 1
     ) c ON true
     WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.baseline AND b.kind = 'page'
         AND c.content_hash <> b.content_hash)::integer AS pages,
    (SELECT count(*) FROM nodes n
     LEFT JOIN LATERAL (
         SELECT true AS baseline FROM upgrade_nodes b
         WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.node_id = n.id AND b.baseline LIMIT 1
     ) b ON true
     WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL AND b.baseline IS NULL)::integer AS created;

-- name: DeleteUpgradeNodes :execrows
-- An upgrade's rows, as it ends (M11 design 4.10), by upgrade_nodes_pkey's prefix.
DELETE FROM upgrade_nodes WHERE upgrade_id = sqlc.arg(upgrade_id);
