-- The clients the changesets and the edit sessions take, written without the locale (M9 design 4.2; the M4/P1
-- hand-off's items 1-2): web, api, cli, or mcp: and 1 to 128 characters none of which is a control character (Cc:
-- U+0000-U+001F, U+007F-U+009F; text holds no NUL), a line or paragraph separator (Zl, Zp: U+2028, U+2029) or a
-- bidirectional control (Bidi_Control: U+061C, U+200E, U+200F, U+202A-U+202E, U+2066-U+2069): shared.Unshowable,
-- which page's Client.Valid checks. [:cntrl:] followed the database's LC_CTYPE. The ranges are the regular
-- expression's own escapes; U+2028-U+202E is Zl, Zp and the five embeddings and overrides at once.
-- Each table is scanned once under the ALTER's lock: a changeset is a unit's write, an edit session a page open.

-- +goose Up
ALTER TABLE changesets DROP CONSTRAINT changesets_client_check,
    ADD CONSTRAINT changesets_client_check CHECK (client IN ('web', 'api', 'cli')
        OR client ~ '^mcp:[^\u0001-\u001f\u007f-\u009f\u061c\u200e\u200f\u2028-\u202e\u2066-\u2069]{1,128}$');
ALTER TABLE edit_sessions DROP CONSTRAINT edit_sessions_client_check,
    ADD CONSTRAINT edit_sessions_client_check CHECK (client IN ('web', 'api', 'cli')
        OR client ~ '^mcp:[^\u0001-\u001f\u007f-\u009f\u061c\u200e\u200f\u2028-\u202e\u2066-\u2069]{1,128}$');

-- +goose Down
ALTER TABLE edit_sessions DROP CONSTRAINT edit_sessions_client_check,
    ADD CONSTRAINT edit_sessions_client_check CHECK (client IN ('web', 'api', 'cli') OR client ~ '^mcp:[^[:cntrl:]]{1,128}$');
ALTER TABLE changesets DROP CONSTRAINT changesets_client_check,
    ADD CONSTRAINT changesets_client_check CHECK (client IN ('web', 'api', 'cli') OR client ~ '^mcp:[^[:cntrl:]]{1,128}$');
