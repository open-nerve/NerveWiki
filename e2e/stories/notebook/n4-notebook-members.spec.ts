import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNotebookMember } from "../../fixtures/assert/notebook";
import { displayNameOf, emailFor, register } from "../../fixtures/auth";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { membersListed, roleOf, who } from "../../fixtures/member-pages";
import {
  addedNotebookMember,
  addNotebookMember,
  listNotebookMembers,
  removeNotebookMember,
  updateNotebookMember,
} from "../../fixtures/notebook-members";
import {
  addNotebookMemberWith,
  changeNotebookRoleWith,
  notebookGroups,
  notebookPath,
  removeNotebookMemberWith,
} from "../../fixtures/notebook-pages";
import { createNotebook, getNotebook, listNotebooks } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N4, a notebook's members (M3 design 3): its admin adds the workspace's
// members, a guest included, changes their roles and removes them (M3/P4
// design 3.4 for the page).

test("N4 (API): the admin adds, changes and removes members; a role is the higher of the two; others are refused", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const mateEmail = emailFor(testInfo, "mate");
  const matePat = await joinAs(api, adminPat, workspace.slug, mateEmail, "member");
  const guestEmail = emailFor(testInfo, "guest");
  const guestPat = await joinAs(api, adminPat, workspace.slug, guestEmail, "guest");
  const outsiderEmail = emailFor(testInfo, "outsider");
  await register(api, outsiderEmail);
  const [ownerId, mateId, guestId, outsiderId] = await Promise.all([
    accountIdOf(db, ownerEmail),
    accountIdOf(db, mateEmail),
    accountIdOf(db, guestEmail),
    accountIdOf(db, outsiderEmail),
  ]);
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Team", "editor");

  // A reader of a notebook open as editor edits it: the higher of the two.
  // A guest has its membership's role alone.
  await addedNotebookMember(api, ownerPat, notebook.id, mateId, "reader");
  const guestMembership = await addedNotebookMember(api, ownerPat, notebook.id, guestId, "reader");
  // A new account's display name is its address's local part.
  expect(guestMembership).toMatchObject({ user_id: guestId, role: "reader", display_name: guestEmail.split("@")[0] });
  const roles = await Promise.all([matePat, guestPat].map((pat) => getNotebook(api, pat, notebook.id)));
  expect(roles.map((r) => r.data?.role)).toEqual(["editor", "reader"]);
  // The workspace's list gives each the role the read does (M3 Codex review R5).
  const listed = await Promise.all([matePat, guestPat].map((pat) => listNotebooks(api, pat, workspace.slug)));
  expect(listed.map((list) => list.find((n) => n.id === notebook.id)?.role)).toEqual(["editor", "reader"]);
  await expectNotebookMember(db, notebook.id, mateId, { role: "reader", active: true, writerId: ownerId });
  const members = await listNotebookMembers(api, ownerPat, notebook.id);
  expect(members.map((m) => [m.user_id, m.role])).toEqual([
    [ownerId, "admin"],
    [mateId, "reader"],
    [guestId, "reader"],
  ]);
  // A guest of the workspace lists them without their emails.
  const guestsList = await listNotebookMembers(api, guestPat, notebook.id);
  expect(guestsList.map((m) => m.email)).toEqual([null, null, null]);

  // Only the admin manages members; not their own, not anyone outside the
  // workspace, not one twice.
  const refused = await Promise.all([
    addNotebookMember(api, matePat, notebook.id, outsiderId, "reader"),
    updateNotebookMember(api, matePat, guestMembership.id, "editor"),
    removeNotebookMember(api, matePat, guestMembership.id),
  ]);
  expect(refused.map((r) => [r.response.status, r.error?.code])).toEqual([
    [403, "forbidden"],
    [403, "forbidden"],
    [403, "forbidden"],
  ]);
  const outsider = await addNotebookMember(api, ownerPat, notebook.id, outsiderId, "reader");
  const twice = await addNotebookMember(api, ownerPat, notebook.id, mateId, "editor");
  expect(
    [outsider, twice].map((r) => [r.response.status, r.error?.errors?.map((e) => `${e.field} ${e.code}`)])
  ).toEqual([
    [422, ["user_id not_allowed"]],
    [422, ["user_id duplicate"]],
  ]);
  const ownMembership = members[0];
  const own = await updateNotebookMember(api, ownerPat, ownMembership?.id ?? "", "reader");
  expect([own.response.status, own.error?.code]).toEqual([409, "notebook.own_membership"]);

  // The guest becomes an editor, is removed and loses the notebook, then
  // comes back as a reader with the same membership, joined when first.
  const changed = await updateNotebookMember(api, ownerPat, guestMembership.id, "editor");
  expect(changed.data).toMatchObject({ id: guestMembership.id, role: "editor" });
  await expectNotebookMember(db, notebook.id, guestId, { role: "editor", active: true, writerId: ownerId });
  expect((await removeNotebookMember(api, ownerPat, guestMembership.id)).response.status).toBe(204);
  await expectNotebookMember(db, notebook.id, guestId, { role: "editor", active: false, writerId: ownerId });
  // An ended membership is no longer one to change or remove.
  const ended = await Promise.all([
    updateNotebookMember(api, ownerPat, guestMembership.id, "reader"),
    removeNotebookMember(api, ownerPat, guestMembership.id),
  ]);
  expect(ended.map((r) => [r.response.status, r.error?.code])).toEqual([
    [404, "notebook.member_not_found"],
    [404, "notebook.member_not_found"],
  ]);
  await expectNotebookMember(db, notebook.id, guestId, { role: "editor", active: false, writerId: ownerId });
  const gone = await getNotebook(api, guestPat, notebook.id);
  expect([gone.response.status, gone.error?.code]).toEqual([404, "notebook.not_found"]);
  const back = await addedNotebookMember(api, ownerPat, notebook.id, guestId, "reader");
  expect(back).toMatchObject({ id: guestMembership.id, role: "reader", created_at: guestMembership.created_at });
  await expectNotebookMember(db, notebook.id, guestId, {
    role: "reader",
    active: true,
    writerId: ownerId,
    joinedAt: guestMembership.created_at,
  });
});

