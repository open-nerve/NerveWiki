-- notebooks: an upgrade to an LLM Wiki under way (M11 design 4.1, 4.2): its id, its state, who started it and when,
-- set together or not at all, and only on a standard notebook. No foreign key on upgrade_id: the upgrades are the
-- wiki module's, which its purger purges leaf to root, and a notebook's row must not hold them (as 00048's
-- source_root_id). A program before this one knows no upgrade: an upgrading notebook reads as a standard one there.

-- +goose Up
ALTER TABLE notebooks ADD COLUMN upgrade_id uuid,
    ADD COLUMN upgrade_state text,
    ADD COLUMN upgrade_by_id uuid REFERENCES users,
    ADD COLUMN upgrade_at timestamptz,
    ADD CONSTRAINT notebooks_upgrade_check CHECK ((upgrade_state IS NULL) = (upgrade_id IS NULL)
        AND (upgrade_id IS NULL) = (upgrade_by_id IS NULL) AND (upgrade_id IS NULL) = (upgrade_at IS NULL)
        AND (upgrade_state IS NULL
            OR upgrade_state IN ('upgrading', 'reviewing', 'rolling_back') AND mode = 'standard'));

-- +goose Down
ALTER TABLE notebooks DROP CONSTRAINT notebooks_upgrade_check, DROP COLUMN upgrade_at, DROP COLUMN upgrade_by_id,
    DROP COLUMN upgrade_state, DROP COLUMN upgrade_id;
