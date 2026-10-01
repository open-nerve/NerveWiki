import { accountIdOf } from "../../fixtures/assert/identity";
import { expectAuditEvents, expectNotebookDeletedWithItsMembers } from "../../fixtures/assert/notebook";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { memberOf, removeMember } from "../../fixtures/members";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import {
  auditEvents,
  deleteOwnerless,
  eventOf,
  listAuditEvents,
  ownerlessNotebooks,
  takeOver,
} from "../../fixtures/ownerless";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N10, deleting an ownerless notebook (M3 design 3, 4; M3/P3 design 3.3,
// 3.4): a workspace admin deletes it, its members with it, while it is
// ownerless; the audit records it, newest first, a page at a time. The
// page version comes with M3/P5.

test("N10 (API): a workspace admin deletes ownerless notebooks, their members with them, and not one taken over; the audit lists the deletions and the take-over newest first, a page at a time", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat: adminPat, workspace } = await newTeam(api, testInfo);
  const slug = workspace.slug;
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, slug, ownerEmail, "member");
  const mateEmail = emailFor(testInfo, "mate");
  const matePat = await joinAs(api, adminPat, slug, mateEmail, "member");
  const [ownerId, mateId] = await Promise.all([accountIdOf(db, ownerEmail), accountIdOf(db, mateEmail)]);
  const plans = await createNotebook(api, ownerPat, slug, "Plans");
  const solo = await createNotebook(api, ownerPat, slug, "Solo");
  const drafts = await createNotebook(api, ownerPat, slug, "Drafts");
  await addedNotebookMember(api, ownerPat, plans.id, mateId, "editor");
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, slug, ownerEmail)).id)).response.status
  ).toBe(204);
  expect((await ownerlessNotebooks(api, adminPat, slug)).map((n) => n.name).toSorted()).toEqual([
    "Drafts",
    "Plans",
    "Solo",
  ]);

  expect((await deleteOwnerless(api, adminPat, solo.id)).response.status).toBe(204);
  await expectNotebookDeletedWithItsMembers(db, solo.id, adminId);
  // Its editor cannot delete plans; once taken over, drafts is no longer
  // ownerless, and is deleted as any notebook is, by its admin.
  expect((await takeOver(api, adminPat, drafts.id)).response.status).toBe(200);
  const refused = await Promise.all([
    deleteOwnerless(api, adminPat, solo.id),
    deleteOwnerless(api, matePat, plans.id),
    deleteOwnerless(api, adminPat, drafts.id),
  ]);
  expect(refused.map((a) => [a.response.status, a.error?.code])).toEqual([
    [404, "notebook.not_found"],
    [404, "notebook.not_found"],
    [404, "notebook.not_found"],
  ]);
  expect((await deleteOwnerless(api, adminPat, plans.id)).response.status).toBe(204);
  await expectNotebookDeletedWithItsMembers(db, plans.id, adminId);
  expect(await ownerlessNotebooks(api, adminPat, slug)).toEqual([]);

  const first = await listAuditEvents(api, adminPat, slug, { limit: 2 });
  expect([first.response.status, first.data?.data.map(eventOf), typeof first.data?.next_cursor]).toEqual([
    200,
    [
      ["deleted", "Plans", ownerEmail, adminEmail],
      ["taken_over", "Drafts", ownerEmail, adminEmail],
    ],
    "string",
  ]);
  const second = await listAuditEvents(api, adminPat, slug, { limit: 2, cursor: first.data?.next_cursor ?? "" });
  expect([second.response.status, second.data]).toEqual([
    200,
    {
      data: [expect.objectContaining({ action: "deleted", notebook_id: solo.id, notebook_name: "Solo" })],
      next_cursor: null,
    },
  ]);
  expect((await auditEvents(api, adminPat, slug, 1)).map((e) => e.notebook_name)).toEqual(["Plans", "Drafts", "Solo"]);
  // A cursor the server did not give is the request's fault; a page size
  // out of 1 to 100 is a field's.
  const wrong = await Promise.all([
    listAuditEvents(api, adminPat, slug, { cursor: "not-a-cursor" }),
    listAuditEvents(api, adminPat, slug, { limit: 0 }),
    listAuditEvents(api, adminPat, slug, { limit: 101 }),
  ]);
  expect(wrong.map((a) => [a.response.status, a.error?.code])).toEqual([
    [400, "bad_request"],
    [422, "validation_failed"],
    [422, "validation_failed"],
  ]);

  await expectAuditEvents(db, workspace.id, [
    { action: "deleted", notebookId: solo.id, notebookName: "Solo", formerOwnerId: ownerId, actorId: adminId },
    { action: "taken_over", notebookId: drafts.id, notebookName: "Drafts", formerOwnerId: ownerId, actorId: adminId },
    { action: "deleted", notebookId: plans.id, notebookName: "Plans", formerOwnerId: ownerId, actorId: adminId },
  ]);
});
