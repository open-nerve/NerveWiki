-- Ingest marks belong to page. Sources remain historical after their nodes
-- disappear, so neither column has a foreign key. PurgeChangesets owns cleanup.
-- +goose Up
ALTER TABLE changesets ADD COLUMN ingest_count integer NOT NULL DEFAULT 0
    CONSTRAINT changesets_ingest_count_check CHECK (ingest_count >= 0);
CREATE TABLE changeset_sources (
    changeset_id uuid NOT NULL, seq integer NOT NULL, node_id uuid NOT NULL,
    CONSTRAINT changeset_sources_pkey PRIMARY KEY (changeset_id, seq),
    CONSTRAINT changeset_sources_changeset_id_node_id_key UNIQUE (changeset_id, node_id)
);
CREATE INDEX changesets_notebook_id_created_at_ingest_idx
    ON changesets (notebook_id, created_at DESC, id DESC) WHERE ingest_count > 0;
CREATE INDEX changesets_notebook_id_created_by_id_created_at_ingest_idx
    ON changesets (notebook_id, created_by_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL AND ingest_count > 0;

-- +goose Down
DROP INDEX changesets_notebook_id_created_at_ingest_idx,
    changesets_notebook_id_created_by_id_created_at_ingest_idx;
DROP TABLE changeset_sources;
ALTER TABLE changesets DROP COLUMN ingest_count;
