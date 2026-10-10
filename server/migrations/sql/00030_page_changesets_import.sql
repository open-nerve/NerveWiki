-- changesets.kind: an import writes its nodes in units of their own, merged into the changeset of its first
-- (M7/P6 design 3.2): one import, one changeset.

-- +goose Up
ALTER TABLE changesets DROP CONSTRAINT changesets_kind_check,
    ADD CONSTRAINT changesets_kind_check CHECK (kind IN ('edit', 'import'));

-- +goose Down
-- A program before it knows only edits: an import's changeset reads as one, its writes kept.
UPDATE changesets SET kind = 'edit' WHERE kind = 'import';
ALTER TABLE changesets DROP CONSTRAINT changesets_kind_check,
    ADD CONSTRAINT changesets_kind_check CHECK (kind IN ('edit'));
