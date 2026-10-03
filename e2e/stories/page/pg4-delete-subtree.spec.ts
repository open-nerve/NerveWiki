import { accountIdOf } from "../../fixtures/assert/identity";
import { expectSubtreeDeleted } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { answerTo } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode, getPage, heartbeat, listNodes, openSession, postPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { choosePageAction, pageHeading, treeTitles, wikiPagePath } from "../../fixtures/wiki-pages";
import { newTeam } from "../../fixtures/workspaces";

// PG4, deleting a subtree (M4 design 3; M4/P5 design 3.7 for the page); the
// edit sessions come with the content (M4/P4): the deleter's own, and the
// expired ones; another account's alive session refuses it (M5/P1).

/** Expires the edit session id, through the database rather than the wall clock. */
async function expire(db: Database, id: string): Promise<void> {
  await db.query(
    "UPDATE edit_sessions SET created_at = now() - interval '3 minutes', expires_at = now() - interval '1 minute' WHERE id = $1",
    [id]
  );
}

test("PG4 (API): a member deletes a page with its subpages at one time, their edit sessions with them, the member's own and an expired one, once another account's alive session in it no longer refuses it; no one reads them after, and its sibling stays, its session too", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const editorPat = await joinAs(api, pat, workspace.slug, editorEmail, "member");
  const editorId = await accountIdOf(db, editorEmail);
  // The admin creates the pages; a member edits by the workspace access.
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans", "editor");
  const doomed = await createPage(api, pat, notebook.id, "Doomed");
  const child = await createPage(api, pat, notebook.id, "Child", doomed.id);
  const grandchild = await createPage(api, pat, notebook.id, "Grandchild", child.id);
  const sibling = await createPage(api, pat, notebook.id, "Sibling");
  const childs = await openSession(api, pat, child.id);
  await openSession(api, editorPat, grandchild.id);
  const siblings = await openSession(api, pat, sibling.id);

  const refused = await deleteNode(api, editorPat, doomed.id);
  expect([refused.response.status, refused.error?.code, refused.error?.lock?.page_id]).toEqual([
    409,
    "page.locked",
    child.id,
  ]);
  await expire(db, childs.id);
  expect((await deleteNode(api, editorPat, doomed.id)).response.status).toBe(204);
  const subtree = [doomed.id, child.id, grandchild.id];
  await expectSubtreeDeleted(db, doomed.id, subtree, editorId);
  expect((await heartbeat(api, pat, siblings.id)).response.status).toBe(200);
  const reads = await Promise.all(subtree.map((id) => getPage(api, pat, id)));
  expect(reads.map((r) => [r.response.status, r.error?.code])).toEqual(subtree.map(() => [404, "page.not_found"]));
  expect((await listNodes(api, pat, notebook.id)).map((n) => n.name)).toEqual(["Sibling"]);

  // What is gone is gone for a write too.
  const again = await deleteNode(api, editorPat, doomed.id);
  expect([again.response.status, again.error?.code]).toEqual([404, "page.not_found"]);
  const late = await postPage(api, pat, notebook.id, { parent_id: child.id, title: "Late" });
  expect([late.response.status, late.error?.errors?.map((e) => [e.field, e.code])]).toEqual([
    422,
    [["parent_id", "not_allowed"]],
  ]);
});

test("PG4 (page): an editor deletes a page with its subpages, the confirmation counting them, their expired edit sessions with them; the page shown, one of them, goes to the deleted page's parent; its address is no page after", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const page = await signedInPage(await joinOnboarded(api, adminPat, workspace.slug, editorEmail, "member"));
  const editorId = await accountIdOf(db, editorEmail);
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Plans", "editor");
  const guide = await createPage(api, adminPat, notebook.id, "Guide");
  const install = await createPage(api, adminPat, notebook.id, "Install", guide.id);
  const linux = await createPage(api, adminPat, notebook.id, "Linux", install.id);
  const mac = await createPage(api, adminPat, notebook.id, "Mac", linux.id);
  await createPage(api, adminPat, notebook.id, "FAQ", guide.id);
  await expire(db, (await openSession(api, adminPat, linux.id)).id);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, mac.id));
  await expect(pageHeading(page, "Mac")).toBeVisible();

  await choosePageAction(page, "Plans", "Install", "Delete");
  const dialog = page.getByRole("alertdialog", { name: "Delete Install?", exact: true });
  await expect(dialog).toContainText("Its subpages go with it: 2.");
  const answer = answerTo(page, "DELETE", `/api/v0/nodes/${install.id}`);
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  expect((await answer).status()).toBe(204);

  await expect(pageHeading(page, "Guide")).toBeFocused();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, guide.id));
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Guide", "FAQ"]);
  await expectSubtreeDeleted(db, install.id, [install.id, linux.id, mac.id], editorId);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, mac.id));
  await expect(page.getByRole("heading", { level: 1, name: "Page not found", exact: true })).toBeVisible();
});
