import { accountIdOf } from "../../fixtures/assert/identity";
import {
  expectNotebookMember,
  expectNotebookMembershipsEndedWith,
  expectOwned,
  expectOwnerless,
} from "../../fixtures/assert/notebook";
import { expectMembership } from "../../fixtures/assert/workspace";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { leave } from "../../fixtures/members";
import { addedNotebookMember, updateNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import { ownerlessNotebooks, soleAdminOfNotebooks } from "../../fixtures/ownerless";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N7, leaving a workspace and its notebooks (M3 design 3, 4; M3/P3 design
// 3.2): leaving ends the account's notebook memberships; rule two refuses
// the only admin of a notebook with another member, counting them in the
// workspace; the account's notebook of its own becomes ownerless. The page
// version comes with M3/P5.

test("N7 (API): the only admin of a notebook with another member cannot leave the workspace; once another is its admin, leaving ends every notebook membership, and the notebook of the account alone is ownerless", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const mateEmail = emailFor(testInfo, "mate");
  await joinAs(api, adminPat, workspace.slug, mateEmail, "member");
  const [ownerId, mateId] = await Promise.all([accountIdOf(db, ownerEmail), accountIdOf(db, mateEmail)]);
  const plans = await createNotebook(api, ownerPat, workspace.slug, "Plans");
  const solo = await createNotebook(api, ownerPat, workspace.slug, "Solo");
  const wiki = await createNotebook(api, adminPat, workspace.slug, "Wiki", "viewer");
  const mate = await addedNotebookMember(api, ownerPat, plans.id, mateId, "reader");
  await addedNotebookMember(api, adminPat, wiki.id, ownerId, "editor");

  const refused = await leave(api, ownerPat, workspace.slug);
  expect([refused.response.status, refused.error?.code, refused.error?.detail]).toEqual([
    409,
    "notebook.sole_admin",
    soleAdminOfNotebooks({ [workspace.slug]: 1 }),
  ]);
  await expectMembership(db, workspace.id, ownerId, "member");
  await expectNotebookMember(db, plans.id, ownerId, { role: "admin", active: true, writerId: ownerId });

  expect((await updateNotebookMember(api, ownerPat, mate.id, "admin")).response.status).toBe(200);
  expect((await leave(api, ownerPat, workspace.slug)).response.status).toBe(204);

  await expectMembership(db, workspace.id, ownerId, "ended");
  await expectNotebookMembershipsEndedWith(db, workspace.id, ownerId, ownerId, 3);
  await expectNotebookMember(db, wiki.id, ownerId, { role: "editor", active: false, writerId: ownerId });
  await expectOwned(db, plans.id);
  await expectOwned(db, wiki.id);
  await expectOwnerless(db, solo.id, ownerId);
  expect((await ownerlessNotebooks(api, adminPat, workspace.slug)).map((n) => n.id)).toEqual([solo.id]);
});
