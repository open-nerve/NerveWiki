import { accountIdOf } from "../../fixtures/assert/identity";
import { expectSubtreeDeleted } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { answerTo } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode, getPage, listNodes, postPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { choosePageAction, pageHeading, treeTitles, wikiPagePath } from "../../fixtures/wiki-pages";
import { newTeam } from "../../fixtures/workspaces";

// PG4, deleting a subtree (M4 design 3; M4/P5 design 3.7 for the page); the
// edit sessions come with the content (M4/P4).

test("PG4 (API): a member deletes a page with its subpages at one time; no one reads them after, and its sibling stays", async ({
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
  await createPage(api, pat, notebook.id, "Sibling");

  expect((await deleteNode(api, editorPat, doomed.id)).response.status).toBe(204);
  const subtree = [doomed.id, child.id, grandchild.id];
  await expectSubtreeDeleted(db, doomed.id, subtree, editorId);
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

test("PG4 (page): an editor deletes a page with its subpages, the confirmation counting them; the page shown, one of them, goes to the deleted page's parent; its address is no page after", async ({
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
