-- workspace_members: an account's membership of a workspace and its role
-- (v0.1 design 3.2; M2/P1 design 3.5). One row per workspace and account:
-- a membership that ends keeps its row, and coming back restores it.

-- +goose Up
CREATE TABLE workspace_members (
    id uuid PRIMARY KEY,
    -- The purge of a deleted workspace takes its members with it.
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    -- Accounts are never deleted (v0.1 design 6.2): no cascade to choose.
    user_id uuid NOT NULL REFERENCES users,
    role text NOT NULL CONSTRAINT workspace_members_role_check CHECK (role IN ('admin', 'member', 'guest')),
    -- When the membership ended: removed, left, or the account deactivated.
    -- NULL while it is active.
    ended_at timestamptz,
    created_by_id uuid NOT NULL REFERENCES users,
    updated_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- Set only with the workspace's, in the same transaction.
    deleted_at timestamptz
);
CREATE UNIQUE INDEX workspace_members_workspace_id_user_id_key ON workspace_members (workspace_id, user_id)
    WHERE deleted_at IS NULL;
-- An account's active memberships: its workspaces, and what a deactivation ends.
CREATE INDEX workspace_members_user_id_idx ON workspace_members (user_id)
    WHERE deleted_at IS NULL AND ended_at IS NULL;
-- The purge's cascade looks members up by workspace, deleted ones included,
-- which the partial index above does not hold.
CREATE INDEX workspace_members_workspace_id_idx ON workspace_members (workspace_id);

-- +goose Down
DROP TABLE workspace_members;
