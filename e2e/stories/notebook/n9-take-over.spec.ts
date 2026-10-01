import { accountIdOf } from "../../fixtures/assert/identity";
import {
  expectAuditEvents,
  expectNotebookMember,
  expectOwned,
  expectOwnerlessListed,
} from "../../fixtures/assert/notebook";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { memberOf, removeMember } from "../../fixtures/members";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook, getNotebook } from "../../fixtures/notebooks";
import {
  auditEvents,
  deleteOwnerless,
  eventOf,
  listAuditEvents,
  listOwnerless,
  ownerlessNotebooks,
  profileOf,
  takeOver,
} from "../../fixtures/ownerless";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N9, taking over an ownerless notebook (M3 design 3, 4; M3/P3 design 3.3):
// a workspace admin finds it in the list, with its former owner, since
// when, its size, and takes it over, its admin from then on; a private one
// stays private. The workspace's members and guests: the lists are
// refused, and by id the notebook is not there for them. The audit records
// the take-over. The page version comes with M3/P5.

test("N9 (API): a workspace admin lists the ownerless notebook, takes it over and is its admin, private as before; members and guests are refused, the list 403, by id 404; the audit has the take-over", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat: adminPat, workspace } = await newTeam(api, testInfo);
  const slug = workspace.slug;
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, slug, ownerEmail, "member");
  const mateEmail = emailFor(testInfo, "mate");
  const matePat = await joinAs(api, adminPat, slug, mateEmail, "member");
  const guestEmail = emailFor(testInfo, "guest");
  const guestPat = await joinAs(api, adminPat, slug, guestEmail, "guest");
  const [ownerId, mateId, guestId] = await Promise.all([
    accountIdOf(db, ownerEmail),
    accountIdOf(db, mateEmail),
    accountIdOf(db, guestEmail),
  ]);
  const plans = await createNotebook(api, ownerPat, slug, "Plans");
  await addedNotebookMember(api, ownerPat, plans.id, mateId, "editor");
  await addedNotebookMember(api, ownerPat, plans.id, guestId, "reader");
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, slug, ownerEmail)).id)).response.status
  ).toBe(204);

  // Its members, the guest one too, are refused the lists, and by id it is
  // not there, as for a notebook that is not.
  const answers = await Promise.all(
    [matePat, guestPat].flatMap((pat) => [
      listOwnerless(api, pat, slug),
      listAuditEvents(api, pat, slug),
      takeOver(api, pat, plans.id),
      deleteOwnerless(api, pat, plans.id),
    ])
  );
  const refused = [
    [403, "forbidden"],
    [403, "forbidden"],
    [404, "notebook.not_found"],
    [404, "notebook.not_found"],
  ];
  expect(answers.map((a) => [a.response.status, a.error?.code])).toEqual([...refused, ...refused]);

  const listed = await ownerlessNotebooks(api, adminPat, slug);
  expect(listed).toEqual([
    {
      id: plans.id,
      name: "Plans",
      workspace_access: "none",
      member_count: 2,
      former_owner: profileOf(ownerId, ownerEmail),
      ownerless_since: expect.any(String),
      last_activity_at: expect.any(String),
      size_bytes: 0,
    },
  ]);
  await Promise.all(listed.map((l) => expectOwnerlessListed(db, l)));

  const took = await takeOver(api, adminPat, plans.id);
  expect(took.response.status).toBe(200);
  expect(took.data).toMatchObject({
    id: plans.id,
    name: "Plans",
    workspace_access: "none",
    role: "admin",
    member_count: 3,
  });
  await expectOwned(db, plans.id);
  await expectNotebookMember(db, plans.id, adminId, { role: "admin", active: true, writerId: adminId });
  expect(await ownerlessNotebooks(api, adminPat, slug)).toEqual([]);
  const again = await takeOver(api, adminPat, plans.id);
  expect([again.response.status, again.error?.code]).toEqual([404, "notebook.not_found"]);
  // Private as before: the editor and the reader keep their roles, and no one else came in.
  const read = await getNotebook(api, guestPat, plans.id);
  expect([read.response.status, read.data?.role, read.data?.workspace_access]).toEqual([200, "reader", "none"]);

  expect((await auditEvents(api, adminPat, slug)).map(eventOf)).toEqual([
    ["taken_over", "Plans", ownerEmail, adminEmail],
  ]);
  await expectAuditEvents(db, workspace.id, [
    { action: "taken_over", notebookId: plans.id, notebookName: "Plans", formerOwnerId: ownerId, actorId: adminId },
  ]);
});
