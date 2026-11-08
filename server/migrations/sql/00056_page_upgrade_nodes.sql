-- upgrade_nodes: an upgrade's baseline (M11 design 4.2): a row for each node alive as the upgrade started, with its
-- kind, parent, name and order then, and a page's revision, content hash and size, the distinct wanted targets it
-- linked to, and the hash the guardrails last accepted. A row not of the baseline protects the source root the start
-- created: a page at the root, nothing else. No deleted_at and no foreign keys, as edit_sessions and the link
-- index's tables: the rows go as the upgrade ends and as the notebook is deleted, so no purger. changesets.upgrade_id
-- is the upgrade a changeset was written in (no key: the upgrades are the wiki module's).

-- +goose Up
CREATE TABLE upgrade_nodes (
    upgrade_id uuid NOT NULL,
    node_id uuid NOT NULL,
    notebook_id uuid NOT NULL,
    baseline boolean NOT NULL,
    kind text NOT NULL,
    parent_id uuid,
    name text,
    sort_order double precision,
    revision integer,
    content_hash bytea,
    byte_size integer,
    wanted integer,
    accepted_hash bytea,
    CONSTRAINT upgrade_nodes_pkey PRIMARY KEY (upgrade_id, node_id),
    CONSTRAINT upgrade_nodes_kind_check CHECK (kind IN ('page', 'asset')),
    CONSTRAINT upgrade_nodes_baseline_check CHECK (baseline = (name IS NOT NULL) AND (name IS NULL) = (sort_order IS NULL)
        AND (baseline OR kind = 'page' AND parent_id IS NULL)),
    CONSTRAINT upgrade_nodes_page_check CHECK ((baseline AND kind = 'page') = (revision IS NOT NULL)
        AND (revision IS NULL) = (content_hash IS NULL) AND (revision IS NULL) = (byte_size IS NULL)
        AND (revision IS NULL) = (wanted IS NULL) AND (revision IS NULL) = (accepted_hash IS NULL)),
    CONSTRAINT upgrade_nodes_wanted_check CHECK (wanted IS NULL OR wanted BETWEEN 0 AND 10000)
);
CREATE INDEX upgrade_nodes_notebook_id_idx ON upgrade_nodes (notebook_id);
ALTER TABLE changesets ADD COLUMN upgrade_id uuid;
CREATE INDEX changesets_upgrade_id_idx ON changesets (upgrade_id) WHERE upgrade_id IS NOT NULL;

-- +goose Down
DROP INDEX changesets_upgrade_id_idx;
ALTER TABLE changesets DROP COLUMN upgrade_id;
DROP TABLE upgrade_nodes;
