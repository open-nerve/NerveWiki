-- The ends that leave a session's row behind (M5 design 4.3; M5/P1 design 3.2): a session taken over by its
-- owner elsewhere, or unlocked by its notebook's admin, stays as a tombstone, so that its tab's next save or
-- heartbeat learns why instead of opening another session. A tombstone is no alive session; it keeps at least a
-- lease after its end, and the expired sessions' cleanup deletes it as any other. The other ends delete the row,
-- as in M4.

-- +goose Up
ALTER TABLE edit_sessions
    ADD COLUMN ended_reason text
        CONSTRAINT edit_sessions_ended_reason_check CHECK (ended_reason IN ('taken_over', 'unlocked')),
    -- Who took it over (its owner) or unlocked it (an admin of its notebook).
    ADD COLUMN ended_by_id uuid REFERENCES users,
    ADD COLUMN ended_at timestamptz,
    ADD CONSTRAINT edit_sessions_ended_check CHECK (
        ended_reason IS NULL AND ended_by_id IS NULL AND ended_at IS NULL
        OR ended_reason IS NOT NULL AND ended_by_id IS NOT NULL AND ended_at IS NOT NULL AND ended_at >= created_at
    );

-- +goose Down
ALTER TABLE edit_sessions
    DROP CONSTRAINT edit_sessions_ended_check,
    DROP COLUMN ended_at,
    DROP COLUMN ended_by_id,
    DROP COLUMN ended_reason;
