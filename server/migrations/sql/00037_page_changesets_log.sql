-- changesets: a notebook's log (M8 design 4.2, 4.7; M8/P3 design 3.2). The log reads a notebook's changesets newer
-- first, all of them or one actor's; a changeset's reverts are looked up by what they revert; the purge deletes the
-- changesets it trimmed once their items and versions are gone. The client is a filter on the log's index: its
-- values are few.

-- +goose Up
-- A notebook's log, newer first, and the foreign key's look-up from notebooks, whose purge's RESTRICT check reads
-- changesets by notebook_id alone: the index the log reads serves both, so it is not partial, and the index on
-- notebook_id alone goes.
CREATE INDEX changesets_notebook_id_created_at_id_idx ON changesets (notebook_id, created_at DESC, id DESC);
CREATE INDEX changesets_notebook_id_created_by_id_created_at_id_idx
    ON changesets (notebook_id, created_by_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
-- What reverted a changeset: the log's reverted_by and a unit's merge (M8/P1 review C3).
CREATE INDEX changesets_reverts_id_idx ON changesets (reverts_id) WHERE reverts_id IS NOT NULL;
-- The changesets the purge trimmed, whose emptied ones it deletes.
CREATE INDEX changesets_trimmed_idx ON changesets (id) WHERE trimmed;
DROP INDEX changesets_notebook_id_idx;

-- +goose Down
CREATE INDEX changesets_notebook_id_idx ON changesets (notebook_id);
DROP INDEX changesets_trimmed_idx, changesets_reverts_id_idx, changesets_notebook_id_created_by_id_created_at_id_idx,
    changesets_notebook_id_created_at_id_idx;
