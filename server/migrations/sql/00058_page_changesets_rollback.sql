-- changesets: an upgrade's rollback (M11 design 4.10, decision 192), a kind of its own: it reverts no one changeset,
-- so not revert, whose reverts_id changesets_reverts_check asks for. Its upgrade_id is the upgrade rolled back.

-- +goose Up
ALTER TABLE changesets DROP CONSTRAINT changesets_kind_check,
    ADD CONSTRAINT changesets_kind_check
        CHECK (kind IN ('edit', 'import', 'version_restore', 'trash_restore', 'revert', 'rollback'));

-- +goose Down
-- A program before it knows no rollback: its changesets read as edits, their writes kept (as 00034's Down).
UPDATE changesets SET kind = 'edit' WHERE kind = 'rollback';
ALTER TABLE changesets DROP CONSTRAINT changesets_kind_check,
    ADD CONSTRAINT changesets_kind_check CHECK (kind IN ('edit', 'import', 'version_restore', 'trash_restore', 'revert'));
