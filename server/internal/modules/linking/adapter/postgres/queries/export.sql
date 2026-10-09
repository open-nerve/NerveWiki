-- name: LinkedPages :many
-- Of targets, the pages a link of a page of sources resolves to (M7/P5 design 3.8): an export writes a file for a page
-- without content that one leads to, so that Obsidian finds it as Nerve does. Each target is looked up on
-- page_links_resolved_id_source_id_idx by its id and the sources, the first link found enough: the links of pages
-- outside the sources, which a subtree's export leaves out, are not read. The sources grow with the notebook: the store
-- plans it with its arguments.
SELECT t.id::uuid AS id FROM unnest(sqlc.arg(targets)::uuid[]) AS t(id)
WHERE EXISTS (
    SELECT 1 FROM page_links l
    WHERE l.resolved_id = t.id AND l.source_id = ANY(sqlc.arg(sources)::uuid[]) AND NOT l.resolved_asset
);
