-- A running job holds its report as it goes, which its heartbeat writes and the rescue keeps when the job is
-- interrupted (M7/P6 design 3.13): a queued job holds none; one that ended, its own.

-- +goose Up
ALTER TABLE transfer_jobs DROP CONSTRAINT transfer_jobs_finished_check,
    ADD CONSTRAINT transfer_jobs_finished_check CHECK (
        (state IN ('queued', 'running')) = (finished_at IS NULL) AND (finished_at IS NULL OR report IS NOT NULL)
        AND (state <> 'queued' OR report IS NULL));

-- +goose Down
UPDATE transfer_jobs SET report = NULL WHERE state = 'running';
ALTER TABLE transfer_jobs DROP CONSTRAINT transfer_jobs_finished_check,
    ADD CONSTRAINT transfer_jobs_finished_check CHECK (
        (state IN ('queued', 'running')) = (finished_at IS NULL) AND (finished_at IS NULL) = (report IS NULL));
