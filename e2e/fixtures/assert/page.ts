import type { Page } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import type { Database } from "../db";

// Database assertions of the page stories, by table. The page version and
// the API version of a story call the same function (v0.1 design 10.1).
// The times are compared in SQL, to the microsecond.

/**
 * nodes, page_contents, changesets, changeset_items, page_revisions: created is new, as the API answered it, created by
 * creatorId from client: an empty content at revision 1, and a changeset of its own with its version and its item,
 * which has it from nothing to its name, parent and order.
 */
export async function expectNewPage(db: Database, created: Page, creatorId: string, client: string): Promise<void> {
  const rows = await db.query(
    `SELECT n.notebook_id, n.parent_id, n.kind, n.name, n.created_by_id = $2 AND n.updated_by_id = $2 AS by_creator,
            n.created_at = $3::timestamptz AND n.updated_at = n.created_at AS at_answer, n.deleted_at IS NULL AS live,
            c.content, c.revision, c.byte_size, c.content_hash = sha256(''::bytea) AS hashed,
            c.updated_by_id = $2 AND c.updated_at = n.created_at AS content_by_creator
       FROM nodes n JOIN page_contents c ON c.node_id = n.id WHERE n.id = $1`,
    [created.id, creatorId, created.created_at]
  );
  expect(rows).toEqual([
    {
      notebook_id: created.notebook_id,
      parent_id: created.parent_id,
      kind: "page",
      name: created.name,
      by_creator: true,
      at_answer: true,
      live: true,
      content: "",
      revision: 1,
      byte_size: 0,
      hashed: true,
      content_by_creator: true,
    },
  ]);
  const changesets = await db.query(
    `SELECT s.kind, s.client, s.created_by_id = $2 AS by_creator, s.created_at = $3::timestamptz AS at_creation,
            i.before_name IS NULL AND i.after_name = $4 AND i.after_parent_id IS NOT DISTINCT FROM n.parent_id
              AND i.after_sort_order = n.sort_order AS item,
            r.base_revision IS NULL AND r.revision = 1 AND r.content = '' AS version
       FROM changesets s JOIN changeset_items i ON i.changeset_id = s.id JOIN nodes n ON n.id = i.node_id
       JOIN page_revisions r ON r.changeset_id = s.id
      WHERE i.node_id = $1 AND r.node_id = $1`,
    [created.id, creatorId, created.created_at, created.name]
  );
  expect(changesets).toEqual([
    { kind: "edit", client, by_creator: true, at_creation: true, item: true, version: true },
  ]);
}

/**
 * nodes, changesets and changeset_items: the node id is named name, renamed by renamerId in its last changeset, whose
 * time it holds and whose item has it from before to name in its place; the content is not touched.
 */
export async function expectRenamed(
  db: Database,
  id: string,
  before: string,
  name: string,
  renamerId: string
): Promise<void> {
  const rows = await db.query(
    `SELECT n.name, n.updated_by_id = $3 AS by_renamer, c.revision,
            s.created_by_id = $3 AND s.created_at = n.updated_at AS at_rename, i.before_name || ' > ' || i.after_name AS item,
            i.before_parent_id IS NOT DISTINCT FROM n.parent_id AND i.after_parent_id IS NOT DISTINCT FROM n.parent_id
              AND i.before_sort_order = n.sort_order AND i.after_sort_order = n.sort_order AS in_place
       FROM nodes n JOIN page_contents c ON c.node_id = n.id
       JOIN changeset_items i ON i.node_id = n.id JOIN changesets s ON s.id = i.changeset_id
      WHERE n.id = $1 AND n.name = $2
      ORDER BY s.created_at DESC, s.id DESC LIMIT 1`,
    [id, name, renamerId]
  );
  expect(rows).toEqual([
    { name, by_renamer: true, revision: 1, at_rename: true, item: `${before} > ${name}`, in_place: true },
  ]);
}

/**
 * nodes and what follows them, and changesets: the pages of the notebook notebookId, count of them, are deleted with
 * it, at its time, and so are their contents, items and versions and the notebook's changesets.
 */
export async function expectPagesDeletedWith(db: Database, notebookId: string, count: number): Promise<void> {
  const rows = await db.query(
    `SELECT (SELECT count(*)::int FROM nodes x WHERE x.notebook_id = n.id AND x.deleted_at = n.deleted_at) AS pages,
            (SELECT count(*)::int FROM nodes x WHERE x.notebook_id = n.id AND x.deleted_at IS DISTINCT FROM n.deleted_at) AS pages_apart,
            (SELECT count(*)::int FROM page_contents c JOIN nodes x ON x.id = c.node_id
              WHERE x.notebook_id = n.id AND c.deleted_at IS DISTINCT FROM n.deleted_at) AS contents_apart,
            (SELECT count(*)::int FROM changeset_items i JOIN nodes x ON x.id = i.node_id
              WHERE x.notebook_id = n.id AND i.deleted_at IS DISTINCT FROM n.deleted_at) AS items_apart,
            (SELECT count(*)::int FROM page_revisions r JOIN nodes x ON x.id = r.node_id
              WHERE x.notebook_id = n.id AND r.deleted_at IS DISTINCT FROM n.deleted_at) AS versions_apart,
            (SELECT count(*)::int FROM changesets s WHERE s.notebook_id = n.id AND s.deleted_at IS DISTINCT FROM n.deleted_at) AS changesets_apart
       FROM notebooks n WHERE n.id = $1 AND n.deleted_at IS NOT NULL`,
    [notebookId]
  );
  expect(rows).toEqual([
    { pages: count, pages_apart: 0, contents_apart: 0, items_apart: 0, versions_apart: 0, changesets_apart: 0 },
  ]);
}
