-- One import queued or running in a notebook at once, whoever started it (M7 design 4.9; M7/P6 design 3.14): its
-- units would race another's for the same names.

-- +goose Up
CREATE UNIQUE INDEX transfer_jobs_importing_key ON transfer_jobs (notebook_id)
    WHERE kind = 'import' AND state IN ('queued', 'running') AND deleted_at IS NULL;

-- +goose Down
DROP INDEX transfer_jobs_importing_key;
