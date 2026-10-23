import { accountIdOf } from "../../fixtures/assert/identity";
import {
  expectChangesetOf,
  expectDeletionRevisions,
  expectSubtreeDeleted,
  expectTrashRoot,
  sessionsOf,
} from "../../fixtures/assert/page";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { answerTo } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode, getPage, heartbeat, listNodes, openSession, postPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { choosePageAction, pageHeading, treeTitles, wikiPagePath } from "../../fixtures/wiki-pages";
import { newTeam } from "../../fixtures/workspaces";

// PG4, deleting a subtree (M4 design 3; M4/P5 design 3.7 for the page); the
// edit sessions come with the content (M4/P4): the deleter's own, and the
// expired ones; another account's alive session refuses it (M5/P1). An
// agent's deletion is refused by any alive session, its own account's too,
// for an agent ends no edit (M9 design 4.10; M9/P5 design 3.2).

/** A deletion's answer, as the tool's structuredContent has it. */
interface Deleted {
  node_id: string;
  pages: number;
  attachments: number;
  changeset_id: string;
}

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
  const { adminEmail, adminId, pat, workspace } = await newTeam(api, testInfo);
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
  expect([refused.response.status, refused.error?.code, refused.error?.lock]).toEqual([
    409,
    "page.locked",
    { page_id: child.id, user_id: adminId, display_name: displayNameOf(adminEmail) },
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

test("PG4 (MCP): an editor's agent is refused a page with subpages without recursive, saying how many; with recursive, refused while any of their pages is being edited, the editor's own too; once those sessions expire, it deletes the subtree at one time, the sessions with it, into the trash as one item; no one reads them after, and its sibling stays, its session too", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { adminEmail, adminId, pat, workspace } = await newTeam(api, testInfo);
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
  const editors = await openSession(api, editorPat, grandchild.id);
  const siblings = await openSession(api, pat, sibling.id);
  const agent = await mcpClient(nervewiki.baseURL, editorPat, { notebook: notebook.id });

  // Without recursive: what is under it counted, before any lock.
  const children = errorOf(await agent.call("delete", { path: "Doomed.md" }), "delete Doomed");
  expect([
    children.message,
    (children.fields as { field: string; code: string }[]).map((f) => [f.field, f.code]),
  ]).toEqual([
    'validation_failed: "Doomed.md" has 2 pages under it: set recursive to true to put them in the trash with it.',
    [["recursive", "required"]],
  ]);
  // With it: each page of the subtree being edited, level by level, the editor's own too.
  const lock = {
    page_id: child.id,
    path: "Doomed/Child.md",
    user_id: adminId,
    display_name: displayNameOf(adminEmail),
    you: false,
  };
  const own = {
    page_id: grandchild.id,
    path: "Doomed/Child/Grandchild.md",
    user_id: editorId,
    display_name: displayNameOf(editorEmail),
    you: true,
  };
  const { path: _, ...holder } = lock;
  const locked = errorOf(
    await agent.call("delete", { path: "Doomed.md", recursive: true }),
    "delete Doomed, recursive"
  );
  expect([locked.code, locked.locks, locked.locks_count, locked.lock]).toEqual(["page.locked", [lock, own], 2, holder]);

  // Expired, the sessions no longer refuse it, and go with the subtree; the deletion is one trash item.
  expect([await sessionsOf(db, child.id), await sessionsOf(db, grandchild.id)]).toEqual([[childs.id], [editors.id]]);
  await expire(db, childs.id);
  await expire(db, editors.id);
  const deleted = resultOf<Deleted>(
    await agent.call("delete", { path: "Doomed.md", recursive: true }),
    "delete Doomed once the sessions expired"
  );
  expect([deleted.node_id, deleted.pages, deleted.attachments]).toEqual([doomed.id, 3, 0]);
  const subtree = [doomed.id, child.id, grandchild.id];
  await expectSubtreeDeleted(db, deleted.node_id, subtree, editorId);
  await expectChangesetOf(db, deleted.changeset_id, {
    client: "mcp:e2e-agent",
    byId: editorId,
    items: subtree,
    versions: [],
  });
  await expectDeletionRevisions(db, deleted.changeset_id, subtree);
  await expectTrashRoot(db, doomed.id, { pages: 3, assets: 0, height: 3 }, { alone: true });
  expect((await heartbeat(api, pat, siblings.id)).response.status).toBe(200);
  const paths = ["Doomed.md", "Doomed/Child.md", "Doomed/Child/Grandchild.md"];
  const reads = await Promise.all(paths.map((path) => agent.call("read", { path })));
  expect(reads.map((answer, i) => errorOf(answer, `read ${paths[i]}`).code)).toEqual(paths.map(() => "page.not_found"));
  const listed = resultOf<{ items: { path: string }[] }>(await agent.call("list"), "list the root");
  expect(listed.items.map((item) => item.path)).toEqual(["Sibling.md"]);

  // What is gone is gone for an agent too.
  const again = errorOf(await agent.call("delete", { path: "Doomed.md", recursive: true }), "delete Doomed again");
  expect(again.code).toBe("page.not_found");
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
