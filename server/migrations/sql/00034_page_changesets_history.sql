-- changesets: M8's kinds (design 4.2): a version restored, a trash item restored, a changeset reverted, which names
-- the changeset it reverts (no key: the one reverted may be purged first); and the mark the purge sets on a changeset
-- of a live notebook whose items or versions it deleted, which can no longer be reverted whole (M8/P3).

-- +goose Up
ALTER TABLE changesets DROP CONSTRAINT changesets_kind_check,
    ADD CONSTRAINT changesets_kind_check CHECK (kind IN ('edit', 'import', 'version_restore', 'trash_restore', 'revert')),
    ADD COLUMN reverts_id uuid,
    ADD CONSTRAINT changesets_reverts_check CHECK ((kind = 'revert') = (reverts_id IS NOT NULL)),
    ADD COLUMN trimmed boolean NOT NULL DEFAULT false;

-- +goose Down
-- A program before it knows only edits and imports: the new kinds read as edits, their writes kept.
ALTER TABLE changesets DROP CONSTRAINT changesets_reverts_check, DROP COLUMN reverts_id, DROP COLUMN trimmed;
UPDATE changesets SET kind = 'edit' WHERE kind NOT IN ('edit', 'import');
ALTER TABLE changesets DROP CONSTRAINT changesets_kind_check,
    ADD CONSTRAINT changesets_kind_check CHECK (kind IN ('edit', 'import'));
