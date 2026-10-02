import { accountIdOf } from "../../fixtures/assert/identity";
import { expectSubtreeDeleted } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode, getPage, listNodes, postPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// PG4, deleting a subtree (M4 design 3): the API's part; the page version
// comes with the tree (M4/P5), the edit sessions with the content (M4/P4).

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
