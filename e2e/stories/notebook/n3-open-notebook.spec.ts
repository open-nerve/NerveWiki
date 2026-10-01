import type { Notebook } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNotebook } from "../../fixtures/assert/notebook";
import { emailFor } from "../../fixtures/auth";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { changeAccessWith, notebookGroups, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook, getNotebook, listNotebooks, updateNotebook } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N3, opening a notebook to its workspace (M3 design 3; M3/P4 design 3.4
// for the page).

test("N3 (API): open as viewer, the workspace's members and admins read it, its guests do not; open as editor, members edit it", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const memberPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "member"), "member");
  const guestPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "guest"), "guest");
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Handbook");

  const viewer = await updateNotebook(api, ownerPat, notebook.id, { workspace_access: "viewer" });
  expect(viewer.response.status).toBe(200);
  expect(viewer.data).toMatchObject({ workspace_access: "viewer", role: "admin", member_count: 1 });
  if (!viewer.data) {
    throw new Error("the change answered no notebook");
  }
  await expectNotebook(db, viewer.data, await accountIdOf(db, ownerEmail));
  const readers = [adminPat, memberPat];
  const reads = await Promise.all(readers.map((pat) => getNotebook(api, pat, notebook.id)));
  expect(reads.map((r) => r.data?.role)).toEqual(["reader", "reader"]);
  const lists = await Promise.all(readers.map((pat) => listNotebooks(api, pat, workspace.slug)));
  expect(lists.map((list) => list.map((n) => [n.name, n.role]))).toEqual([
    [["Handbook", "reader"]],
    [["Handbook", "reader"]],
  ]);
  const hidden = await getNotebook(api, guestPat, notebook.id);
  expect(hidden.response.status).toBe(404);
  expect(hidden.error?.code).toBe("notebook.not_found");
  expect(await listNotebooks(api, guestPat, workspace.slug)).toEqual([]);

  expect((await updateNotebook(api, ownerPat, notebook.id, { workspace_access: "editor" })).response.status).toBe(200);
  expect((await getNotebook(api, memberPat, notebook.id)).data?.role).toBe("editor");
  expect((await getNotebook(api, ownerPat, notebook.id)).data?.role).toBe("admin");
  expect((await getNotebook(api, guestPat, notebook.id)).response.status).toBe(404);
});

test("N3 (page): its admin opens the notebook to the workspace on its general page; the members see it with the access's role, the guests do not", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerTokens = await joinOnboarded(api, adminPat, workspace.slug, ownerEmail, "member");
  const ownerId = await accountIdOf(db, ownerEmail);
  const memberPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "member"), "member");
  const guestPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "guest"), "guest");
  const notebook = await createNotebook(api, ownerTokens.access_token, workspace.slug, "Handbook");
  const page = await signedInPage(ownerTokens);
  await page.goto(notebookPath(workspace.slug, notebook.id, "general"));
  await expect.poll(() => notebookGroups(page, "Acme")).toEqual({ "My notebooks": ["Handbook"] });
  /** The roles the workspace's member, its admin and its guest see the notebook with, as their lists have it. */
  const seen = () =>
    Promise.all(
      [memberPat, adminPat, guestPat].map(async (pat) =>
        (await listNotebooks(api, pat, workspace.slug)).map((each) => each.role)
      )
    );

  const viewer = await changeAccessWith(page, notebook.id, "Workspace can read");
  expect(viewer.status()).toBe(200);
  await expect(page.getByText("Saved.", { exact: true })).toBeVisible();
  // Open to the workspace, it is the team's.
  await expect.poll(() => notebookGroups(page, "Acme")).toEqual({ "Team notebooks": ["Handbook"] });
  await expectNotebook(db, (await viewer.json()) as Notebook, ownerId);
  expect(await seen()).toEqual([["reader"], ["reader"], []]);

  expect((await changeAccessWith(page, notebook.id, "Workspace can edit")).status()).toBe(200);
  expect(await seen()).toEqual([["editor"], ["editor"], []]);
});
