import type { Page, TreeNode } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import type { Database } from "../db";

// Database assertions of the page stories, by table. The page version and
// the API version of a story call the same function (v0.1 design 10.1).
// The times are compared in SQL, to the microsecond.

/**
 * nodes, page_contents, changesets, changeset_items, page_revisions: created is new, as the API answered it, or as a
 * test builds it from an MCP write's answer, created by creatorId from client: content (none unless given) at revision
 * 1, with its size and SHA-256, and a changeset with its version and its item, which has it from nothing to its name,
 * parent and order.
 */
export async function expectNewPage(
  db: Database,
  created: Pick<Page, "id" | "notebook_id" | "parent_id" | "name" | "created_at">,
  creatorId: string,
  client: string,
  content = ""
): Promise<void> {
  const rows = await db.query(
    `SELECT n.notebook_id, n.parent_id, n.kind, n.name, n.created_by_id = $2 AND n.updated_by_id = $2 AS by_creator,
            n.created_at = $3::timestamptz AND n.updated_at = n.created_at AS at_answer, n.deleted_at IS NULL AS live,
            c.content, c.revision, c.byte_size, c.content_hash = sha256(convert_to($4, 'UTF8')) AS hashed,
            c.updated_by_id = $2 AND c.updated_at = n.created_at AS content_by_creator
       FROM nodes n JOIN page_contents c ON c.node_id = n.id WHERE n.id = $1`,
    [created.id, creatorId, created.created_at, content]
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
      content,
      revision: 1,
      byte_size: Buffer.byteLength(content),
      hashed: true,
      content_by_creator: true,
    },
  ]);
  const changesets = await db.query(
    `SELECT s.kind, s.client, s.created_by_id = $2 AS by_creator, s.created_at = $3::timestamptz AS at_creation,
            i.before_name IS NULL AND i.after_name = $4 AND i.after_parent_id IS NOT DISTINCT FROM n.parent_id
              AND i.after_sort_order = n.sort_order AS item,
            r.base_revision IS NULL AND r.revision = 1 AND r.content = $5 AS version
       FROM changesets s JOIN changeset_items i ON i.changeset_id = s.id JOIN nodes n ON n.id = i.node_id
       JOIN page_revisions r ON r.changeset_id = s.id
      WHERE i.node_id = $1 AND r.node_id = $1`,
    [created.id, creatorId, created.created_at, created.name, content]
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

/** Where a node is: its parent (null at the root) and its order among its siblings. */
export interface Place {
  parentId: string | null;
  sortOrder: number;
}

/** nodes: where the node id is now. */
export async function placeOf(db: Database, id: string): Promise<Place> {
  const [row] = await db.query<{ parent_id: string | null; sort_order: number }>(
    "SELECT parent_id, sort_order FROM nodes WHERE id = $1",
    [id]
  );
  if (!row) {
    throw new Error(`no node ${id}`);
  }
  return { parentId: row.parent_id, sortOrder: row.sort_order };
}

/**
 * nodes, changesets and changeset_items: the node moved, as the API answered its move, or as a story builds it from
 * an MCP answer, is where moved puts it, moved there from the place from (see placeOf) by moverId in its last
 * changeset, whose time it holds and whose item has it from that place to its new one, its name unchanged.
 */
export async function expectMoved(
  db: Database,
  moved: Pick<TreeNode, "id" | "parent_id">,
  from: Place,
  moverId: string
): Promise<void> {
  const rows = await db.query(
    `SELECT n.parent_id, n.updated_by_id = $3 AS by_mover, s.created_by_id = $3 AND s.created_at = n.updated_at AS at_move,
            i.before_parent_id IS NOT DISTINCT FROM $2::uuid AND i.before_sort_order = $4::float8 AS from_place,
            i.after_parent_id IS NOT DISTINCT FROM n.parent_id AND i.after_sort_order = n.sort_order AS to_place,
            i.before_name = n.name AND i.after_name = n.name AS same_name
       FROM nodes n JOIN changeset_items i ON i.node_id = n.id JOIN changesets s ON s.id = i.changeset_id
      WHERE n.id = $1
      ORDER BY s.created_at DESC, s.id DESC LIMIT 1`,
    [moved.id, from.parentId, moverId, from.sortOrder]
  );
  expect(rows).toEqual([
    { parent_id: moved.parent_id, by_mover: true, at_move: true, from_place: true, to_place: true, same_name: true },
  ]);
}

/**
 * nodes, changesets, changeset_items, page_revisions: the node id was moved and renamed in one step, by moverId in its
 * last changeset, from from (its parent, order and name, as placeOf and the test have them) to to, which the test gives
 * rather than reads from the node: that changeset's item has it from the one to the other, the node's row is there
 * now, at the changeset's time, which is at when given, and the changeset holds no version of it.
 */
export async function expectRelocated(
  db: Database,
  id: string,
  from: Place & { name: string },
  to: { parentId: string | null; name: string },
  moverId: string,
  at?: string
): Promise<void> {
  const rows = await db.query(
    `SELECT n.parent_id AS "parentId", n.name, n.updated_by_id = $6 AS by_mover,
            s.created_by_id = $6 AND s.created_at = n.updated_at AS at_move,
            ($7::timestamptz IS NULL OR s.created_at = $7::timestamptz) AS at_answer,
            i.before_parent_id IS NOT DISTINCT FROM $2::uuid AND i.before_sort_order = $3::float8 AND i.before_name = $4
              AS from_place,
            i.after_parent_id IS NOT DISTINCT FROM $5::uuid AND i.after_name = $8
              AND i.after_parent_id IS NOT DISTINCT FROM n.parent_id AND i.after_name = n.name
              AND i.after_sort_order = n.sort_order AS to_place,
            NOT EXISTS (SELECT 1 FROM page_revisions r WHERE r.changeset_id = s.id AND r.node_id = n.id) AS content_untouched
       FROM nodes n JOIN changeset_items i ON i.node_id = n.id JOIN changesets s ON s.id = i.changeset_id
      WHERE n.id = $1
      ORDER BY s.created_at DESC, s.id DESC LIMIT 1`,
    [id, from.parentId, from.sortOrder, from.name, to.parentId, moverId, at ?? null, to.name]
  );
  expect(rows).toEqual([
    {
      parentId: to.parentId,
      name: to.name,
      by_mover: true,
      at_move: true,
      at_answer: true,
      from_place: true,
      to_place: true,
      content_untouched: true,
    },
  ]);
}

/**
 * What a changeset holds, as expectChangesetOf checks it: its client and author, its items' nodes, its versions' pages;
 * and, when given, its message (null for none) and kind (M9/P3 design 4.1).
 */
export interface ChangesetOf {
  client: string;
  byId: string;
  items: string[];
  versions: string[];
  message?: string | null;
  kind?: string;
}

/**
 * changesets, changeset_items, page_revisions: the changeset id is of want's client by its author, and holds an item of
 * exactly want's items and a version of exactly its versions, in any order: what one MCP write or move did, the
 * parents it created and the pages whose links it wrote again among them; of want's message and kind when it gives
 * them.
 */
export async function expectChangesetOf(db: Database, id: string, want: ChangesetOf): Promise<void> {
  const rows = await db.query<{
    client: string;
    byId: string;
    items: string[];
    versions: string[];
    message: string | null;
    kind: string;
  }>(
    `SELECT s.client, s.created_by_id AS "byId",
            ARRAY(SELECT i.node_id::text FROM changeset_items i WHERE i.changeset_id = s.id ORDER BY i.node_id::text) AS items,
            ARRAY(SELECT r.node_id::text FROM page_revisions r WHERE r.changeset_id = s.id ORDER BY r.node_id::text) AS versions,
            s.message, s.kind
       FROM changesets s WHERE s.id = $1`,
    [id]
  );
  // The message and the kind are compared when want gives them.
  const compared = (changeset: Omit<(typeof rows)[number], "message" | "kind"> & Partial<(typeof rows)[number]>) => {
    const out: Record<string, unknown> = {
      client: changeset.client,
      byId: changeset.byId,
      items: changeset.items,
      versions: changeset.versions,
    };
    if (want.message !== undefined) {
      out.message = changeset.message;
    }
    if (want.kind !== undefined) {
      out.kind = changeset.kind;
    }
    return out;
  };
  expect(rows.map(compared)).toEqual([
    compared({ ...want, items: want.items.toSorted(), versions: want.versions.toSorted() }),
  ]);
}

/**
 * The names a revert puts nodes aside under (M9/P3 design 3.17): a node's name, " ~", 8 or 32 hex digits of its id, the
 * 32 maybe with a count ("-2"), and an attachment's extension.
 */
const asideName = String.raw` ~[0-9a-f]{8}([0-9a-f]{24}(-[0-9]+)?)?(\.[^./]+)?$`;

/**
 * nodes, changeset_items: no node of the notebook notebookId, alive or in the trash, has a name a revert puts aside,
 * nor does an item of its changesets, before or after (M9/P3 design 4.1). The query's regex is first shown to match
 * names made as a revert makes them, lest it match nothing at all (M9/P3 review C-N7).
 */
export async function expectNoAsideNames(db: Database, notebookId: string): Promise<void> {
  const made = ["Plan ~1a2b3c4d", `Plan ~${"0123456789abcdef".repeat(2)}-2`, "photo ~1a2b3c4d.png"];
  const matched = await db.query<{ matches: boolean }>(`SELECT name ~ $1 AS matches FROM unnest($2::text[]) AS name`, [
    asideName,
    made,
  ]);
  expect(
    matched.map((row) => row.matches),
    "the regex matches names put aside"
  ).toEqual(made.map(() => true));
  const rows = await db.query(
    `SELECT name FROM nodes WHERE notebook_id = $1 AND name ~ $2
     UNION ALL
     SELECT coalesce(i.before_name, '') || ' / ' || coalesce(i.after_name, '') FROM changeset_items i
       JOIN changesets s ON s.id = i.changeset_id
      WHERE s.notebook_id = $1 AND (i.before_name ~ $2 OR i.after_name ~ $2)`,
    [notebookId, asideName]
  );
  expect(rows).toEqual([]);
}

/**
 * nodes, page_contents, page_revisions, changeset_items, changesets, edit_sessions: the subtree of rootId, the pages
 * ids, is deleted by deleterId at one time, with what follows them, and the changeset that deleted it holds an item of
 * each page with no after, deleted with it; no edit session of them is left.
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
              AND s.created_at = r.deleted_at AND s.created_by_id = $3) AS deletions,
            (SELECT count(*)::int FROM edit_sessions e WHERE e.node_id = ANY($2::uuid[])) AS sessions
       FROM nodes r WHERE r.id = $1 AND r.deleted_at IS NOT NULL`,
    [rootId, ids, deleterId]
  );
  expect(rows).toEqual([
    { pages: ids.length, contents: ids.length, versions_apart: 0, items_apart: 0, deletions: ids.length, sessions: 0 },
  ]);
}

/** The counts a deletion keeps on its root (M8/P2 design 3.2): its group's pages, attachments and levels of pages. */
export interface TrashCounts {
  pages: number;
  assets: number;
  height: number;
}

/**
 * nodes: the node rootId is a trash item's root, deleted, with the counts of its group, which are its alone: the
 * nodes under it deleted at its time, down from it, as the server's invariants count them (M8/P4 design 4.5), not
 * every node of the notebook deleted then, as a batch of a revert deletes many items at once. The other nodes of its
 * group are no roots and have none, and its pages, attachments and levels of pages are as its counts say. alone, its
 * deletion deleted nothing else of the notebook at its time: a deletion of the tree's, one item (P4B review C-N2).
 */
export async function expectTrashRoot(
  db: Database,
  rootId: string,
  counts: TrashCounts,
  { alone = false }: { alone?: boolean } = {}
): Promise<void> {
  const rows = await db.query(
    `WITH RECURSIVE g AS (
       SELECT r.id, r.kind, 1 AS level, r.deleted_at FROM nodes r WHERE r.id = $1
       UNION ALL
       SELECT x.id, x.kind, g.level + 1, g.deleted_at
         FROM g JOIN nodes x ON x.parent_id = g.id AND x.deleted_at = g.deleted_at
     )
     SELECT n.trash_root, n.trash_pages AS pages, n.trash_assets AS assets, n.trash_height AS height,
            (SELECT count(*)::int FROM g WHERE g.kind = 'page') AS grouped_pages,
            (SELECT count(*)::int FROM g WHERE g.kind = 'asset') AS grouped_assets,
            (SELECT coalesce(max(g.level) FILTER (WHERE g.kind = 'page'), 0)::int FROM g) AS grouped_height,
            (SELECT count(*)::int FROM g JOIN nodes x ON x.id = g.id
              WHERE x.id <> n.id AND (x.trash_root OR x.trash_pages IS NOT NULL OR x.trash_assets IS NOT NULL
                OR x.trash_height IS NOT NULL)) AS others_marked,
            (SELECT count(*)::int FROM nodes x WHERE x.notebook_id = n.notebook_id AND x.deleted_at = n.deleted_at)
              AS at_its_time
       FROM nodes n WHERE n.id = $1 AND n.deleted_at IS NOT NULL`,
    [rootId]
  );
  expect(rows).toEqual([
    {
      trash_root: true,
      ...counts,
      grouped_pages: counts.pages,
      grouped_assets: counts.assets,
      grouped_height: counts.height,
      others_marked: 0,
      at_its_time: alone ? counts.pages + counts.assets : expect.any(Number),
    },
  ]);
}

/** Where a restore put a trash item's root: its parent (null at the root) and its name, numbered or not. */
export interface RestoredRoot {
  parentId: string | null;
  name: string;
}

/**
 * nodes, page_contents, page_revisions, changeset_items, changesets: the trash item of rootId, the nodes ids, is
 * restored by restorerId: its root under root.parentId named root.name, each node alive, no root nor counts, updated
 * by the restorer at the time of its own item, which a restore in the background writes batch by batch (M8/P4 design
 * 4.5); their contents, versions and items alive; the restore's changeset of its kind, a trash_restore or, a deletion
 * reverted, a revert (M8/P3 design 4.8), holding an item of each node, restored, from nothing to where the node is, a
 * page's at its content's revision.
 */
export async function expectSubtreeRestored(
  db: Database,
  rootId: string,
  ids: string[],
  restorerId: string,
  root: RestoredRoot,
  { kind }: { kind: "trash_restore" | "revert" }
): Promise<void> {
  const rows = await db.query(
    `SELECT n.parent_id AS "parentId", n.name, s.kind, s.created_by_id = $3 AS by_restorer,
            (SELECT count(*)::int FROM nodes x WHERE x.id = ANY($2::uuid[]) AND x.deleted_at IS NULL AND NOT x.trash_root
               AND x.trash_pages IS NULL AND x.trash_assets IS NULL AND x.trash_height IS NULL
               AND x.updated_by_id = $3 AND x.updated_at = (SELECT i.created_at FROM changeset_items i
                 WHERE i.changeset_id = s.id AND i.node_id = x.id)) AS nodes,
            (SELECT count(*)::int FROM nodes x WHERE x.id = ANY($2::uuid[]) AND x.kind = 'page' AND NOT EXISTS (
               SELECT 1 FROM page_contents c WHERE c.node_id = x.id AND c.deleted_at IS NULL)) AS pages_without_content,
            (SELECT count(*)::int FROM page_revisions v WHERE v.node_id = ANY($2::uuid[]) AND v.deleted_at IS NOT NULL) AS versions_deleted,
            (SELECT count(*)::int FROM changeset_items i WHERE i.node_id = ANY($2::uuid[]) AND i.deleted_at IS NOT NULL) AS items_deleted,
            (SELECT count(*)::int FROM changeset_items i JOIN nodes x ON x.id = i.node_id
              WHERE i.changeset_id = s.id AND i.node_id = ANY($2::uuid[]) AND i.restored AND i.deleted_at IS NULL
                AND i.before_name IS NULL AND i.before_parent_id IS NULL AND i.before_sort_order IS NULL
                AND i.after_name = x.name AND i.after_parent_id IS NOT DISTINCT FROM x.parent_id
                AND i.after_sort_order = x.sort_order
                AND i.revision IS NOT DISTINCT FROM (SELECT c.revision FROM page_contents c WHERE c.node_id = x.id)) AS items,
            (SELECT count(*)::int FROM changeset_items i WHERE i.changeset_id = s.id) AS changeset_items
       FROM nodes n
       JOIN LATERAL (SELECT s.* FROM changeset_items i JOIN changesets s ON s.id = i.changeset_id
                      WHERE i.node_id = n.id ORDER BY s.created_at DESC, s.id DESC LIMIT 1) s ON true
      WHERE n.id = $1`,
    [rootId, ids, restorerId]
  );
  expect(rows).toEqual([
    {
      ...root,
      kind,
      by_restorer: true,
      nodes: ids.length,
      pages_without_content: 0,
      versions_deleted: 0,
      items_deleted: 0,
      items: ids.length,
      changeset_items: ids.length,
    },
  ]);
}

/**
 * page_contents: the page written, as the API answered its write, or as a story builds it from an MCP answer, holds
 * content, its bytes' size and SHA-256 at the answer's revision, written by writerId at the answer's time.
 */
export async function expectContentWritten(
  db: Database,
  written: Pick<Page, "id" | "revision" | "content_updated_at">,
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

/** edit_sessions: the ids of the edit sessions of the page pageId, alive or expired, oldest first. */
export async function sessionsOf(db: Database, pageId: string): Promise<string[]> {
  const rows = await db.query<{ id: string }>(
    "SELECT id FROM edit_sessions WHERE node_id = $1 ORDER BY created_at, id",
    [pageId]
  );
  return rows.map((row) => row.id);
}

/** edit_sessions: the edit session id is gone. */
export async function expectSessionGone(db: Database, id: string): Promise<void> {
  const [row] = await db.query<{ n: number }>("SELECT count(*)::int AS n FROM edit_sessions WHERE id = $1", [id]);
  expect(row?.n).toBe(0);
}

/** A version of a page as the stories hold it (M8 design 4.3): what it holds, from base to revision, and who wrote it how. */
export interface StoredVersion {
  base: number | null;
  revision: number;
  content: string;
  /** Its changeset's kind: edit, import, version_restore… */
  kind: string;
  /** Who wrote it, and through which client: web, api… */
  byId: string;
  client: string;
  /** The changeset its changeset reverts, a revert's alone: none unless given. */
  revertsId?: string | null;
  /** The versions its changeset holds: itself alone unless given, as an MCP write's parents created with it are more. */
  versions?: number;
  /** Every write merged into this version was a system rewrite; ordinary versions default to false. */
  rewrite?: boolean;
}

/**
 * page_revisions, changesets: the versions of the page pageId are versions, oldest first, each a row of its content,
 * with its size and SHA-256, in a changeset of its kind by its writer through its client, reverting the changeset
 * given or none, which holds that version alone or as many as it says.
 */
export async function expectVersions(db: Database, pageId: string, versions: StoredVersion[]): Promise<void> {
  const rows = await db.query(
    `SELECT r.base_revision AS base, r.revision, r.content, r.rewrite, x.kind, x.created_by_id AS "byId", x.client,
            x.reverts_id AS "revertsId",
            r.byte_size = octet_length(convert_to(r.content, 'UTF8'))
              AND r.content_hash = sha256(convert_to(r.content, 'UTF8')) AS hashed,
            (SELECT count(*)::int FROM page_revisions o WHERE o.changeset_id = x.id) AS versions
       FROM page_revisions r JOIN changesets x ON x.id = r.changeset_id
      WHERE r.node_id = $1 AND r.deleted_at IS NULL
      ORDER BY r.revision`,
    [pageId]
  );
  expect(rows).toEqual(
    versions.map((version) => ({ revertsId: null, versions: 1, rewrite: false, ...version, hashed: true }))
  );
}

/**
 * The raw durable rows of one small story notebook, including deleted rows. A refusal must leave them byte for
 * byte unchanged: tree, content, notebook activity, history, sessions and attachment metadata. Each table returns
 * at most 65 rows and the fixture must contain at most 64; an overflowing snapshot fails instead of hiding writes
 * beyond its bound. Search-index and event-queue work is asynchronous and outside this refusal snapshot.
 */
export async function pageStateOf(db: Database, notebookId: string): Promise<string[]> {
  const rows = await db.query<{ kind: string; id: string; snapshot: string }>(
    `WITH stored AS (
       (SELECT 'notebook' AS kind, n.id::text AS id, to_jsonb(n) AS doc
          FROM notebooks n WHERE n.id = $1 ORDER BY n.id LIMIT 65)
       UNION ALL
       (SELECT 'node', n.id::text, to_jsonb(n)
          FROM nodes n WHERE n.notebook_id = $1 ORDER BY n.id LIMIT 65)
       UNION ALL
       (SELECT 'content', p.node_id::text, to_jsonb(p)
          FROM page_contents p JOIN nodes n ON n.id = p.node_id
         WHERE n.notebook_id = $1 ORDER BY p.node_id LIMIT 65)
       UNION ALL
       (SELECT 'changeset', c.id::text, to_jsonb(c)
          FROM changesets c WHERE c.notebook_id = $1 ORDER BY c.id LIMIT 65)
       UNION ALL
       (SELECT 'item', i.id::text, to_jsonb(i)
          FROM changeset_items i JOIN changesets c ON c.id = i.changeset_id
         WHERE c.notebook_id = $1 ORDER BY i.id LIMIT 65)
       UNION ALL
       (SELECT 'revision', r.id::text, to_jsonb(r)
          FROM page_revisions r JOIN nodes n ON n.id = r.node_id
         WHERE n.notebook_id = $1 ORDER BY r.id LIMIT 65)
       UNION ALL
       (SELECT 'session', s.id::text, to_jsonb(s)
          FROM edit_sessions s WHERE s.notebook_id = $1 ORDER BY s.id LIMIT 65)
       UNION ALL
       (SELECT 'blob', b.id::text, to_jsonb(b)
          FROM asset_blobs b WHERE b.notebook_id = $1 ORDER BY b.id LIMIT 65)
     ) SELECT kind, id, kind || ':' || doc::text AS snapshot FROM stored ORDER BY kind, id`,
    [notebookId]
  );
  expect(rows.filter((row) => row.kind === "notebook")).toHaveLength(1);
  const counts = new Map<string, number>();
  for (const row of rows) counts.set(row.kind, (counts.get(row.kind) ?? 0) + 1);
  for (const [kind, count] of counts) {
    expect(count, `${kind}: refusal snapshots require a small story notebook`).toBeLessThanOrEqual(64);
  }
  return rows.map((row) => row.snapshot);
}

/** What a revert holds (M8/P3 design 4.8): the changeset it reverts, its items and its versions. */
export interface StoredRevert {
  revertsId: string;
  items: number;
  versions: number;
}

/**
 * changesets, changeset_items, page_revisions: the changeset revertId is a revert of the changeset revert.revertsId by
 * reverterId from client, alive, holding revert.items items and revert.versions versions, deleted with their nodes or
 * not.
 */
export async function expectRevert(
  db: Database,
  revertId: string,
  reverterId: string,
  client: string,
  revert: StoredRevert
): Promise<void> {
  const rows = await db.query(
    `SELECT s.kind, s.reverts_id AS "revertsId", s.created_by_id = $2 AS by_reverter, s.client,
            (SELECT count(*)::int FROM changeset_items i WHERE i.changeset_id = s.id) AS items,
            (SELECT count(*)::int FROM page_revisions r WHERE r.changeset_id = s.id) AS versions
       FROM changesets s WHERE s.id = $1 AND s.deleted_at IS NULL`,
    [revertId, reverterId]
  );
  expect(rows).toEqual([{ kind: "revert", by_reverter: true, client, ...revert }]);
}

/** changesets: the reverts of the changeset id, newest first. */
export async function revertsOf(db: Database, id: string): Promise<string[]> {
  const rows = await db.query<{ id: string }>(
    "SELECT id FROM changesets WHERE reverts_id = $1 AND kind = 'revert' ORDER BY created_at DESC, id DESC",
    [id]
  );
  return rows.map((row) => row.id);
}

/**
 * changeset_items, page_contents: the changeset deletionId deleted the nodes ids, each of its items one of a node to
 * nothing, a page's holding its content's revision as it was deleted (M8/P3 design 3.2), which a revert writes back;
 * an attachment's none.
 */
export async function expectDeletionRevisions(db: Database, deletionId: string, ids: string[]): Promise<void> {
  const rows = await db.query(
    `SELECT i.node_id AS id, i.before_name IS NOT NULL AND i.after_name IS NULL AS deleted,
            i.revision IS NOT DISTINCT FROM (SELECT c.revision FROM page_contents c WHERE c.node_id = i.node_id) AS revision
       FROM changeset_items i WHERE i.changeset_id = $1 ORDER BY i.node_id`,
    [deletionId]
  );
  expect(rows).toEqual(ids.toSorted().map((id) => ({ id, deleted: true, revision: true })));
}
