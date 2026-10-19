-- name: CreateChangeset :exec
INSERT INTO changesets (id, notebook_id, kind, reverts_id, client, message, created_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(notebook_id), sqlc.arg(kind), sqlc.narg(reverts_id), sqlc.arg(client), sqlc.narg(message),
    sqlc.arg(by), sqlc.arg(now), sqlc.arg(now));

-- name: TouchChangeset :exec
-- An edit session's changeset, written again: its updated_at is its last write's time, which the notebook's
-- activity reads.
UPDATE changesets SET updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id);

-- name: RecordItems :exec
-- A step's changes of nodes in a changeset, one statement (M8/P2 design 3.3): each node's first one inserts its before
-- and after, a later one moves the after on and keeps the before, restored and revision (M4 design 4). An item that
-- deletes its node goes to the trash with it, with its page's revision (M8/P3 design 3.4): the content the unit's
-- deletion, which wrote before its items, put in the trash at its time (none for an attachment), read by its key alone
-- and its time compared after: a lookup by the time too may go through the partial index on deleted_at, every row the
-- deletion put there for each item, and a deletion of 45,000 pages took 92 s (M8/P4A); a merge into a deletion takes
-- the deletion's (a rename, then a deletion, in one unit), any other keeps the first's. A
-- step changes each node once, so no row is reached twice. The arrays are the items' columns, a parent at the root and
-- a state there is none of marked as linking's SetResolutions marks none: the zero id; has_before and has_after;
-- revision 0 for none.
INSERT INTO changeset_items (id, changeset_id, node_id, before_parent_id, before_name, before_sort_order,
    after_parent_id, after_name, after_sort_order, restored, revision, created_at, updated_at, deleted_at)
SELECT u.id, sqlc.arg(changeset_id), u.node_id,
    CASE WHEN u.has_before THEN NULLIF(u.before_parent_id, '00000000-0000-0000-0000-000000000000'::uuid) END,
    CASE WHEN u.has_before THEN u.before_name END, CASE WHEN u.has_before THEN u.before_sort_order END,
    CASE WHEN u.has_after THEN NULLIF(u.after_parent_id, '00000000-0000-0000-0000-000000000000'::uuid) END,
    CASE WHEN u.has_after THEN u.after_name END, CASE WHEN u.has_after THEN u.after_sort_order END,
    u.restored,
    CASE WHEN u.has_after THEN NULLIF(u.revision, 0)
        ELSE (SELECT CASE WHEN c.deleted_at = sqlc.arg(now)::timestamptz THEN c.revision END FROM page_contents c
            WHERE c.node_id = u.node_id) END,
    sqlc.arg(now), sqlc.arg(now),
    CASE WHEN u.has_after THEN NULL ELSE sqlc.arg(now)::timestamptz END
FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id, unnest(sqlc.arg(node_ids)::uuid[]) AS node_id,
        unnest(sqlc.arg(has_befores)::boolean[]) AS has_before, unnest(sqlc.arg(before_parent_ids)::uuid[]) AS before_parent_id,
        unnest(sqlc.arg(before_names)::text[]) AS before_name, unnest(sqlc.arg(before_sort_orders)::float8[]) AS before_sort_order,
        unnest(sqlc.arg(has_afters)::boolean[]) AS has_after, unnest(sqlc.arg(after_parent_ids)::uuid[]) AS after_parent_id,
        unnest(sqlc.arg(after_names)::text[]) AS after_name, unnest(sqlc.arg(after_sort_orders)::float8[]) AS after_sort_order,
        unnest(sqlc.arg(restored)::boolean[]) AS restored, unnest(sqlc.arg(revisions)::integer[]) AS revision) AS u
ON CONFLICT (changeset_id, node_id) DO UPDATE
SET after_parent_id = excluded.after_parent_id, after_name = excluded.after_name,
    after_sort_order = excluded.after_sort_order, updated_at = excluded.updated_at, deleted_at = excluded.deleted_at,
    revision = CASE WHEN excluded.after_name IS NULL THEN excluded.revision ELSE changeset_items.revision END;

-- name: RecordRevision :execrows
-- A page's content as a changeset leaves it: a later write of the same changeset updates the row, keeping
-- the base it started from, when it goes on from the row's revision. One on another's write would make the row
-- hold that write's revisions too (M8/P1 review C4): it updates nothing, and the unit fails.
INSERT INTO page_revisions (id, changeset_id, node_id, base_revision, revision, content, content_hash, byte_size,
    created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(changeset_id), sqlc.arg(node_id), sqlc.narg(base_revision), sqlc.arg(revision),
    sqlc.arg(content), sqlc.arg(content_hash), sqlc.arg(byte_size), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (changeset_id, node_id) DO UPDATE
SET revision = excluded.revision, content = excluded.content, content_hash = excluded.content_hash,
    byte_size = excluded.byte_size, updated_at = excluded.updated_at
WHERE page_revisions.revision = excluded.base_revision;

-- name: DropUnchangedItems :exec
-- A batch's items whose node is where it was, its parent, name and order (M9/P3 design 3.5.4): a node moved and moved
-- back, merged into one item of the same before and after, which the history would tell as reordered.
DELETE FROM changeset_items
WHERE changeset_id = sqlc.arg(changeset_id) AND before_name IS NOT NULL AND after_name IS NOT NULL
    AND before_parent_id IS NOT DISTINCT FROM after_parent_id AND before_name = after_name
    AND before_sort_order = after_sort_order;

-- name: DropEmptyChangeset :execrows
-- The changeset when it holds no item and no version, once DropUnchangedItems ran: a batch that left nothing.
DELETE FROM changesets c
WHERE c.id = sqlc.arg(id)
    AND NOT EXISTS (SELECT 1 FROM changeset_items i WHERE i.changeset_id = c.id)
    AND NOT EXISTS (SELECT 1 FROM page_revisions r WHERE r.changeset_id = c.id);

-- name: LockChangeset :one
-- A changeset a unit merges into (M7/P6 design 3.2), locked until the unit ends: an import's later units write in
-- its first's. reverted tells whether a revert of it was made, through the partial index on reverts_id: one reverted
-- takes no more (M8/P3 design 3.7).
SELECT c.id, c.notebook_id, c.kind, c.reverts_id, c.client, c.created_by_id,
    EXISTS (SELECT 1 FROM changesets r WHERE r.reverts_id = c.id AND r.deleted_at IS NULL) AS reverted
FROM changesets c
WHERE c.id = sqlc.arg(id) AND c.deleted_at IS NULL
FOR NO KEY UPDATE OF c;
