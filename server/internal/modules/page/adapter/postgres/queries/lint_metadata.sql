-- name: LintNodeMetadata :many
SELECT n.id, n.parent_id, n.kind, n.name_key, n.created_at,
       COALESCE(c.byte_size, 0)::bigint AS page_bytes,
       (c.node_id IS NOT NULL)::boolean AS has_content,
       (n.kind = 'page' OR
         (strpos(reverse(n.name), '.') > 1 AND
          strpos(reverse(n.name), '.') < char_length(n.name)))::boolean AS linkable
FROM nodes n
LEFT JOIN LATERAL (
    SELECT c.node_id, c.byte_size
    FROM page_contents c
    WHERE c.node_id = n.id AND c.deleted_at IS NULL
    LIMIT 1
) c ON n.kind = 'page'
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL;
