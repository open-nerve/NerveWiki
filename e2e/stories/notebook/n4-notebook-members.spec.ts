import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNotebookMember } from "../../fixtures/assert/notebook";
import { emailFor, register } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import {
  addedNotebookMember,
  addNotebookMember,
  listNotebookMembers,
  removeNotebookMember,
  updateNotebookMember,
} from "../../fixtures/notebook-members";
import { createNotebook, getNotebook } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N4, a notebook's members (M3 design 3): its admin adds the workspace's
// members, a guest included, changes their roles and removes them. The
// page version comes with M3/P4.

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
