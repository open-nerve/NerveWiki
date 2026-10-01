import { accountIdOf } from "../../fixtures/assert/identity";
import { expectAuditEvents, expectNotebookDeletedWithItsMembers } from "../../fixtures/assert/notebook";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { who } from "../../fixtures/member-pages";
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
import {
  auditLog,
  deleteOwnerlessWith,
  listedOwnerless,
  loadMoreWith,
  ownerlessListed,
  ownerlessPath,
} from "../../fixtures/ownerless-pages";
import { expect, test } from "../../fixtures/test";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// N10, deleting an ownerless notebook (M3 design 3, 4; M3/P3 design 3.3,
// 3.4): a workspace admin deletes it, its members with it, while it is
// ownerless; the audit records it, newest first, a page at a time (M3/P5
// design 3.3 for the page).

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

test("N10 (page): a workspace admin reads the audit log a page at a time, then deletes the ownerless notebook once its name is typed; the log has it first, keeping what was loaded", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { adminEmail, adminId, pat: adminPat, tokens: adminTokens, workspace } = await newOnboardedTeam(api, testInfo);
  const slug = workspace.slug;
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, slug, ownerEmail, "member");
  const plans = await createNotebook(api, ownerPat, slug, "Plans");
  // Fifty-one more, deleted through the API: their events are a page of 50 and one past it.
  const drafts = await Promise.all(
    Array.from({ length: 51 }, (_, i) => createNotebook(api, ownerPat, slug, `Draft ${i + 1}`))
  );
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, slug, ownerEmail)).id)).response.status
  ).toBe(204);
  const deleted = await Promise.all(drafts.map((draft) => deleteOwnerless(api, adminPat, draft.id)));
  expect(deleted.map((d) => d.response.status)).toEqual(drafts.map(() => 204));
  const [admin, owner] = [displayNameOf(adminEmail), displayNameOf(ownerEmail)];
  const sentence = (name: string) => `${admin} deleted ${name} (former owner ${owner}).`;
  const page = await signedInPage(adminTokens);
  await page.goto(ownerlessPath(slug));

  await expect.poll(() => ownerlessListed(page)).toEqual([listedOwnerless("Plans", ownerEmail, "Private", 0)]);
  await expect.poll(async () => (await auditLog(page)).length).toBe(50);
  expect((await loadMoreWith(page, slug)).status()).toBe(200);
  await expect.poll(async () => (await auditLog(page)).length).toBe(51);
  expect((await auditLog(page)).toSorted()).toEqual(drafts.map((draft) => sentence(draft.name)).toSorted());
  await expect(page.getByRole("button", { name: "Load more", exact: true })).toHaveCount(0);

  expect(await deleteOwnerlessWith(page, plans.id, "Plans", who(owner, ownerEmail))).toBe(204);
  await expect(page.getByText("No ownerless notebooks.", { exact: true })).toBeVisible();
  await expectNotebookDeletedWithItsMembers(db, plans.id, adminId);

  // The log is read again: the deletion first; what was loaded past the first page stays (M3/P5 review Q2).
  await expect.poll(async () => (await auditLog(page))[0]).toBe(sentence("Plans"));
  expect((await auditLog(page)).toSorted()).toEqual(
    ["Plans", ...drafts.map((draft) => draft.name)].map(sentence).toSorted()
  );
  await expect(page.getByRole("button", { name: "Load more", exact: true })).toHaveCount(0);
});