test("N4 (page): the admin adds a member and a guest, changes a role and removes one; their own row has no controls", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerTokens = await joinOnboarded(api, adminPat, workspace.slug, ownerEmail, "member");
  const mateEmail = emailFor(testInfo, "mate");
  const guestEmail = emailFor(testInfo, "guest");
  await joinAs(api, adminPat, workspace.slug, mateEmail, "member");
  await joinAs(api, adminPat, workspace.slug, guestEmail, "guest");
  const [ownerId, mateId, guestId] = await Promise.all([
    accountIdOf(db, ownerEmail),
    accountIdOf(db, mateEmail),
    accountIdOf(db, guestEmail),
  ]);
  const notebook = await createNotebook(api, ownerTokens.access_token, workspace.slug, "Team");
  const page = await signedInPage(ownerTokens);
  await page.goto(notebookPath(workspace.slug, notebook.id, "members"));
  await expect.poll(() => membersListed(page)).toHaveLength(1);
  await expect(roleOf(page, who(displayNameOf(ownerEmail), ownerEmail))).toHaveCount(0);

  const mate = who(displayNameOf(mateEmail), mateEmail);
  const guest = who(displayNameOf(guestEmail), guestEmail);
  expect((await addNotebookMemberWith(page, notebook.id, mate, "Editor")).status).toBe(201);
  await expect(page.getByText(`${displayNameOf(mateEmail)} added.`, { exact: true })).toBeVisible();
  const { status, added } = await addNotebookMemberWith(page, notebook.id, guest, "Reader");
  expect(status).toBe(201);
  await expect.poll(() => membersListed(page)).toHaveLength(3);
  // With other members, the notebook is the team's.
  await expect.poll(() => notebookGroups(page, "Acme")).toEqual({ "Team notebooks": ["Team"] });

  expect((await changeNotebookRoleWith(page, mate, "Admin")).status()).toBe(200);
  await expect(roleOf(page, mate)).toHaveText("Admin");
  await expect(roleOf(page, mate)).toBeFocused();
  await expectNotebookMember(db, notebook.id, mateId, { role: "admin", active: true, writerId: ownerId });

  expect(await removeNotebookMemberWith(page, guest, added.id)).toBe(204);
  await expect(page.getByRole("heading", { level: 2, name: "Members", exact: true })).toBeFocused();
  await expect.poll(() => membersListed(page)).toHaveLength(2);
  await expectNotebookMember(db, notebook.id, guestId, { role: "reader", active: false, writerId: ownerId });
});

test("N4 (page): an editor sees the members and their roles, and can change nothing", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "owner"), "member");
  const editorEmail = emailFor(testInfo, "editor");
  const page = await signedInPage(await joinOnboarded(api, adminPat, workspace.slug, editorEmail, "member"));
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Team");
  await addedNotebookMember(api, ownerPat, notebook.id, await accountIdOf(db, editorEmail), "editor");

  await page.goto(notebookPath(workspace.slug, notebook.id, "members"));

  await expect.poll(() => membersListed(page)).toHaveLength(2);
  await expect(page.getByText("Admin", { exact: true })).toBeVisible();
  await expect(page.getByText("Editor", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: /, role of / })).toHaveCount(0);
  await expect(page.getByRole("button", { name: /^Remove / })).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Add a member" })).toHaveCount(0);
});
