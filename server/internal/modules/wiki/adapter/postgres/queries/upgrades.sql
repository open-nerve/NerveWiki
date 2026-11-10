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
-- notebook not deleted; its submission and the report it kept (M11/P2 design 3.12).
SELECT id, notebook_id, state, ending, source_root_id, seeded_root, seeded_agents_id, locale, baseline_nodes,
    baseline_pages, baseline_wanted, baseline_wanted_capped, submission_id, submitted_by_id, submitted_at, report,
    rollback_job_id, rollback_complete, rollback_left_nodes, rollback_left_pages, rollback_left_created, created_by_id,
    created_at
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

-- name: EndUpgradeRollback :one
-- A rollback's end (M11 design 4.10, decision 230), under the notebook's row lock: the upgrade rolled back, complete
-- when nothing was left, else what was left; who and when. It answers when the row changed last before, as the
-- rollback was abandoned or continued, which the end's log measures from; no row when the upgrade no longer rolls
-- back.
UPDATE notebook_upgrades u
SET state = 'rolled_back', rollback_complete = sqlc.arg(complete), rollback_left_nodes = sqlc.narg(left_nodes)::integer,
    rollback_left_pages = sqlc.narg(left_pages)::integer, rollback_left_created = sqlc.narg(left_created)::integer,
    updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
FROM (SELECT o.id, o.updated_at FROM notebook_upgrades o WHERE o.id = sqlc.arg(id)) b
WHERE u.id = b.id AND u.state = 'rolling_back' AND u.deleted_at IS NULL
RETURNING b.updated_at AS since;

-- name: RollBackUpgrade :execrows
-- An upgrade abandoned (M11 design 4.10), under the notebook's row lock: rolling back, why, and its rollback's job, in
-- one statement, as notebook_upgrades_rollback_job_check wants the job with the state. No row when the upgrade is not
-- upgrading or reviewing.
UPDATE notebook_upgrades
SET state = 'rolling_back', ending = sqlc.arg(ending)::text, rollback_job_id = sqlc.arg(job_id)::uuid,
    updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND state IN ('upgrading', 'reviewing') AND deleted_at IS NULL;

-- name: ContinueUpgradeRollback :execrows
-- A stopped rollback continued (M11 design 4.10), under the notebook's row lock: its new job. No row when the upgrade
-- does not roll back.
UPDATE notebook_upgrades
SET rollback_job_id = sqlc.arg(job_id)::uuid, updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND state = 'rolling_back' AND deleted_at IS NULL;

-- name: SubmitUpgrade :execrows
-- An upgrade's submission's first transaction (M11 design 4.8 step 1), under the notebook's row lock: reviewing, a new
-- submission, who and when; from upgrading, or from reviewing without a report whose submission was taken over.
UPDATE notebook_upgrades
SET state = 'reviewing', submission_id = sqlc.arg(submission_id), submitted_by_id = sqlc.arg(by)::uuid,
    submitted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
    AND (state = 'upgrading' OR state = 'reviewing' AND report IS NULL AND submitted_at <= sqlc.arg(taken_over)::timestamptz);

-- name: KeepUpgradeReport :execrows
-- A submission's report, as its check passed (M11 design 4.8 step 4), under the notebook's row lock.
UPDATE notebook_upgrades
SET report = sqlc.arg(report)::jsonb, updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND submission_id = sqlc.arg(submission_id) AND state = 'reviewing' AND report IS NULL
    AND deleted_at IS NULL;

-- name: UnsubmitUpgrade :execrows
-- A submission whose check did not pass, or failed (M11 design 4.8 step 4): upgrading again, its submission cleared.
UPDATE notebook_upgrades
SET state = 'upgrading', submission_id = NULL, submitted_by_id = NULL, submitted_at = NULL,
    updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND submission_id = sqlc.arg(submission_id) AND state = 'reviewing' AND report IS NULL
    AND deleted_at IS NULL;
