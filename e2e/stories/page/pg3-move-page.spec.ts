import type { ApiClient, Page } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectMoved } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getPage, listNodes, moveNode, postMove } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// PG3, moving and ordering (M4 design 3): the API's part; the page version
// comes with the tree (M4/P5).

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
