import { randomUUID } from "node:crypto";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNewPage } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { addNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, postPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// PG1, creating a page (M4 design 3); the page version comes with the tree
// (M4/P5).

test("PG1 (API): an editor creates pages at the root and under a page; a reader cannot; a title a file could not take and a parent of no page are refused", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const editorPat = await joinAs(api, adminPat, workspace.slug, editorEmail, "member");
  const readerPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "reader"), "member");
  const editorId = await accountIdOf(db, editorEmail);
  // Open to the workspace to read: the editor writes by an explicit membership.
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Plans", "viewer");
  expect((await addNotebookMember(api, adminPat, notebook.id, editorId, "editor")).response.status).toBe(201);

  const root = await createPage(api, editorPat, notebook.id, "  Roadmap  ");
  expect(root).toMatchObject({
    notebook_id: notebook.id,
    parent_id: null,
    kind: "page",
    name: "Roadmap",
    ancestors: [],
    revision: 1,
    byte_size: 0,
    content_updated_by: editorId,
  });
  await expectNewPage(db, root, editorId, "api");
  const child = await createPage(api, editorPat, notebook.id, "Q4", root.id);
  expect(child).toMatchObject({ parent_id: root.id, name: "Q4", ancestors: [{ id: root.id, name: "Roadmap" }] });
  await expectNewPage(db, child, editorId, "api");

  // The reader cannot; titles a file could not take, and a parent of no
  // page of the notebook, are refused; nothing is added.
  const refused = await postPage(api, readerPat, notebook.id, { parent_id: null, title: "Reader's" });
  expect(refused.response.status).toBe(403);
  expect(refused.error?.code).toBe("forbidden");
  const invalid = [
    ["  ", "required"],
    ["Plans/2026", "invalid_format"],
    ["con.txt", "not_allowed"],
  ] as const;
  const answers = await Promise.all(
    invalid.map(([title]) => postPage(api, editorPat, notebook.id, { parent_id: root.id, title }))
  );
  expect(answers.map((a) => [a.response.status, a.error?.errors?.map((e) => [e.field, e.code])])).toEqual(
    invalid.map(([, code]) => [422, [["title", code]]])
  );
  const lost = await postPage(api, editorPat, notebook.id, { parent_id: randomUUID(), title: "Lost" });
  expect([lost.response.status, lost.error?.errors?.map((e) => [e.field, e.code])]).toEqual([
    422,
    [["parent_id", "not_allowed"]],
  ]);
  const [pages] = await db.query<{ n: number }>("SELECT count(*)::int AS n FROM nodes WHERE notebook_id = $1", [
    notebook.id,
  ]);
  expect(pages?.n).toBe(2);
});
