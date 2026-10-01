-- workspace_invitations: an invitation to join a workspace by an e-mail
-- address (M2/P3 design 3.1). The link's token is not stored: it is a MAC
-- of the id. A row stops being pending when it is accepted or deleted, and
-- is kept for the purge (M2/P4).

-- +goose Up
CREATE TABLE workspace_invitations (
    id uuid PRIMARY KEY,
    -- The purge of a deleted workspace takes its invitations with it.
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    -- Normalized, by users.email's rule: the two are compared on acceptance.
    email varchar(255) NOT NULL
        CONSTRAINT workspace_invitations_email_check CHECK (email = lower(email) AND email !~ '[[:space:]]'),
    role text NOT NULL CONSTRAINT workspace_invitations_role_check CHECK (role IN ('admin', 'member', 'guest')),
    -- When it was accepted; it is deleted at the same moment. NULL when it
    -- was deleted otherwise, or is pending.
    accepted_at timestamptz,
    created_by_id uuid NOT NULL REFERENCES users,
    updated_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- When it stopped being pending: accepted, or deleted by an admin, by
    -- the end of the membership of the address's account, or with the
    -- workspace.
    deleted_at timestamptz,
    CONSTRAINT workspace_invitations_accepted_check
        CHECK (accepted_at IS NULL OR (deleted_at IS NOT NULL AND accepted_at = deleted_at))
);
-- At most one pending invitation per workspace and address; a membership's
-- end deletes the pending one of its account's address through it.
CREATE UNIQUE INDEX workspace_invitations_workspace_id_email_key ON workspace_invitations (workspace_id, email)
    WHERE deleted_at IS NULL;
-- The purge's cascade looks invitations up by workspace, deleted ones
-- included, which the partial index above does not hold.
CREATE INDEX workspace_invitations_workspace_id_idx ON workspace_invitations (workspace_id);

-- +goose Down
DROP TABLE workspace_invitations;
