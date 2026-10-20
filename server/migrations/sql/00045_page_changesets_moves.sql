-- changesets.renames, moves: whether a changeset renames a node, or moves one to another parent, of all its items
-- (M9/P3 design 3.20): the web's revert warns that links written to the new place stay. A unit sets them as it
-- ends, from its merged changes (page/app/unit.go); until 00045 a read took them from the first 1,000 items.

-- +goose Up
ALTER TABLE changesets ADD COLUMN renames boolean NOT NULL DEFAULT false,
    ADD COLUMN moves boolean NOT NULL DEFAULT false;
-- The changesets so far, each from all its items: a changeset's units each merged their changes into its items,
-- which hold them as the read did.
UPDATE changesets c SET renames = m.renames, moves = m.moves
FROM (SELECT i.changeset_id,
             coalesce(bool_or(i.before_name <> i.after_name), false) AS renames,
             coalesce(bool_or(i.before_name IS NOT NULL AND i.after_name IS NOT NULL
                 AND i.before_parent_id IS DISTINCT FROM i.after_parent_id), false) AS moves
      FROM changeset_items i GROUP BY i.changeset_id) m
WHERE m.changeset_id = c.id AND (m.renames OR m.moves);

-- +goose Down
ALTER TABLE changesets DROP COLUMN renames, DROP COLUMN moves;
