-- notebooks: a workspace's notebooks (v0.1 design 3.3; M3/P1 design 3.6).
-- The use cases write every column, the times from their clock.

-- +goose Up
CREATE TABLE notebooks (
    id uuid PRIMARY KEY,
    -- Another module's table: the purge deletes a deleted workspace's notebooks
    -- with the notebook module's purger, before the workspace's (v0.1 design 13.1, item 6).
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE RESTRICT,
    -- The domain trims the name, holds it in NFC and refuses what a file name
    -- cannot be (shared.CheckTitle); the CHECK keeps what it can of that.
    name text NOT NULL CONSTRAINT notebooks_name_check CHECK (name <> '' AND octet_length(name) <= 255),
    -- The role it gives the workspace's admins and members who are not its members.
    workspace_access text NOT NULL DEFAULT 'none'
        CONSTRAINT notebooks_workspace_access_check CHECK (workspace_access IN ('none', 'viewer', 'editor')),
    -- Since when it has no active admin, and who its admin was (M3/P3).
    ownerless_since timestamptz,
    former_owner_id uuid REFERENCES users,
    created_by_id uuid NOT NULL REFERENCES users,
    -- Who last changed it; on a deleted notebook, who deleted it.
    updated_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- Soft delete: the clean-up purges it after the retention (v0.1 design 7.1).
    deleted_at timestamptz,
    CONSTRAINT notebooks_ownerless_check CHECK ((ownerless_since IS NULL) = (former_owner_id IS NULL))
);
-- A workspace's notebooks, deleted ones included: the list, and the purge's
-- look-up of a workspace's rows.
CREATE INDEX notebooks_workspace_id_idx ON notebooks (workspace_id);

-- +goose Down
DROP TABLE notebooks;
