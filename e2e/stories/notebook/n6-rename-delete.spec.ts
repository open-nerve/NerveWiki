import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNotebook, expectNotebookDeletedWithItsMembers } from "../../fixtures/assert/notebook";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook, deleteNotebook, getNotebook, updateNotebook } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N6, renaming and deleting a notebook (M3 design 3). The page version
// comes with M3/P4.

test("N6 (API): its admin renames and deletes a notebook, an editor can do neither; deleted, no one sees it, its members gone with it", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const editorPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "editor"), "member");
  const ownerId = await accountIdOf(db, ownerEmail);
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Drafts", "editor");

  const renamed = await updateNotebook(api, ownerPat, notebook.id, { name: " Specs " });
  expect(renamed.response.status).toBe(200);
  expect(renamed.data).toMatchObject({ name: "Specs", workspace_access: "editor", role: "admin" });
  if (!renamed.data) {
    throw new Error("the rename answered no notebook");
  }
  await expectNotebook(db, renamed.data, ownerId);

  // An editor by the workspace access: it reads, it cannot manage.
  expect((await getNotebook(api, editorPat, notebook.id)).data?.role).toBe("editor");
  const refusals = await Promise.all([
    updateNotebook(api, editorPat, notebook.id, { name: "Mine" }),
    deleteNotebook(api, editorPat, notebook.id),
  ]);
  expect(refusals.map((r) => [r.response.status, r.error?.code])).toEqual([
    [403, "forbidden"],
    [403, "forbidden"],
  ]);

  expect((await deleteNotebook(api, ownerPat, notebook.id)).response.status).toBe(204);
  await expectNotebookDeletedWithItsMembers(db, notebook.id, ownerId);
  const reads = await Promise.all([ownerPat, editorPat].map((pat) => getNotebook(api, pat, notebook.id)));
  expect(reads.map((r) => [r.response.status, r.error?.code])).toEqual([
    [404, "notebook.not_found"],
    [404, "notebook.not_found"],
  ]);
});
