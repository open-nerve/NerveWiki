-- edit_sessions: who is editing which page, while their lease lasts (v0.1 design 3.9; M4 design 4; M4/P4
-- design 3.2). Runtime state: ending a session deletes its row, and so do deleting its page, its subtree or
-- its notebook, in their transaction. No foreign key to the page's tables: a session never outlives its page,
-- and the purge has nothing to order it by.

-- +goose Up
CREATE TABLE edit_sessions (
    id uuid PRIMARY KEY,
    -- The page edited, and its notebook, which a notebook's deletion deletes its sessions by.
    node_id uuid NOT NULL,
    notebook_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users,
    -- Where it was opened from, as a changeset's client.
    client text NOT NULL
        CONSTRAINT edit_sessions_client_check CHECK (client IN ('web', 'api', 'cli') OR client ~ '^mcp:[^[:cntrl:]]{1,128}$'),
    -- The session's changeset and the revision it last wrote, set together by its writes: its next write goes
    -- to that changeset while the page is still at that revision.
    changeset_id uuid,
    revision integer,
    created_at timestamptz NOT NULL,
    -- The lease: a session is alive while expires_at is later than now. A heartbeat moves it on.
    expires_at timestamptz NOT NULL,
    CONSTRAINT edit_sessions_written_check
        CHECK (changeset_id IS NULL AND revision IS NULL OR changeset_id IS NOT NULL AND revision IS NOT NULL AND revision >= 1),
    CONSTRAINT edit_sessions_expires_at_check CHECK (expires_at > created_at)
);
CREATE INDEX edit_sessions_node_id_idx ON edit_sessions (node_id);
CREATE INDEX edit_sessions_notebook_id_idx ON edit_sessions (notebook_id);
-- No index on expires_at: the heartbeats move it every 20 seconds, which an index would keep from being HOT
-- updates, and the cleanup scans a table of the sessions alive (M4/P4 review P5).

-- +goose Down
DROP TABLE edit_sessions;
