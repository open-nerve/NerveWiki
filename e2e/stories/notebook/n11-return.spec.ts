import { nervewikiWorkspaces } from "../../fixtures/admin";
import { accountIdOf } from "../../fixtures/assert/identity";
import { expectAuditEvents, expectNotebookMember, expectOwned } from "../../fixtures/assert/notebook";
import { expectMembership, membershipEndedAt } from "../../fixtures/assert/workspace";
import { emailFor } from "../../fixtures/auth";
import { accept, invite, joinAs } from "../../fixtures/invitations";
import { memberOf, removeMember } from "../../fixtures/members";
import { createNotebook, getNotebook } from "../../fixtures/notebooks";
import { auditEvents, deleteOwnerless, eventOf, takeOver } from "../../fixtures/ownerless";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N11, the return (M3 design 3, 4; M3/P3 design 3.2, 3.5): the former
// owner, back in the workspace by an invitation or by workspaces
// reactivate-member, gets back the ownerless notebooks that no workspace
// admin took over or deleted meanwhile, each recorded; the others stay as
// the admins left them. The page version comes with M3/P5.

test("N11 (API): the former owner, back by an invitation as a guest, gets back its ownerless notebook, recorded as returned; the one taken over stays its taker's, the one deleted deleted", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat: adminPat, workspace } = await newTeam(api, testInfo);
  const slug = workspace.slug;
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, slug, ownerEmail, "member");
  const ownerId = await accountIdOf(db, ownerEmail);
  const plans = await createNotebook(api, ownerPat, slug, "Plans");
  const solo = await createNotebook(api, ownerPat, slug, "Solo");
  const gone = await createNotebook(api, ownerPat, slug, "Gone");
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, slug, ownerEmail)).id)).response.status
  ).toBe(204);
  expect((await takeOver(api, adminPat, plans.id)).response.status).toBe(200);
  expect((await deleteOwnerless(api, adminPat, gone.id)).response.status).toBe(204);

  await accept(api, ownerPat, await invite(api, adminPat, slug, ownerEmail, "guest"));

  await expectMembership(db, workspace.id, ownerId, "guest");
  await expectOwned(db, solo.id);
  await expectNotebookMember(db, solo.id, ownerId, {
    role: "admin",
    active: true,
    writerId: ownerId,
    joinedAt: solo.created_at,
  });
  const read = await getNotebook(api, ownerPat, solo.id);
  expect([read.response.status, read.data?.role]).toEqual([200, "admin"]);
  // The taken over and the deleted are not the owner's to come back to.
  await expectOwned(db, plans.id);
  await expectNotebookMember(db, plans.id, ownerId, { role: "admin", active: false, writerId: adminId });
  const refused = await Promise.all([getNotebook(api, ownerPat, plans.id), getNotebook(api, ownerPat, gone.id)]);
  expect(refused.map((a) => [a.response.status, a.error?.code])).toEqual([
    [404, "notebook.not_found"],
    [404, "notebook.not_found"],
  ]);

  expect((await auditEvents(api, adminPat, slug)).map(eventOf)).toEqual([
    ["returned", "Solo", ownerEmail, ownerEmail],
    ["deleted", "Gone", ownerEmail, adminEmail],
    ["taken_over", "Plans", ownerEmail, adminEmail],
  ]);
  await expectAuditEvents(db, workspace.id, [
    { action: "taken_over", notebookId: plans.id, notebookName: "Plans", formerOwnerId: ownerId, actorId: adminId },
    { action: "deleted", notebookId: gone.id, notebookName: "Gone", formerOwnerId: ownerId, actorId: adminId },
    { action: "returned", notebookId: solo.id, notebookName: "Solo", formerOwnerId: ownerId, actorId: ownerId },
  ]);
});

test("N11 (command line): workspaces reactivate-member returns the ownerless notebooks and says how many", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const slug = workspace.slug;
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, slug, ownerEmail, "member");
  const ownerId = await accountIdOf(db, ownerEmail);
  const plans = await createNotebook(api, ownerPat, slug, "Plans");
  const solo = await createNotebook(api, ownerPat, slug, "Solo");
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, slug, ownerEmail)).id)).response.status
  ).toBe(204);

  const ended = await membershipEndedAt(db, workspace.id, ownerId);

  expect(await nervewikiWorkspaces(db, ["reactivate-member", "--workspace", slug, "--email", ownerEmail])).toBe(
    `reactivated ${ownerEmail} in ${slug} as member; the membership had ended at ${ended}; ownerless notebooks returned: 2\n`
  );
  await expectMembership(db, workspace.id, ownerId, "member");
  await Promise.all(
    [plans, solo].flatMap((n) => [
      expectOwned(db, n.id),
      expectNotebookMember(db, n.id, ownerId, {
        role: "admin",
        active: true,
        writerId: ownerId,
        joinedAt: n.created_at,
      }),
    ])
  );
  // The command acts as the owner: the returns are recorded as theirs.
  await expectAuditEvents(db, workspace.id, [
    { action: "returned", notebookId: plans.id, notebookName: "Plans", formerOwnerId: ownerId, actorId: ownerId },
    { action: "returned", notebookId: solo.id, notebookName: "Solo", formerOwnerId: ownerId, actorId: ownerId },
  ]);
});
