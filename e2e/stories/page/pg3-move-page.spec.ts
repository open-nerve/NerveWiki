import type { ApiClient, Page } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectMoved } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getPage, listNodes, moveNode, postMove } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { choosePageAction, dragPage, moveDialog, movePageWith, optionsOf, treeTitles } from "../../fixtures/wiki-pages";
import { newTeam } from "../../fixtures/workspaces";

// PG3, moving and ordering (M4 design 3; M4/P5 design 3.7 for the page).

/** Creates count pages in the notebook with credential, each under the one before, under parentId, and returns them from the top. */
async function chain(
  api: ApiClient,
  credential: string,
  notebookId: string,
  count: number,
  parentId: string | null = null
): Promise<Page[]> {
  if (count === 0) {
    return [];
  }
  const page = await createPage(api, credential, notebookId, `Level ${count}`, parentId);
  return [page, ...(await chain(api, credential, notebookId, count - 1, page.id))];
}

test("PG3 (API): a member orders pages among their siblings and moves them under another parent or to the root; a move under itself is page.cycle, one past ten levels page.too_deep", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const editorPat = await joinAs(api, pat, workspace.slug, editorEmail, "member");
  const editorId = await accountIdOf(db, editorEmail);
  // The admin creates the pages; a member edits by the workspace access.
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans", "editor");
  const a = await createPage(api, pat, notebook.id, "A");
  const a1 = await createPage(api, pat, notebook.id, "A1", a.id);
  const a2 = await createPage(api, pat, notebook.id, "A2", a1.id);
  const b = await createPage(api, pat, notebook.id, "B");
  const c = await createPage(api, pat, notebook.id, "C");
  const roots = async () =>
    (await listNodes(api, pat, notebook.id)).filter((n) => n.parent_id === null).map((n) => n.name);

  // Among its siblings: first, after one, last.
  await expectMoved(db, await moveNode(api, editorPat, c.id, { parent_id: null, after_id: null }), null, editorId);
  expect(await roots()).toEqual(["C", "A", "B"]);
  await moveNode(api, editorPat, c.id, { parent_id: null, after_id: a.id });
  expect(await roots()).toEqual(["A", "C", "B"]);
  await moveNode(api, editorPat, c.id, { parent_id: null });
  expect(await roots()).toEqual(["A", "B", "C"]);

  // Under another parent, and back to the root.
  const under = await moveNode(api, editorPat, b.id, { parent_id: a.id });
  expect(under).toMatchObject({ id: b.id, parent_id: a.id, name: "B" });
  await expectMoved(db, under, null, editorId);
  expect((await getPage(api, pat, b.id)).data?.ancestors).toEqual([{ id: a.id, name: "A" }]);
  await expectMoved(db, await moveNode(api, editorPat, b.id, { parent_id: null, after_id: null }), a.id, editorId);
  expect(await roots()).toEqual(["B", "A", "C"]);

  // Under itself or its subtree; past ten levels with its subtree.
  const cycles = await Promise.all(
    [a.id, a2.id].map((parent) => postMove(api, editorPat, a.id, { parent_id: parent }))
  );
  expect(cycles.map((m) => [m.response.status, m.error?.code])).toEqual([
    [409, "page.cycle"],
    [409, "page.cycle"],
  ]);
  const levels = await chain(api, pat, notebook.id, 8);
  const tooDeep = await postMove(api, editorPat, a.id, { parent_id: levels[7]?.id ?? null });
  expect([tooDeep.response.status, tooDeep.error?.code]).toEqual([409, "page.too_deep"]);
  expect((await getPage(api, pat, a.id)).data?.parent_id).toBeNull();
  const deepest = await moveNode(api, editorPat, a.id, { parent_id: levels[6]?.id ?? null });
  expect((await getPage(api, pat, a2.id)).data?.ancestors.length).toBe(9);
  await expectMoved(db, deepest, null, editorId);
});

test("PG3 (page): an editor drags a page before another and into another, and moves one with Move to; a drop into its own subtree sends nothing, the dialog offers no parent the tree forbids, and a move past ten levels that the tree did not foresee stays in the dialog", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const page = await signedInPage(await joinOnboarded(api, adminPat, workspace.slug, editorEmail, "member"));
  const editorId = await accountIdOf(db, editorEmail);
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Plans", "editor");
  const a = await createPage(api, adminPat, notebook.id, "A");
  await createPage(api, adminPat, notebook.id, "B");
  const c = await createPage(api, adminPat, notebook.id, "C");
  const x = await createPage(api, adminPat, notebook.id, "X");
  const y = await createPage(api, adminPat, notebook.id, "Y", x.id);
  // Level 8 at the root, Level 1 eight levels down.
  const levels = await chain(api, adminPat, notebook.id, 8);
  const pathTo = (level: number) =>
    levels
      .slice(0, 9 - level)
      .map((each) => each.name)
      .join(" / ");
  const nodeOf = async (id: string) => {
    const node = (await listNodes(api, adminPat, notebook.id)).find((each) => each.id === id);
    if (node === undefined) {
      throw new Error(`no node ${id} in the tree`);
    }
    return node;
  };
  await page.goto(notebookPath(workspace.slug, notebook.id));
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["A", "B", "C", "X", "Level 8"]);

  const before = answerTo(page, "POST", `/api/v0/nodes/${c.id}/move`);
  await dragPage(page, "Plans", "C", "A", "before");
  expect((await before).status()).toBe(200);
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["C", "A", "B", "X", "Level 8"]);
  await expectMoved(db, await nodeOf(c.id), null, editorId);

  // Into B: A is its last child, and B opens.
  const into = answerTo(page, "POST", `/api/v0/nodes/${a.id}/move`);
  await dragPage(page, "Plans", "A", "B", "into");
  expect((await into).status()).toBe(200);
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["C", "B", "A", "X", "Level 8"]);
  await expectMoved(db, await nodeOf(a.id), null, editorId);
  // B into its own child is blocked: nothing goes out (checked with the requests below).
  await dragPage(page, "Plans", "B", "A", "into");

  // X, with Y under it, can go eight levels down, not nine, and not under itself.
  await choosePageAction(page, "Plans", "X", "Move to…");
  const dialog = moveDialog(page, "X");
  const parents = await optionsOf(dialog, "Parent page");
  expect(parents).toContain(pathTo(1));
  expect(parents).not.toContain("X");
  expect(parents).not.toContain("X / Y");
  // Another tab gives Y a child: under Level 1, X would be eleven levels down.
  await createPage(api, adminPat, notebook.id, "Z", y.id);
  expect((await movePageWith(page, "Plans", x.id, "X", pathTo(1), "Last")).status()).toBe(409);
  await expect(dialog.getByRole("alert")).toHaveText("Pages nest at most 10 levels deep.");
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  // The tree read again, the dialog no longer offers it.
  await expect.poll(() => optionsOf(dialog, "Parent page")).not.toContain(pathTo(1));

  expect((await movePageWith(page, "Plans", x.id, "X", pathTo(2), "Last")).status()).toBe(200);
  await expect(dialog).toBeHidden();
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["C", "B", "A", ...levels.map((each) => each.name), "X"]);
  await expectMoved(db, await nodeOf(x.id), null, editorId);
  expect(pageWatch.apiRequests.filter((request) => request.endsWith("/move"))).toEqual(
    [c.id, a.id, x.id, x.id].map((id) => `POST /api/v0/nodes/${id}/move`)
  );
});
