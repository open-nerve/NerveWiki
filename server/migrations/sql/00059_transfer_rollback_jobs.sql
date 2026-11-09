-- transfer_jobs: an upgrade's rollback (M11 design 4.10), a job that writes its notebook's tree as a restore and a
-- revert do, one at a time with them; upgrade_id is the upgrade it rolls back (no key: the wiki module's), set on it
-- alone. Its changeset is made by its first unit that writes, as theirs. transfer_jobs_changeset_check,
-- transfer_jobs_target_check and transfer_jobs_message_check hold of it unchanged: a changeset, no target, no message.

-- +goose Up
ALTER TABLE transfer_jobs DROP CONSTRAINT transfer_jobs_kind_check,
    ADD CONSTRAINT transfer_jobs_kind_check CHECK (kind IN ('import', 'export', 'trash_restore', 'revert', 'rollback')),
    ADD COLUMN upgrade_id uuid,
    ADD CONSTRAINT transfer_jobs_upgrade_check CHECK ((upgrade_id IS NOT NULL) = (kind = 'rollback')),
    DROP CONSTRAINT transfer_jobs_root_check,
    ADD CONSTRAINT transfer_jobs_root_check CHECK (CASE kind WHEN 'trash_restore' THEN root_id IS NOT NULL
        WHEN 'revert' THEN root_id IS NULL WHEN 'rollback' THEN root_id IS NULL ELSE true END);
DROP INDEX transfer_jobs_writing_key;
CREATE UNIQUE INDEX transfer_jobs_writing_key ON transfer_jobs (notebook_id)
    WHERE kind IN ('import', 'trash_restore', 'revert', 'rollback') AND state IN ('queued', 'running')
        AND deleted_at IS NULL;

-- +goose Down
DELETE FROM transfer_jobs WHERE kind = 'rollback';
DROP INDEX transfer_jobs_writing_key;
CREATE UNIQUE INDEX transfer_jobs_writing_key ON transfer_jobs (notebook_id)
    WHERE kind IN ('import', 'trash_restore', 'revert') AND state IN ('queued', 'running') AND deleted_at IS NULL;
ALTER TABLE transfer_jobs DROP CONSTRAINT transfer_jobs_root_check,
    ADD CONSTRAINT transfer_jobs_root_check CHECK (CASE kind WHEN 'trash_restore' THEN root_id IS NOT NULL
        WHEN 'revert' THEN root_id IS NULL ELSE true END),
    DROP CONSTRAINT transfer_jobs_upgrade_check, DROP COLUMN upgrade_id,
    DROP CONSTRAINT transfer_jobs_kind_check,
    ADD CONSTRAINT transfer_jobs_kind_check CHECK (kind IN ('import', 'export', 'trash_restore', 'revert'));
