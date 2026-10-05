-- name: LockNotebook :exec
-- The notebook's lock of the index until the transaction ends (M6 design 4.5): the key pair's space is the
-- index's alone, apart from the single keys of goose and River.
SELECT pg_advisory_xact_lock(sqlc.arg(space)::integer, sqlc.arg(key)::integer);

-- name: InsertIndexedPage :exec
INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
VALUES (sqlc.arg(node_id), sqlc.arg(notebook_id), sqlc.arg(revision), sqlc.arg(extractor), sqlc.arg(frontmatter_valid));

-- name: InsertTags :exec
INSERT INTO page_tags (source_id, tag_key, notebook_id, tag, count)
SELECT sqlc.arg(source_id), u.tag_key, sqlc.arg(notebook_id), u.tag, u.count
FROM (
    SELECT unnest(sqlc.arg(tag_keys)::text[]) AS tag_key, unnest(sqlc.arg(tags)::text[]) AS tag,
        unnest(sqlc.arg(counts)::integer[]) AS count
) AS u;

-- name: InsertProperties :exec
-- Each value is JSON, as text.
INSERT INTO page_properties (source_id, position, notebook_id, key, value)
SELECT sqlc.arg(source_id), u.position, sqlc.arg(notebook_id), u.key, u.value::jsonb
FROM (
    SELECT unnest(sqlc.arg(positions)::integer[]) AS position, unnest(sqlc.arg(keys)::text[]) AS key,
        unnest(sqlc.arg(values)::text[]) AS value
) AS u;

-- name: InsertAliases :exec
INSERT INTO page_aliases (source_id, alias_key, notebook_id, alias)
SELECT sqlc.arg(source_id), u.alias_key, sqlc.arg(notebook_id), u.alias
FROM (SELECT unnest(sqlc.arg(alias_keys)::text[]) AS alias_key, unnest(sqlc.arg(aliases)::text[]) AS alias) AS u;

-- name: DeleteIndexedPages :exec
DELETE FROM indexed_pages WHERE node_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DeleteTagsOf :exec
DELETE FROM page_tags WHERE source_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DeletePropertiesOf :exec
DELETE FROM page_properties WHERE source_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DeleteAliasesOf :many
-- The keys of the aliases deleted, each once.
WITH deleted AS (
    DELETE FROM page_aliases WHERE source_id = ANY(sqlc.arg(ids)::uuid[]) RETURNING alias_key
)
SELECT DISTINCT alias_key FROM deleted ORDER BY alias_key;

-- name: DeleteNotebooksIndexedPages :exec
DELETE FROM indexed_pages WHERE notebook_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DeleteNotebooksTags :exec
DELETE FROM page_tags WHERE notebook_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DeleteNotebooksProperties :exec
DELETE FROM page_properties WHERE notebook_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DeleteNotebooksAliases :exec
DELETE FROM page_aliases WHERE notebook_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: AliasesByKeys :many
-- The pages of a notebook with an alias whose key is one of keys.
SELECT source_id, alias_key FROM page_aliases
WHERE notebook_id = sqlc.arg(notebook_id) AND alias_key = ANY(sqlc.arg(keys)::text[])
ORDER BY source_id, alias_key;

-- name: AliasKeysOf :many
-- The keys of the aliases of the pages ids, each once.
SELECT DISTINCT alias_key FROM page_aliases WHERE source_id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY alias_key;
