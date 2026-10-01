-- workspaces: the top of the hierarchy (v0.1 design 3.2; M2/P1 design 3.5).
-- The use cases write every column, the times from their clock.

-- +goose Up
CREATE TABLE workspaces (
    id uuid PRIMARY KEY,
    -- The address's segment, fixed once created. Only lower case: the domain
    -- refuses anything else rather than fold it.
    slug varchar(48) NOT NULL CONSTRAINT workspaces_slug_check CHECK (slug ~ '^[a-z0-9_-]{1,48}$'),
    -- The domain trims the name and refuses the characters that change how
    -- the text around it reads.
    name varchar(80) NOT NULL CONSTRAINT workspaces_name_check CHECK (name <> ''),
    created_by_id uuid NOT NULL REFERENCES users,
    -- Who last changed it; on a deleted workspace, who deleted it.
    updated_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- Soft delete: the clean-up purges it after the retention (v0.1 design 7.1).
    deleted_at timestamptz
);
-- A deleted workspace frees its slug at once.
CREATE UNIQUE INDEX workspaces_slug_key ON workspaces (slug) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE workspaces;
