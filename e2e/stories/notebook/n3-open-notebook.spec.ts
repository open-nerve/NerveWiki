import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNotebook } from "../../fixtures/assert/notebook";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook, getNotebook, listNotebooks, updateNotebook } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N3, opening a notebook to its workspace (M3 design 3). The page version
// comes with M3/P4.

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
