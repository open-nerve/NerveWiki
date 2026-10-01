import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNotebookMember } from "../../fixtures/assert/notebook";
import { emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { addedNotebookMember, leaveNotebook } from "../../fixtures/notebook-members";
import { leaveNotebookWith, notebookGroups, notebookPath, workspaceNav } from "../../fixtures/notebook-pages";
import { createNotebook, getNotebook, updateNotebook } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { workspaceHeading } from "../../fixtures/workspace-pages";
import { newTeam } from "../../fixtures/workspaces";

// N5, leaving a notebook (M3 design 3, rule one): a member leaves; the
// only admin cannot, even alone in it, until another is an admin (M3/P4
// design 3.4 for the page).

test("N5 (API): a member leaves; the only admin cannot, even alone, until another member is an admin", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const mateEmail = emailFor(testInfo, "mate");
  const matePat = await joinAs(api, adminPat, workspace.slug, mateEmail, "member");
  const [ownerId, mateId] = await Promise.all([accountIdOf(db, ownerEmail), accountIdOf(db, mateEmail)]);
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Plans");

  const alone = await leaveNotebook(api, ownerPat, notebook.id);
  expect([alone.response.status, alone.error?.code]).toEqual([409, "notebook.sole_admin"]);

  // A reader leaves, and no longer sees the private notebook.
  await addedNotebookMember(api, ownerPat, notebook.id, mateId, "reader");
  expect((await leaveNotebook(api, matePat, notebook.id)).response.status).toBe(204);
  await expectNotebookMember(db, notebook.id, mateId, { role: "reader", active: false, writerId: mateId });
  const gone = await getNotebook(api, matePat, notebook.id);
  expect([gone.response.status, gone.error?.code]).toEqual([404, "notebook.not_found"]);

  // One who uses it by its workspace access alone has no membership to end.
  expect((await updateNotebook(api, ownerPat, notebook.id, { workspace_access: "viewer" })).response.status).toBe(200);
  const byAccess = await leaveNotebook(api, adminPat, notebook.id);
  expect([byAccess.response.status, byAccess.error?.code]).toEqual([404, "notebook.member_not_found"]);

  // With a second admin, the first may leave.
  await addedNotebookMember(api, ownerPat, notebook.id, mateId, "admin");
  expect((await leaveNotebook(api, ownerPat, notebook.id)).response.status).toBe(204);
  await expectNotebookMember(db, notebook.id, ownerId, { role: "admin", active: false, writerId: ownerId });
  const last = await leaveNotebook(api, matePat, notebook.id);
  expect([last.response.status, last.error?.code]).toEqual([409, "notebook.sole_admin"]);
});

test("N5 (page): a member leaves a private notebook, and lands on the workspace's home, its heading focused", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const leaverEmail = emailFor(testInfo, "leaver");
  const page = await signedInPage(await joinOnboarded(api, adminPat, workspace.slug, leaverEmail, "member"));
  const leaverId = await accountIdOf(db, leaverEmail);
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Diary");
  await addedNotebookMember(api, ownerPat, notebook.id, leaverId, "editor");
  await page.goto(notebookPath(workspace.slug, notebook.id, "members"));

  expect(await leaveNotebookWith(page, notebook.id)).toBe(204);

  await expect(workspaceHeading(page, "Acme")).toBeFocused();
  await expect(page).toHaveURL(`/${workspace.slug}`);
  await expect(workspaceNav(page, "Acme").getByText("No notebooks yet.", { exact: true })).toBeVisible();
  expect(await notebookGroups(page, "Acme")).toEqual({});
  await expectNotebookMember(db, notebook.id, leaverId, { role: "editor", active: false, writerId: leaverId });
});

test("N5 (page): the only admin cannot leave, alone in it: the dialog says why, and the notebook stays", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerTokens = await joinOnboarded(api, adminPat, workspace.slug, ownerEmail, "member");
  const notebook = await createNotebook(api, ownerTokens.access_token, workspace.slug, "Diary");
  const page = await signedInPage(ownerTokens);
  await page.goto(notebookPath(workspace.slug, notebook.id, "members"));

  expect(await leaveNotebookWith(page, notebook.id)).toBe(409);

  await expect(page.getByRole("alertdialog").getByRole("alert")).toHaveText(
    "You are this notebook's only admin: make another member an admin first, or delete the notebook."
  );
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  await expectNotebookMember(db, notebook.id, await accountIdOf(db, ownerEmail), {
    role: "admin",
    active: true,
    writerId: await accountIdOf(db, ownerEmail),
  });
});
