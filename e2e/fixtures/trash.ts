import type { ApiClient } from "@nervewiki/api-client";
import type { Locator, Page, Response } from "@playwright/test";

import { bearer } from "./auth";
import { answerTo } from "./browser";
import type { Database } from "./db";

// The trash of the stories (M8 design 4.5, 4.6; M8/P2 design 4.7): through
// the API, its page as a user works it, and the time of an item moved back
// by SQL.

/** credential's read of a page of the notebook notebookId's trash, after cursor, as the API answers it. */
export async function listTrash(api: ApiClient, credential: string, notebookId: string, cursor?: string) {
  return api.GET("/api/v0/notebooks/{notebook_id}/trash", {
    params: { path: { notebook_id: notebookId }, query: cursor === undefined ? {} : { cursor } },
    headers: bearer(credential),
  });
}

/** credential's read of the trash item whose root is the node id, as the API answers it. */
export async function getTrashItem(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/nodes/{node_id}/trash-item", {
    params: { path: { node_id: id } },
    headers: bearer(credential),
  });
}

/** credential's restore of the trash item whose root is the node id, as the API answers it. */
export async function restoreNode(api: ApiClient, credential: string, id: string) {
  return api.POST("/api/v0/nodes/{node_id}/restore", {
    params: { path: { node_id: id } },
    headers: bearer(credential),
  });
}

/** The address of the trash of the notebook notebookId in the workspace of slug, or of the preview of its item id. */
export function trashPath(slug: string, notebookId: string, id?: string): string {
  return `/${slug}/notebooks/${notebookId}/trash${id === undefined ? "" : `/${id}`}`;
}

/**
 * The trash's status line on page: the output its heading is described by, once a restore said where its item went.
 * The list's loading output, there while the list is read again after the restore, is a status too.
 */
export function trashStatus(page: Page): Locator {
  return page.getByRole("main").locator('xpath=.//output[@id = //h1[normalize-space() = "Trash"]/@aria-describedby]');
}

/** The items the trash on page lists, newer first. */
export function trashListed(page: Page): Locator {
  return page.getByRole("main").getByRole("list", { name: "Trash", exact: true }).getByRole("listitem");
}

/**
 * Restores the trash item whose root is the node id, named name and from where it was (its Restore's name: "Restore
 * “name” (from)"), with its Restore on the trash's page or its preview; resolves the answer.
 */
export async function restoreWith(page: Page, id: string, name: string, from: string): Promise<Response> {
  const answer = answerTo(page, "POST", `/api/v0/nodes/${id}/restore`);
  await page
    .getByRole("main")
    .getByRole("button", { name: `Restore “${name}” (${from})`, exact: true })
    .click();
  return answer;
}

/**
 * Moves the deletion of the trash item whose root is rootId back by ago (a PostgreSQL interval) from its time T: the
 * nodes of its group, deleted at T, and their contents, versions, items and attachments' rows deleted at T, each to T
 * less ago, in one statement (M8/P2 design 3.10, 4.7). A time of now() each, a statement each, would break the group:
 * the trash and the restore take what was deleted at one time.
 */
export async function trashedAgo(db: Database, rootId: string, ago: string): Promise<void> {
  const [moved] = await db.query<{ nodes: number }>(
    `WITH RECURSIVE r AS (SELECT deleted_at AS t FROM nodes WHERE id = $1 AND trash_root),
     g AS (SELECT $1::uuid AS id
           UNION ALL
           SELECT n.id FROM nodes n JOIN g ON n.parent_id = g.id, r WHERE n.deleted_at = r.t),
     contents AS (UPDATE page_contents x SET deleted_at = r.t - $2::interval FROM r
                  WHERE x.node_id IN (SELECT id FROM g) AND x.deleted_at = r.t RETURNING 1),
     versions AS (UPDATE page_revisions x SET deleted_at = r.t - $2::interval FROM r
                  WHERE x.node_id IN (SELECT id FROM g) AND x.deleted_at = r.t RETURNING 1),
     items AS (UPDATE changeset_items x SET deleted_at = r.t - $2::interval FROM r
               WHERE x.node_id IN (SELECT id FROM g) AND x.deleted_at = r.t RETURNING 1),
     blobs AS (UPDATE asset_blobs x SET deleted_at = r.t - $2::interval FROM r
               WHERE x.node_id IN (SELECT id FROM g) AND x.deleted_at = r.t RETURNING 1),
     moved AS (UPDATE nodes x SET deleted_at = r.t - $2::interval FROM r
               WHERE x.id IN (SELECT id FROM g) AND x.deleted_at = r.t RETURNING 1)
     SELECT (SELECT count(*) FROM moved)::int AS nodes`,
    [rootId, ago]
  );
  if (!moved || moved.nodes === 0) {
    throw new Error(`no trash item ${rootId} to move back`);
  }
}
