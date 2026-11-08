-- name: InsertUpgrade :exec
-- An upgrade's start (M11 design 4.4 step 14), under the notebook's row lock: upgrading, its proposed source root,
-- the pages it wrote, its template's language and its baseline's counts.
INSERT INTO notebook_upgrades (id, notebook_id, state, source_root_id, seeded_root, seeded_agents_id, locale, baseline_nodes,
    baseline_pages, baseline_wanted, baseline_wanted_capped, created_by_id, updated_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(notebook_id), 'upgrading', sqlc.arg(source_root_id), sqlc.arg(seeded_root),
    sqlc.narg(seeded_agents_id), sqlc.arg(locale), sqlc.arg(baseline_nodes), sqlc.arg(baseline_pages),
    sqlc.arg(baseline_wanted), sqlc.arg(baseline_wanted_capped), sqlc.arg(by), sqlc.arg(by), sqlc.arg(now), sqlc.arg(now));

-- name: CurrentUpgrade :one
-- The notebook's upgrade under way, by notebook_upgrades_notebook_id_key: upgrading, reviewing or rolling back, its
-- notebook not deleted. Its report is P2's.
SELECT id, notebook_id, state, ending, source_root_id, seeded_root, seeded_agents_id, locale, baseline_nodes,
    baseline_pages, baseline_wanted, baseline_wanted_capped, submitted_by_id, submitted_at, rollback_job_id,
    rollback_complete, rollback_left_nodes, rollback_left_pages, rollback_left_created, created_by_id, created_at
FROM notebook_upgrades
WHERE notebook_id = sqlc.arg(notebook_id) AND state IN ('upgrading', 'reviewing', 'rolling_back') AND deleted_at IS NULL;

-- name: SoftDeleteUpgradesOf :exec
-- The notebooks' deletion (M11/P1 design 3.15): their upgrades, under way or ended, go with them at its time.
UPDATE notebook_upgrades
SET deleted_at = sqlc.arg(at)::timestamptz, updated_at = sqlc.arg(at)::timestamptz, updated_by_id = sqlc.arg(by)
WHERE notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]) AND deleted_at IS NULL;

-- name: PurgeUpgrades :execrows
-- The purge (v0.1 design 13.1, item 6): up to batch upgrades deleted before the time, those of the deleted notebooks.
-- A row another transaction holds is skipped, not waited for: the next run takes it, and the purge never joins the
-- lock order.
DELETE FROM notebook_upgrades
WHERE id IN (
    SELECT u.id FROM notebook_upgrades u
    WHERE u.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
);
