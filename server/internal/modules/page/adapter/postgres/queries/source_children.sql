-- name: SourceChildrenWithin :many
-- The verified parents are unique and bounded. Each parent's live name-key
-- range can stop at max_rows without sorting its unbounded children; the
-- outer limit applies the same remaining budget across all parents.
-- The composite parent FK puts every child in its parent's notebook. Check
-- the notebook after the per-parent limit so its selectivity cannot turn
-- the ordered parent range into a notebook scan or a bitmap scan and sort.
-- Scalar subqueries keep the same bound as a runtime value: a custom plan
-- must not cost it as fetching all children when their estimate is too low.
SELECT child.id, child.notebook_id, child.parent_id, child.kind,
       child.name, child.name_key, child.sort_order,
       child.created_by_id, child.updated_by_id, child.created_at, child.updated_at
FROM unnest(sqlc.arg(parents)::uuid[]) AS p(id)
CROSS JOIN LATERAL (
  SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key,
         n.sort_order, n.created_by_id, n.updated_by_id, n.created_at, n.updated_at
  FROM nodes n
  WHERE n.parent_id = p.id AND n.parent_id IS NOT NULL
    AND n.deleted_at IS NULL
  ORDER BY n.name_key
  LIMIT (SELECT sqlc.arg(max_rows)::integer)
) child
WHERE notebook_id = sqlc.arg(notebook_id)
LIMIT (SELECT sqlc.arg(max_rows)::integer);
