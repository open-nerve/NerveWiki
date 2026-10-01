-- notebook_members: an account's membership of a notebook and its role
-- (v0.1 design 3.3; M3/P1 design 3.6). One row per notebook and account:
-- a membership that ends keeps its row, and coming back restores it.

-- +goose Up
CREATE TABLE notebook_members (
    id uuid PRIMARY KEY,
    -- The purge deletes a deleted notebook's members before it; the cascade is
    -- the fallback within the module.
    notebook_id uuid NOT NULL REFERENCES notebooks ON DELETE CASCADE,
    -- Accounts are never deleted (v0.1 design 6.2): no cascade to choose.
    user_id uuid NOT NULL REFERENCES users,
    role text NOT NULL CONSTRAINT notebook_members_role_check CHECK (role IN ('admin', 'editor', 'reader')),
    -- When the membership ended: removed, left, or ended with the workspace's
    -- membership. NULL while it is active.
    ended_at timestamptz,
    created_by_id uuid NOT NULL REFERENCES users,
    updated_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- Set only with the notebook's, in the same transaction.
    deleted_at timestamptz
);
CREATE UNIQUE INDEX notebook_members_notebook_id_user_id_key ON notebook_members (notebook_id, user_id)
    WHERE deleted_at IS NULL;
-- An account's active memberships: what the end of its workspace membership ends.
CREATE INDEX notebook_members_user_id_idx ON notebook_members (user_id)
    WHERE deleted_at IS NULL AND ended_at IS NULL;
-- A notebook's members, deleted ones included: the member counts, and the
-- purge's look-up, which the partial index above does not hold.
CREATE INDEX notebook_members_notebook_id_idx ON notebook_members (notebook_id);

-- +goose Down
DROP TABLE notebook_members;
