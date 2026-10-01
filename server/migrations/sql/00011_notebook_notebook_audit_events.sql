-- notebook_audit_events: what was done with ownerless notebooks, which the
-- workspace's admins read (v0.1 design 3.4; M3/P3 design 3.4). A row is
-- written once and changes only when its workspace is deleted.

-- +goose Up
CREATE TABLE notebook_audit_events (
    id uuid PRIMARY KEY,
    -- Another module's table: the purge deletes a deleted workspace's events
    -- with the notebook module's purger, before the workspace's (v0.1 design 13.1, item 6).
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE RESTRICT,
    -- No reference: an event outlives its notebook, whose purge leaves it.
    notebook_id uuid NOT NULL,
    -- The notebook's name when it happened.
    notebook_name text NOT NULL,
    action text NOT NULL
        CONSTRAINT notebook_audit_events_action_check CHECK (action IN ('taken_over', 'deleted', 'returned')),
    -- Whose notebook it was: its admin when it became ownerless.
    former_owner_id uuid NOT NULL REFERENCES users,
    -- Who did it: a workspace admin took it over or deleted it, or its former
    -- owner came back to it.
    created_by_id uuid NOT NULL REFERENCES users,
    -- Who last changed it; on a deleted event, who deleted its workspace.
    updated_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- Set only with the workspace's, in the same transaction.
    deleted_at timestamptz
);
-- A workspace's events, newest first: the list's order and its cursor.
CREATE INDEX notebook_audit_events_workspace_id_created_at_idx ON notebook_audit_events (workspace_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
-- A workspace's events, deleted ones included: the purge's look-up of a
-- workspace's rows, which the partial index above does not hold.
CREATE INDEX notebook_audit_events_workspace_id_idx ON notebook_audit_events (workspace_id);

-- +goose Down
DROP TABLE notebook_audit_events;
