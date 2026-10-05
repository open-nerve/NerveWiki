-- name: LinkTargetsByKeys :many
-- The pages not deleted of a notebook whose title key is one of keys, each with its path from the root (the link
-- index's candidates, M6/P3 design 3.3): a row a step of a page's path, its own the step 0 up. The chain stops at
-- a deleted node, and the bound at a chain that loops, both of which only a defect could make: such a path reaches
-- no root.
WITH RECURSIVE chain AS (
    SELECT n.id AS page_id, n.id, n.parent_id, n.name, n.name_key, 0 AS up
    FROM nodes n
    WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.name_key = ANY(sqlc.arg(keys)::text[])
        AND n.kind = 'page' AND n.deleted_at IS NULL
    UNION ALL
    SELECT c.page_id, p.id, p.parent_id, p.name, p.name_key, c.up + 1
    FROM chain c JOIN nodes p ON p.notebook_id = sqlc.arg(notebook_id) AND p.id = c.parent_id
    WHERE c.up < 64 AND p.deleted_at IS NULL
)
SELECT page_id, id, parent_id, name, name_key, up::integer AS up FROM chain ORDER BY page_id, up DESC;

-- name: LinkTargetsByIDs :many
-- The pages not deleted of a notebook among ids, each with its path from the root, as LinkTargetsByKeys gives
-- them.
WITH RECURSIVE chain AS (
    SELECT n.id AS page_id, n.id, n.parent_id, n.name, n.name_key, 0 AS up
    FROM nodes n
    WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.id = ANY(sqlc.arg(ids)::uuid[])
        AND n.kind = 'page' AND n.deleted_at IS NULL
    UNION ALL
    SELECT c.page_id, p.id, p.parent_id, p.name, p.name_key, c.up + 1
    FROM chain c JOIN nodes p ON p.notebook_id = sqlc.arg(notebook_id) AND p.id = c.parent_id
    WHERE c.up < 64 AND p.deleted_at IS NULL
)
SELECT page_id, id, parent_id, name, name_key, up::integer AS up FROM chain ORDER BY page_id, up DESC;

-- name: UnsetNameKeys :execrows
-- Each node's title key, before nervewiki reindex sets it anew (M6/P3 design 3.6), a value of its own that no
-- name's key is (a title has no control character): one statement sets every key, and the unique index of the
-- siblings' keys is checked row by row, so a key set to another's old one would clash before that one moves.
UPDATE nodes SET name_key = chr(1) || id::text
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

-- name: SetNameKeys :execrows
-- Each node's title key, taken anew from its name by nervewiki reindex (M6/P3 design 3.6): a derived column, so
-- neither updated_at nor a changeset moves.
UPDATE nodes n SET name_key = u.name_key
FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id, unnest(sqlc.arg(name_keys)::text[]) AS name_key) AS u
WHERE n.id = u.id AND n.deleted_at IS NULL;
