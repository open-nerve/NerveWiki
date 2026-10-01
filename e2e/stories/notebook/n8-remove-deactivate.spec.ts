import { nervewikiUsers, nervewikiUsersFails } from "../../fixtures/admin";
import { accountIdOf, expectDeactivated } from "../../fixtures/assert/identity";
import { expectNotebookMembershipsEndedWith, expectOwnerless } from "../../fixtures/assert/notebook";
import { expectMembership } from "../../fixtures/assert/workspace";
import { bearer, emailFor } from "../../fixtures/auth";
import { accept, invite, joinAs } from "../../fixtures/invitations";
import { memberOf, removeMember } from "../../fixtures/members";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook, getNotebook, listNotebooks } from "../../fixtures/notebooks";
import { soleAdminOfNotebooks } from "../../fixtures/ownerless";
import { expect, test } from "../../fixtures/test";
import { createWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// N8, removal and deactivation and the notebooks (M3 design 3, 4; M3/P3
// design 3.2): a workspace admin's removal of the only admin of a notebook
// is not refused, and the notebook becomes ownerless, its other members
// using it as before; the account's deactivation is refused as its leaving
// is (N7), through the API and by users deactivate, with the reason; the
// way out is a workspace admin's removal. The page version comes with
// M3/P5.

test("N8 (API): the account's deactivation is refused while it is the only admin of a notebook with another member; a workspace admin removes it, the notebook is ownerless, its reader reads it still, and the deactivation goes through, leaving the account's notebook of its own ownerless", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const mateEmail = emailFor(testInfo, "mate");
  const matePat = await joinAs(api, adminPat, workspace.slug, mateEmail, "member");
  const [ownerId, mateId] = await Promise.all([accountIdOf(db, ownerEmail), accountIdOf(db, mateEmail)]);
  const plans = await createNotebook(api, ownerPat, workspace.slug, "Plans");
  await addedNotebookMember(api, ownerPat, plans.id, mateId, "reader");
  // A notebook of the account alone, in another workspace, does not block.
  const lab = await createWorkspace(api, adminPat, "Lab", slugFor(testInfo, "lab"));
  await accept(api, ownerPat, await invite(api, adminPat, lab.slug, ownerEmail, "member"));
  const solo = await createNotebook(api, ownerPat, lab.slug, "Solo");

  const refused = await api.POST("/api/v0/me/deactivate", { headers: bearer(ownerPat) });
  expect([refused.response.status, refused.error?.code, refused.error?.detail]).toEqual([
    409,
    "notebook.sole_admin",
    soleAdminOfNotebooks({ [workspace.slug]: 1 }),
  ]);
  await expectMembership(db, workspace.id, ownerId, "member");

  const removed = await removeMember(api, adminPat, (await memberOf(api, adminPat, workspace.slug, ownerEmail)).id);
  expect(removed.response.status).toBe(204);
  await expectMembership(db, workspace.id, ownerId, "ended");
  await expectNotebookMembershipsEndedWith(db, workspace.id, ownerId, adminId, 1);
  await expectOwnerless(db, plans.id, ownerId);
  // The reader uses plans as before.
  const read = await getNotebook(api, matePat, plans.id);
  expect([read.response.status, read.data?.role, read.data?.member_count]).toEqual([200, "reader", 1]);
  expect((await listNotebooks(api, matePat, workspace.slug)).map((n) => n.id)).toEqual([plans.id]);

  expect((await api.POST("/api/v0/me/deactivate", { headers: bearer(ownerPat) })).response.status).toBe(204);
  await expectDeactivated(db, ownerId);
  await expectMembership(db, lab.id, ownerId, "ended");
  await expectNotebookMembershipsEndedWith(db, lab.id, ownerId, ownerId, 1);
  await expectOwnerless(db, solo.id, ownerId);
});

test("N8 (command line): users deactivate refuses the only admin of notebooks with other members, counting them in each workspace; removed from both, the account is deactivated", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat: adminPat, workspace } = await newTeam(api, testInfo);
  const lab = await createWorkspace(api, adminPat, "Lab", slugFor(testInfo, "lab"));
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const mateEmail = emailFor(testInfo, "mate");
  const matePat = await joinAs(api, adminPat, workspace.slug, mateEmail, "member");
  await accept(api, ownerPat, await invite(api, adminPat, lab.slug, ownerEmail, "member"));
  await accept(api, matePat, await invite(api, adminPat, lab.slug, mateEmail, "guest"));
  const [ownerId, mateId] = await Promise.all([accountIdOf(db, ownerEmail), accountIdOf(db, mateEmail)]);
  const blocking = [
    await createNotebook(api, ownerPat, workspace.slug, "Plans"),
    await createNotebook(api, ownerPat, lab.slug, "Trials"),
    await createNotebook(api, ownerPat, lab.slug, "Results"),
  ];
  await Promise.all(blocking.map((n) => addedNotebookMember(api, ownerPat, n.id, mateId, "reader")));
  const solo = await createNotebook(api, ownerPat, lab.slug, "Solo");

  await nervewikiUsersFails(
    db,
    ["deactivate", "--email", ownerEmail],
    soleAdminOfNotebooks({ [workspace.slug]: 1, [lab.slug]: 2 })
  );
  await expectMembership(db, workspace.id, ownerId, "member");
  await expectMembership(db, lab.id, ownerId, "member");

  const removals = await Promise.all(
    [workspace, lab].map(async (w) =>
      removeMember(api, adminPat, (await memberOf(api, adminPat, w.slug, ownerEmail)).id)
    )
  );
  expect(removals.map((r) => r.response.status)).toEqual([204, 204]);
  await nervewikiUsers(db, ["deactivate", "--email", ownerEmail]);

  await expectDeactivated(db, ownerId);
  await expectNotebookMembershipsEndedWith(db, workspace.id, ownerId, adminId, 1);
  await expectNotebookMembershipsEndedWith(db, lab.id, ownerId, adminId, 3);
  await Promise.all([...blocking, solo].map((n) => expectOwnerless(db, n.id, ownerId)));
});
