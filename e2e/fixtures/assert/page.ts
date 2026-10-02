import type { Page, TreeNode } from "@nervewiki/api-client";
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

/**
 * nodes, changesets and changeset_items: the node id is where moved puts it, moved there from under fromParentId by
 * moverId in its last changeset, whose time it holds and whose item has it from that parent to its place, its name
 * unchanged.
 */
export async function expectMoved(
  db: Database,
  moved: TreeNode,
  fromParentId: string | null,
  moverId: string
): Promise<void> {
  const rows = await db.query(
    `SELECT n.parent_id, n.updated_by_id = $3 AS by_mover, s.created_by_id = $3 AND s.created_at = n.updated_at AS at_move,
            i.before_parent_id IS NOT DISTINCT FROM $2::uuid AS from_parent,
            i.after_parent_id IS NOT DISTINCT FROM n.parent_id AND i.after_sort_order = n.sort_order AS to_place,
            i.before_name = n.name AND i.after_name = n.name AS same_name
       FROM nodes n JOIN changeset_items i ON i.node_id = n.id JOIN changesets s ON s.id = i.changeset_id
      WHERE n.id = $1
      ORDER BY s.created_at DESC, s.id DESC LIMIT 1`,
    [moved.id, fromParentId, moverId]
  );
  expect(rows).toEqual([
    { parent_id: moved.parent_id, by_mover: true, at_move: true, from_parent: true, to_place: true, same_name: true },
  ]);
}

/**
 * nodes, page_contents, page_revisions, changeset_items, changesets: the subtree of rootId, the pages ids, is deleted
 * by deleterId at one time, with what follows them, and the changeset that deleted it holds an item of each page with
 * no after, deleted with it.
 */
export async function expectSubtreeDeleted(
  db: Database,
  rootId: string,
  ids: string[],
  deleterId: string
): Promise<void> {
  const rows = await db.query(
    `SELECT (SELECT count(*)::int FROM nodes x WHERE x.id = ANY($2::uuid[]) AND x.deleted_at = r.deleted_at
              AND x.updated_at = r.deleted_at AND x.updated_by_id = $3) AS pages,
            (SELECT count(*)::int FROM page_contents c WHERE c.node_id = ANY($2::uuid[]) AND c.deleted_at = r.deleted_at) AS contents,
            (SELECT count(*)::int FROM page_revisions v WHERE v.node_id = ANY($2::uuid[])
              AND v.deleted_at IS DISTINCT FROM r.deleted_at) AS versions_apart,
            (SELECT count(*)::int FROM changeset_items i WHERE i.node_id = ANY($2::uuid[])
              AND i.deleted_at IS DISTINCT FROM r.deleted_at) AS items_apart,
            (SELECT count(*)::int FROM changeset_items i JOIN changesets s ON s.id = i.changeset_id
              WHERE i.node_id = ANY($2::uuid[]) AND i.after_name IS NULL AND i.deleted_at = r.deleted_at
              AND s.created_at = r.deleted_at AND s.created_by_id = $3) AS deletions
       FROM nodes r WHERE r.id = $1 AND r.deleted_at IS NOT NULL`,
    [rootId, ids, deleterId]
  );
  expect(rows).toEqual([
    { pages: ids.length, contents: ids.length, versions_apart: 0, items_apart: 0, deletions: ids.length },
  ]);
}

/**
 * page_contents: the page written, as the API answered its write, holds content, its bytes' size and SHA-256 at the
 * answer's revision, written by writerId at the answer's time.
 */
export async function expectContentWritten(
  db: Database,
  written: Page,
  content: string,
  writerId: string
): Promise<void> {
  const rows = await db.query(
    `SELECT c.content = $2 AS content, c.revision, c.byte_size = octet_length(convert_to($2, 'UTF8')) AS sized,
            c.content_hash = sha256(convert_to($2, 'UTF8')) AS hashed, c.updated_by_id = $3 AS by_writer,
            c.updated_at = $4::timestamptz AS at_answer
       FROM page_contents c WHERE c.node_id = $1 AND c.deleted_at IS NULL`,
    [written.id, content, writerId, written.content_updated_at]
  );
  expect(rows).toEqual([
    { content: true, revision: written.revision, sized: true, hashed: true, by_writer: true, at_answer: true },
  ]);
}

/**
 * edit_sessions, changesets, page_revisions: the edit session sessionId's writes to the page pageId are one changeset,
 * the session's, which holds one version of the page, from base to revision with its content, and whose last write is
 * the page content's.
 */
export async function expectOneSessionRevision(
  db: Database,
  sessionId: string,
  pageId: string,
  base: number,
  revision: number
): Promise<void> {
  const rows = await db.query(
    `SELECT s.revision AS session_revision,
            (SELECT count(*)::int FROM page_revisions r WHERE r.changeset_id = s.changeset_id) AS versions,
            (SELECT count(*)::int FROM page_revisions r WHERE r.changeset_id = s.changeset_id AND r.node_id = $2
               AND r.base_revision = $3 AND r.revision = $4 AND r.content = c.content
               AND r.content_hash = c.content_hash) AS the_version,
            x.updated_at = c.updated_at AS last_write
       FROM edit_sessions s JOIN changesets x ON x.id = s.changeset_id JOIN page_contents c ON c.node_id = s.node_id
      WHERE s.id = $1 AND s.node_id = $2`,
    [sessionId, pageId, base, revision]
  );
  expect(rows).toEqual([{ session_revision: revision, versions: 1, the_version: 1, last_write: true }]);
}

/** edit_sessions: the edit session id is gone. */
export async function expectSessionGone(db: Database, id: string): Promise<void> {
  const [row] = await db.query<{ n: number }>("SELECT count(*)::int AS n FROM edit_sessions WHERE id = $1", [id]);
  expect(row?.n).toBe(0);
}
