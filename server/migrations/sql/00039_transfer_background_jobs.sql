-- transfer_jobs: the background restores and reverts (M8/P4 design 3.2): a trash item's restore (root_id the item)
-- and a changeset's revert (target_changeset_id the changeset, message its revert's), and the changeset each writing
-- job writes, an import's too: its id decided as the row is made, the changeset made by the job's first unit that
-- writes, with it. An import made before has none. No foreign keys: the changesets are the page module's, purged by
-- its purger, which a job's row must not hold.

-- +goose Up
ALTER TABLE transfer_jobs DROP CONSTRAINT transfer_jobs_kind_check,
    ADD CONSTRAINT transfer_jobs_kind_check CHECK (kind IN ('import', 'export', 'trash_restore', 'revert')),
    ADD COLUMN changeset_id uuid,
    ADD COLUMN target_changeset_id uuid,
    ADD COLUMN message text,
    ADD CONSTRAINT transfer_jobs_changeset_check CHECK (CASE kind WHEN 'export' THEN changeset_id IS NULL
        WHEN 'import' THEN true ELSE changeset_id IS NOT NULL END),
    ADD CONSTRAINT transfer_jobs_target_check CHECK ((target_changeset_id IS NOT NULL) = (kind = 'revert')),
    ADD CONSTRAINT transfer_jobs_message_check CHECK (message IS NULL
        OR kind = 'revert' AND octet_length(message) BETWEEN 1 AND 4096),
    ADD CONSTRAINT transfer_jobs_root_check CHECK (CASE kind WHEN 'trash_restore' THEN root_id IS NOT NULL
        WHEN 'revert' THEN root_id IS NULL ELSE true END);
-- A queued import runs as the new ones do, merging into the changeset its row names; a running one is interrupted
-- by the restart this migration runs in, and runs no more.
UPDATE transfer_jobs SET changeset_id = uuidv7() WHERE kind = 'import' AND state = 'queued';
-- One writing job queued or running in a notebook at once, whoever started it (M8 design 4.10): an import, a
-- restore and a revert each write the notebook's tree in units that would race another's.
DROP INDEX transfer_jobs_importing_key;
CREATE UNIQUE INDEX transfer_jobs_writing_key ON transfer_jobs (notebook_id)
    WHERE kind IN ('import', 'trash_restore', 'revert') AND state IN ('queued', 'running') AND deleted_at IS NULL;

-- +goose Down
DELETE FROM transfer_jobs WHERE kind IN ('trash_restore', 'revert');
DROP INDEX transfer_jobs_writing_key;
CREATE UNIQUE INDEX transfer_jobs_importing_key ON transfer_jobs (notebook_id)
    WHERE kind = 'import' AND state IN ('queued', 'running') AND deleted_at IS NULL;
ALTER TABLE transfer_jobs DROP CONSTRAINT transfer_jobs_root_check, DROP CONSTRAINT transfer_jobs_message_check,
    DROP CONSTRAINT transfer_jobs_target_check, DROP CONSTRAINT transfer_jobs_changeset_check,
    DROP COLUMN message, DROP COLUMN target_changeset_id, DROP COLUMN changeset_id,
    DROP CONSTRAINT transfer_jobs_kind_check,
    ADD CONSTRAINT transfer_jobs_kind_check CHECK (kind IN ('import', 'export'));
