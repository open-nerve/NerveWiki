-- changesets: a notebook's log, one row per write (v0.1 design 3.8; M4/P1 design 3.4): who wrote, from
-- which client, and when.

-- +goose Up
CREATE TABLE changesets (
    id uuid PRIMARY KEY,
    -- Another module's table: deleted with the notebook, purged before it.
    notebook_id uuid NOT NULL REFERENCES notebooks ON DELETE RESTRICT,
    -- What kind of write: M4 writes edits; later milestones add their kinds here.
    kind text NOT NULL CONSTRAINT changesets_kind_check CHECK (kind IN ('edit')),
    -- web (a sign-in session), api (a personal access token), cli, or mcp: and the MCP client's name (M9).
    client text NOT NULL
        CONSTRAINT changesets_client_check CHECK (client IN ('web', 'api', 'cli') OR client ~ '^mcp:[^[:cntrl:]]{1,128}$'),
    message text CONSTRAINT changesets_message_check CHECK (octet_length(message) BETWEEN 1 AND 4096),
    created_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    -- An edit session's later writes move it on (M4/P4).
    updated_at timestamptz NOT NULL,
    -- Set with the notebook's.
    deleted_at timestamptz
);
CREATE INDEX changesets_notebook_id_idx ON changesets (notebook_id);
CREATE INDEX changesets_deleted_at_idx ON changesets (deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP TABLE changesets;
