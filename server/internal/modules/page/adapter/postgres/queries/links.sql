-- name: LinkTargetsByKeys :many
-- The pages and attachments not deleted of a notebook whose title key is one of keys, each with its path from the
-- root and its kind (the link index's candidates, M6/P3 design 3.3; M7/P3 design 4.3): a row a step of a node's path,
-- its own the step 0 up, each with its node's kind. The chain stops at
-- a deleted node, and the bound at a chain that loops, both of which only a defect could make: such a path reaches
-- no root. Each step up reads its parent by its key alone, LIMIT 1 keeping the planner from joining the notebook's
-- nodes instead, its notebook and deletion checked after: without statistics (an import, a restore) a step read by
-- the notebook was the chain's rows times the notebook's (M6 closeout A-M2).
WITH RECURSIVE chain AS (
    SELECT n.id AS page_id, n.id, n.parent_id, n.name, n.name_key, n.kind, 0 AS up
    FROM nodes n
    WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.name_key = ANY(sqlc.arg(keys)::text[])
        AND n.deleted_at IS NULL
    UNION ALL
    SELECT c.page_id, p.id, p.parent_id, p.name, p.name_key, p.kind, c.up + 1
    FROM chain c CROSS JOIN LATERAL (
        SELECT p.id, p.parent_id, p.name, p.name_key, p.kind, p.notebook_id, p.deleted_at FROM nodes p
        WHERE p.id = c.parent_id
        LIMIT 1
    ) p
    WHERE c.up < 64 AND p.notebook_id = sqlc.arg(notebook_id) AND p.deleted_at IS NULL
)
SELECT page_id, id, parent_id, name, name_key, kind, up::integer AS up FROM chain ORDER BY page_id, up DESC;

-- name: LinkTargetsByIDs :many
-- The pages and attachments not deleted of a notebook among ids, each with its path from the root and its kind, as
-- LinkTargetsByKeys gives them.
WITH RECURSIVE chain AS (
    SELECT n.id AS page_id, n.id, n.parent_id, n.name, n.name_key, n.kind, 0 AS up
    FROM nodes n
    WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.id = ANY(sqlc.arg(ids)::uuid[])
        AND n.deleted_at IS NULL
    UNION ALL
    SELECT c.page_id, p.id, p.parent_id, p.name, p.name_key, p.kind, c.up + 1
    FROM chain c CROSS JOIN LATERAL (
        SELECT p.id, p.parent_id, p.name, p.name_key, p.kind, p.notebook_id, p.deleted_at FROM nodes p
        WHERE p.id = c.parent_id
        LIMIT 1
    ) p
    WHERE c.up < 64 AND p.notebook_id = sqlc.arg(notebook_id) AND p.deleted_at IS NULL
)
SELECT page_id, id, parent_id, name, name_key, kind, up::integer AS up FROM chain ORDER BY page_id, up DESC;

-- name: AttachmentsByIDs :many
-- The attachments not deleted of a notebook among ids, each with its path from the root, as LinkTargetsByIDs gives
-- them, and on its own step the number of the notebook's attachments not deleted with its title key, itself among
-- them, counted to 2: whether another has it (M7/P3 design 4.6). One statement, so one snapshot, which an
-- attachment's link is written from.
WITH RECURSIVE chain AS (
    SELECT n.id AS page_id, n.id, n.parent_id, n.name, n.name_key, n.kind, 0 AS up,
        (SELECT count(*) FROM (
            SELECT 1 FROM nodes o
            WHERE o.notebook_id = n.notebook_id AND o.name_key = n.name_key AND o.kind = 'asset' AND o.deleted_at IS NULL
            LIMIT 2
        ) o) AS alike
    FROM nodes n
    WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.id = ANY(sqlc.arg(ids)::uuid[]) AND n.kind = 'asset'
        AND n.deleted_at IS NULL
    UNION ALL
    SELECT c.page_id, p.id, p.parent_id, p.name, p.name_key, p.kind, c.up + 1, 0::bigint
    FROM chain c CROSS JOIN LATERAL (
        SELECT p.id, p.parent_id, p.name, p.name_key, p.kind, p.notebook_id, p.deleted_at FROM nodes p
        WHERE p.id = c.parent_id
        LIMIT 1
    ) p
    WHERE c.up < 64 AND p.notebook_id = sqlc.arg(notebook_id) AND p.deleted_at IS NULL
)
SELECT page_id, id, parent_id, name, name_key, kind, up::integer AS up, alike::integer AS alike FROM chain
ORDER BY page_id, up DESC;

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
