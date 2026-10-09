-- transfer_jobs: the imports and exports of notebooks, each a background job (M7 design 4.9, 4.12; M7/P5 design 3.5).
-- A request writes the row queued and enqueues its River job in the same transaction; the job moves it on. Its
-- archive is a file of the store at exports/<id>.zip or imports/<id>.zip. No credential's id is kept: the sessions'
-- cleanup deletes their rows, and a job outlives the credential it was started with.

-- +goose Up
CREATE TABLE transfer_jobs (
    -- The job's id (UUIDv7): its River job's argument and its archive's name.
    id uuid PRIMARY KEY,
    -- Another module's table: the purge deletes a deleted notebook's jobs before it (v0.1 design 13.1, item 6).
    notebook_id uuid NOT NULL REFERENCES notebooks ON DELETE RESTRICT,
    -- The page exported with its subtree, or the parent imported under; NULL for the whole notebook, or its root. No
    -- key: one to nodes would hold their purge.
    root_id uuid,
    kind text NOT NULL CONSTRAINT transfer_jobs_kind_check CHECK (kind IN ('import', 'export')),
    state text NOT NULL CONSTRAINT transfer_jobs_state_check
        CHECK (state IN ('queued', 'running', 'succeeded', 'failed', 'cancelled', 'expired')),
    -- What is exported or imported, by name: the notebook's or the page's, the archive's; the list shows it and the
    -- download is named after it.
    name text NOT NULL CONSTRAINT transfer_jobs_name_check CHECK (octet_length(name) BETWEEN 1 AND 255),
    -- Who started it, and from where: the job acts as them (shared.Actor's JobID), its writes from that client.
    created_by_id uuid NOT NULL REFERENCES users,
    client text NOT NULL CONSTRAINT transfer_jobs_client_check CHECK (client IN ('web', 'api')),
    -- The nodes done of all: an export's written, an import's entries processed.
    progress_done bigint NOT NULL DEFAULT 0,
    progress_total bigint NOT NULL DEFAULT 0,
    -- A cancel of a running job, which stops between its steps; a queued one is cancelled at once.
    cancel_requested_at timestamptz,
    -- A running job writes it every second: one that stopped writing it was interrupted.
    heartbeat_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    -- The report, written as the job ends: its failure, its counts and its problems (M7/P5 design 3.9).
    report jsonb,
    -- The bytes of an export's archive.
    result_bytes bigint,
    created_at timestamptz NOT NULL,
    -- Set with its notebook's deletion: a queued job then does nothing, a running one stops.
    deleted_at timestamptz,
    CONSTRAINT transfer_jobs_progress_check CHECK (progress_done >= 0 AND progress_done <= progress_total),
    -- A queued job has not started; a running one has, and beats; a cancel is asked of a job that started.
    CONSTRAINT transfer_jobs_started_check CHECK (
        (state <> 'queued' OR started_at IS NULL AND heartbeat_at IS NULL)
        AND (state <> 'running' OR started_at IS NOT NULL AND heartbeat_at IS NOT NULL)
        AND (cancel_requested_at IS NULL OR started_at IS NOT NULL)),
    -- A job that ended has its time and its report; one that did not, neither.
    CONSTRAINT transfer_jobs_finished_check CHECK (
        (state IN ('queued', 'running')) = (finished_at IS NULL) AND (finished_at IS NULL) = (report IS NULL)),
    -- Only an export's archive expires, and only an export that succeeded has one.
    CONSTRAINT transfer_jobs_expired_check CHECK (state <> 'expired' OR kind = 'export'),
    CONSTRAINT transfer_jobs_result_bytes_check CHECK (
        (result_bytes IS NOT NULL) = (kind = 'export' AND state IN ('succeeded', 'expired')) AND result_bytes >= 0)
);
-- A notebook's jobs, newest first: its list.
CREATE INDEX transfer_jobs_notebook_id_created_at_idx ON transfer_jobs (notebook_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
-- One export queued or running for a reader in a notebook at once (M7 design 4.9).
CREATE UNIQUE INDEX transfer_jobs_exporting_key ON transfer_jobs (notebook_id, created_by_id)
    WHERE kind = 'export' AND state IN ('queued', 'running') AND deleted_at IS NULL;
-- The jobs queued or running: how many wait, and those to rescue.
CREATE INDEX transfer_jobs_state_idx ON transfer_jobs (state) WHERE state IN ('queued', 'running') AND deleted_at IS NULL;
-- The exports that succeeded, by when: their expiry.
CREATE INDEX transfer_jobs_finished_at_idx ON transfer_jobs (finished_at)
    WHERE kind = 'export' AND state = 'succeeded' AND deleted_at IS NULL;
-- The purge's batches.
CREATE INDEX transfer_jobs_deleted_at_idx ON transfer_jobs (deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP TABLE transfer_jobs;
