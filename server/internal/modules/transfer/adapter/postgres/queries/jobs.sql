-- The jobs' rows (M7/P5 design 3.5). Each statement that moves a job names the state it moves it from: the state
-- machine is the statements', the checks keep each row's columns in step with its state.

-- name: CreateJob :exec
INSERT INTO transfer_jobs (id, notebook_id, root_id, kind, state, name, created_by_id, client, created_at)
VALUES (sqlc.arg(id), sqlc.arg(notebook_id), sqlc.narg(root_id), sqlc.arg(kind), 'queued', sqlc.arg(name),
    sqlc.arg(created_by_id), sqlc.arg(client), sqlc.arg(created_at));

-- name: LockQueue :exec
-- The jobs' creations one at a time, until the transaction ends: how many are queued or running is counted under it
-- (M7/P5 design 3.7). The key is the transfer module's alone.
SELECT pg_advisory_xact_lock(7366512418245025792);

-- name: CountActive :one
-- The jobs queued or running, of every notebook.
SELECT count(*) FROM transfer_jobs WHERE state IN ('queued', 'running') AND deleted_at IS NULL;

-- name: Exporting :one
-- Whether the account has an export queued or running in the notebook.
SELECT EXISTS (
    SELECT 1 FROM transfer_jobs
    WHERE notebook_id = sqlc.arg(notebook_id) AND created_by_id = sqlc.arg(created_by_id) AND kind = 'export'
        AND state IN ('queued', 'running') AND deleted_at IS NULL
);

-- name: FindJob :one
SELECT id, notebook_id, root_id, kind, state, name, created_by_id, client, progress_done, progress_total,
    cancel_requested_at, heartbeat_at, started_at, finished_at, report, result_bytes, created_at
FROM transfer_jobs
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: LockJob :one
-- FindJob locked FOR UPDATE until the transaction ends: a cancel decides on the state it reads.
SELECT id, notebook_id, root_id, kind, state, name, created_by_id, client, progress_done, progress_total,
    cancel_requested_at, heartbeat_at, started_at, finished_at, report, result_bytes, created_at
FROM transfer_jobs
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
FOR UPDATE;

-- name: ListJobs :many
-- A notebook's jobs, the newest first, the account's alone when created_by_id is set, after the cursor's. A report
-- comes without its problems, up to 1,000 a job, which the list does not show.
SELECT id, notebook_id, root_id, kind, state, name, created_by_id, client, progress_done, progress_total,
    cancel_requested_at, heartbeat_at, started_at, finished_at, (report - 'problems')::jsonb AS report, result_bytes, created_at
