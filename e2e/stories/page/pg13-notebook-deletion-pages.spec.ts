import type { ApiClient, Notebook } from "@nervewiki/api-client";

import { expectPagesDeletedWith } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { memberOf, removeMember } from "../../fixtures/members";
import { deleteNotebookWith, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook, deleteNotebook } from "../../fixtures/notebooks";
import { deleteOwnerless } from "../../fixtures/ownerless";
import { createPage, getPage } from "../../fixtures/pages";
import { deletedDaysAgo } from "../../fixtures/purge";
import { expect, test } from "../../fixtures/test";
import { pageHeading, pageTree, wikiPagePath } from "../../fixtures/wiki-pages";
import { workspaceHeading } from "../../fixtures/workspace-pages";
import { createWorkspace, deleteWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// PG13, a notebook's deletion takes its pages (M4 design 3): by its admin,
// with its workspace, or ownerless by the workspace's admin, each at the
// notebook's time; the purge, the background job's alone, clears them.
// In the browser (M4/P5 design 3.6), its pages leave the left column and
// their addresses.

/** Writes a tree of three levels in the notebook with credential, and returns its pages' ids. */
async function tree(api: ApiClient, credential: string, notebook: Notebook): Promise<string[]> {
  const root = await createPage(api, credential, notebook.id, "Root");
  const child = await createPage(api, credential, notebook.id, "Child", root.id);
  const grandchild = await createPage(api, credential, notebook.id, "Grandchild", child.id);
  return [root.id, child.id, grandchild.id];
}

test("PG13 (API): deleting a notebook, its workspace or an ownerless notebook deletes its pages at its time; the purge clears a tree of three levels 60 days on", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const plans = await createNotebook(api, pat, workspace.slug, "Plans");
  const planPages = await tree(api, pat, plans);
  expect((await deleteNotebook(api, pat, plans.id)).response.status).toBe(204);
  await expectPagesDeletedWith(db, plans.id, 3);

  const leaverEmail = emailFor(testInfo, "leaver");
  const leaverPat = await joinAs(api, pat, workspace.slug, leaverEmail, "member");
  const drafts = await createNotebook(api, leaverPat, workspace.slug, "Drafts");
  await tree(api, leaverPat, drafts);
  expect(
    (await removeMember(api, pat, (await memberOf(api, pat, workspace.slug, leaverEmail)).id)).response.status
  ).toBe(204);
  expect((await deleteOwnerless(api, pat, drafts.id)).response.status).toBe(204);
  await expectPagesDeletedWith(db, drafts.id, 3);

  const old = await createWorkspace(api, pat, "Old", slugFor(testInfo, "old"));
  const notes = await createNotebook(api, pat, old.slug, "Notes");
  await tree(api, pat, notes);
  await deleteWorkspace(api, pat, old.slug);
  await expectPagesDeletedWith(db, notes.id, 3);
  const reads = await Promise.all(planPages.map((id) => getPage(api, pat, id)));
  expect(reads.map((r) => [r.response.status, r.error?.code])).toEqual(planPages.map(() => [404, "page.not_found"]));

  // The purge: one run clears the tree, then the notebook and the
  // workspace, with no error on the way. A run already going may see the
  // tree half moved back and fail on a foreign key, to be retried; the
  // runs that count are those queued after the move.
  await deletedDaysAgo(db, old.id, 61);
  const [mark] = await db.query<{ id: string }>("SELECT coalesce(max(id), 0)::text AS id FROM river_job");
  await expect
    .poll(async () => (await db.query("SELECT id FROM workspaces WHERE id = $1", [old.id])).length, {
      message: "the workspace deleted 61 days ago is purged, after its notebook and its pages",
      timeout: 15_000,
    })
    .toBe(0);
  const left = await db.query(
    `SELECT (SELECT count(*)::int FROM nodes WHERE notebook_id = $1) AS pages,
            (SELECT count(*)::int FROM changesets WHERE notebook_id = $1) AS changesets,
            (SELECT count(*)::int FROM notebooks WHERE id = $1) AS notebooks`,
    [notes.id]
  );
  expect(left).toEqual([{ pages: 0, changesets: 0, notebooks: 0 }]);
  const failed = await db.query(
    "SELECT id FROM river_job WHERE kind = 'platform.purge_soft_deleted' AND id > $1::bigint AND errors IS NOT NULL",
    [mark?.id]
  );
  expect(failed, "purge runs that failed").toEqual([]);
});

test("PG13 (page): once its admin deletes the notebook, its pages leave the left column, and a page's address is no page", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const tokens = await joinOnboarded(api, adminPat, workspace.slug, emailFor(testInfo, "owner"), "member");
  const notebook = await createNotebook(api, tokens.access_token, workspace.slug, "Plans");
  const guide = await createPage(api, tokens.access_token, notebook.id, "Guide");
  await createPage(api, tokens.access_token, notebook.id, "Install", guide.id);
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, guide.id));
  await expect(pageHeading(page, "Guide")).toBeVisible();
  await expect(pageTree(page, "Plans").getByRole("link", { name: "Guide", exact: true })).toBeVisible();

  await page.goto(notebookPath(workspace.slug, notebook.id, "general"));
  expect(await deleteNotebookWith(page, notebook.id, "Plans")).toBe(204);

  await expect(workspaceHeading(page, "Acme")).toBeFocused();
  await expect(pageTree(page, "Plans")).toHaveCount(0);
  await expectPagesDeletedWith(db, notebook.id, 2);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, guide.id));
  await expect(page.getByRole("heading", { level: 1, name: "Page not found", exact: true })).toBeVisible();
  await expect(pageTree(page, "Plans")).toHaveCount(0);
});
