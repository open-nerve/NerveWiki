import { randomUUID } from "node:crypto";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNewPage } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { addedNotebookMember, addNotebookMember } from "../../fixtures/notebook-members";
import { notebookHeading, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, postPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  breadcrumbs,
  newPageWith,
  newSubpageWith,
  pageHeading,
  pageTree,
  treeTitles,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newTeam } from "../../fixtures/workspaces";

// PG1, creating a page (M4 design 3; M4/P5 design 3.7 for the page).

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

test("PG1 (page): an editor creates Untitled, then Untitled 2, from the tree's New page, and a subpage from a page's menu, arriving on each; the subpage's parent opens", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const page = await signedInPage(await joinOnboarded(api, adminPat, workspace.slug, editorEmail, "member"));
  const editorId = await accountIdOf(db, editorEmail);
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Plans", "viewer");
  await addedNotebookMember(api, adminPat, notebook.id, editorId, "editor");
  await page.goto(notebookPath(workspace.slug, notebook.id));
  await expect(notebookHeading(page, "Plans")).toBeVisible();
  await expect(pageTree(page, "Plans").getByRole("link")).toHaveCount(0);

  const first = await newPageWith(page, notebook);
  expect(first.status).toBe(201);
  expect(first.created).toMatchObject({ notebook_id: notebook.id, parent_id: null, name: "Untitled" });
  await expect(pageHeading(page, "Untitled")).toBeFocused();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, first.created.id));
  await expectNewPage(db, first.created, editorId, "web");

  const second = await newPageWith(page, notebook);
  expect(second.created).toMatchObject({ parent_id: null, name: "Untitled 2" });
  await expect(pageHeading(page, "Untitled 2")).toBeFocused();
  await expectNewPage(db, second.created, editorId, "web");

  // A title is free among its own siblings: the subpage is Untitled too.
  const sub = await newSubpageWith(page, notebook, "Untitled 2");
  expect(sub.created).toMatchObject({ parent_id: second.created.id, name: "Untitled" });
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, sub.created.id));
  await expect(pageHeading(page, "Untitled")).toBeFocused();
  expect(await breadcrumbs(page)).toEqual(["Plans", "Untitled 2", "Untitled"]);
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Untitled", "Untitled 2", "Untitled"]);
  await expectNewPage(db, sub.created, editorId, "web");
});