FROM transfer_jobs
WHERE notebook_id = sqlc.arg(notebook_id) AND deleted_at IS NULL
    AND (sqlc.narg(created_by_id)::uuid IS NULL OR created_by_id = sqlc.narg(created_by_id))
    AND (sqlc.narg(after_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: StartJob :one
-- A queued job, running: none when it was cancelled or deleted meanwhile.
UPDATE transfer_jobs SET state = 'running', started_at = sqlc.arg(at), heartbeat_at = sqlc.arg(at)
WHERE id = sqlc.arg(id) AND state = 'queued' AND deleted_at IS NULL
RETURNING id, notebook_id, root_id, kind, state, name, created_by_id, client, progress_done, progress_total,
    cancel_requested_at, heartbeat_at, started_at, finished_at, report, result_bytes, created_at;

-- name: BeatJob :one
-- A running job's heartbeat and progress; it reads back whether a cancel was asked and whether the job was deleted
-- (its notebook's deletion). None when the job no longer runs.
UPDATE transfer_jobs SET heartbeat_at = sqlc.arg(at), progress_done = sqlc.arg(done), progress_total = sqlc.arg(total)
WHERE id = sqlc.arg(id) AND state = 'running'
RETURNING (cancel_requested_at IS NOT NULL)::boolean AS cancel_requested, (deleted_at IS NOT NULL)::boolean AS deleted;

-- name: FinishJob :execrows
-- A running job, ended: its state, its report, an export's archive's bytes and the name it was exported under.
UPDATE transfer_jobs SET state = sqlc.arg(state), finished_at = sqlc.arg(at), report = sqlc.arg(report),
    result_bytes = sqlc.narg(result_bytes), name = sqlc.arg(name), progress_done = sqlc.arg(done),
    progress_total = sqlc.arg(total)
WHERE id = sqlc.arg(id) AND state = 'running' AND deleted_at IS NULL;

-- name: ExpireOthers :many
-- The account's other exports of the notebook that succeeded, expired: only the latest is kept (M7 design 4.9).
UPDATE transfer_jobs SET state = 'expired'
WHERE notebook_id = sqlc.arg(notebook_id) AND created_by_id = sqlc.arg(created_by_id) AND kind = 'export'
    AND state = 'succeeded' AND id <> sqlc.arg(id) AND deleted_at IS NULL
RETURNING id;

-- name: CancelQueued :execrows
-- A queued job, cancelled at once: River finds it so and does nothing.
UPDATE transfer_jobs SET state = 'cancelled', finished_at = sqlc.arg(at), report = sqlc.arg(report)
WHERE id = sqlc.arg(id) AND state = 'queued';

-- name: RequestCancel :execrows
-- A running job's cancel, asked once: it stops at its next heartbeat.
UPDATE transfer_jobs SET cancel_requested_at = coalesce(cancel_requested_at, sqlc.arg(at))
WHERE id = sqlc.arg(id) AND state = 'running';

-- name: ExpireExports :many
-- Up to batch exports that succeeded before before, expired, the oldest first; the rows another transaction holds
-- are skipped.
UPDATE transfer_jobs SET state = 'expired'
WHERE id IN (
    SELECT id FROM transfer_jobs
    WHERE kind = 'export' AND state = 'succeeded' AND deleted_at IS NULL AND finished_at < sqlc.arg(before)::timestamptz
    ORDER BY finished_at
    LIMIT sqlc.arg(batch)
    FOR UPDATE SKIP LOCKED
)
RETURNING id;

-- name: InterruptJobs :many
-- The running jobs, failed with report: all of them, or those whose heartbeat is older than beat_before when it is
-- set. Their progress stays as it was. The rows another transaction holds are skipped: a notebook's deletion that
-- holds some waits for none of these.
UPDATE transfer_jobs SET state = 'failed', finished_at = sqlc.arg(at), report = sqlc.arg(report)
WHERE state = 'running' AND id IN (
    SELECT id FROM transfer_jobs
    WHERE state = 'running' AND deleted_at IS NULL
        AND (sqlc.narg(beat_before)::timestamptz IS NULL OR heartbeat_at < sqlc.narg(beat_before)::timestamptz)
    FOR UPDATE SKIP LOCKED
)
RETURNING id, notebook_id, created_by_id, client;

-- name: QueuedExports :many
-- The queued exports not deleted: River holds them, or dropped them.
SELECT id FROM transfer_jobs WHERE kind = 'export' AND state = 'queued' AND deleted_at IS NULL;

-- name: FailQueued :many
-- Of ids, the queued jobs not deleted, failed with report: River dropped them (M7/P5 design 3.12). The rows another
-- transaction holds are skipped.
UPDATE transfer_jobs SET state = 'failed', finished_at = sqlc.arg(at), report = sqlc.arg(report)
WHERE state = 'queued' AND id IN (
    SELECT id FROM transfer_jobs
    WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND state = 'queued' AND deleted_at IS NULL
    FOR UPDATE SKIP LOCKED
)
RETURNING id, notebook_id, created_by_id, client;

-- name: DeleteJobsOfNotebooks :exec
-- The deleted notebooks' jobs, deleted at the deletion's time.
UPDATE transfer_jobs SET deleted_at = sqlc.arg(at)
WHERE notebook_id = ANY(sqlc.arg(notebook_ids)::uuid[]) AND deleted_at IS NULL;

-- name: LiveArchives :many
-- Of ids, the jobs whose archives are kept: the exports that succeeded, not deleted.
SELECT id FROM transfer_jobs
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL AND kind = 'export' AND state = 'succeeded';

-- name: ExpiredJobs :many
-- The purge's batch: up to batch rows deleted before before, the oldest deletions first, locked; the rows another
-- transaction holds are skipped.
SELECT id, kind FROM transfer_jobs
WHERE deleted_at < sqlc.arg(before)::timestamptz
ORDER BY deleted_at
LIMIT sqlc.arg(batch)
FOR UPDATE SKIP LOCKED;

-- name: DeleteJobs :execrows
DELETE FROM transfer_jobs WHERE id = ANY(sqlc.arg(ids)::uuid[]);
