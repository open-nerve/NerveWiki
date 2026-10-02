-- name: CreateChangeset :exec
INSERT INTO changesets (id, notebook_id, kind, client, message, created_by_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(notebook_id), sqlc.arg(kind), sqlc.arg(client), sqlc.narg(message), sqlc.arg(by),
    sqlc.arg(now), sqlc.arg(now));

-- name: TouchChangeset :exec
-- An edit session's changeset, written again: its updated_at is its last write's time, which the notebook's
-- activity reads.
UPDATE changesets SET updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id);

-- name: RecordItem :exec
-- A node's change in a changeset: the first one inserts its before and after, a later one moves the after
-- on and keeps the before (M4 design 4). An item that deletes its node goes to the trash with it.
INSERT INTO changeset_items (id, changeset_id, node_id, before_parent_id, before_name, before_sort_order,
    after_parent_id, after_name, after_sort_order, created_at, updated_at, deleted_at)
VALUES (sqlc.arg(id), sqlc.arg(changeset_id), sqlc.arg(node_id), sqlc.narg(before_parent_id), sqlc.narg(before_name),
    sqlc.narg(before_sort_order), sqlc.narg(after_parent_id), sqlc.narg(after_name), sqlc.narg(after_sort_order),
    sqlc.arg(now), sqlc.arg(now), sqlc.narg(deleted_at))
ON CONFLICT (changeset_id, node_id) DO UPDATE
SET after_parent_id = excluded.after_parent_id, after_name = excluded.after_name,
    after_sort_order = excluded.after_sort_order, updated_at = excluded.updated_at, deleted_at = excluded.deleted_at;

-- name: RecordRevision :exec
-- A page's content as a changeset leaves it: a later write of the same changeset updates the row, keeping
-- the base it started from.
INSERT INTO page_revisions (id, changeset_id, node_id, base_revision, revision, content, content_hash, byte_size,
    created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(changeset_id), sqlc.arg(node_id), sqlc.narg(base_revision), sqlc.arg(revision),
    sqlc.arg(content), sqlc.arg(content_hash), sqlc.arg(byte_size), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (changeset_id, node_id) DO UPDATE
SET revision = excluded.revision, content = excluded.content, content_hash = excluded.content_hash,
    byte_size = excluded.byte_size, updated_at = excluded.updated_at;
