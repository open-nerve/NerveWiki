import type { OwnerlessNotebook } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import {
  expectAuditEvents,
  expectNotebookMember,
  expectOwned,
  expectOwnerlessListed,
} from "../../fixtures/assert/notebook";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { answerTo } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { who } from "../../fixtures/member-pages";
import { memberOf, removeMember } from "../../fixtures/members";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { notebookGroups, notebookHeading, notebookPath } from "../../fixtures/notebook-pages";
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
import {
  auditLog,
  listedOwnerless,
  ownerlessListed,
  ownerlessPath,
  takeOverWith,
} from "../../fixtures/ownerless-pages";
import { expect, test } from "../../fixtures/test";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// N9, taking over an ownerless notebook (M3 design 3, 4; M3/P3 design 3.3):
// a workspace admin finds it in the list, with its former owner, since
// when, its size, and takes it over, its admin from then on; a private one
// stays private. The workspace's members and guests: the lists are
// refused, and by id the notebook is not there for them. The audit records
// the take-over (M3/P5 design 3.3 for the page).

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

test("N9 (page): a workspace admin takes the ownerless notebook over: its left column has it, the status opens it, the audit log has it; a member's settings have no ownerless notebooks, and their address says why", async ({
  anotherPage,
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { adminEmail, adminId, pat: adminPat, tokens: adminTokens, workspace } = await newOnboardedTeam(api, testInfo);
  const slug = workspace.slug;
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, slug, ownerEmail, "member");
  const mateEmail = emailFor(testInfo, "mate");
  const mateTokens = await joinOnboarded(api, adminPat, slug, mateEmail, "member");
  const [ownerId, mateId] = await Promise.all([accountIdOf(db, ownerEmail), accountIdOf(db, mateEmail)]);
  const plans = await createNotebook(api, ownerPat, slug, "Plans");
  await addedNotebookMember(api, ownerPat, plans.id, mateId, "editor");
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, slug, ownerEmail)).id)).response.status
  ).toBe(204);
  const page = await signedInPage(adminTokens);
  const listedAnswer = answerTo(page, "GET", `/api/v0/workspaces/${slug}/ownerless-notebooks`);
  await page.goto(ownerlessPath(slug));

  await expect.poll(() => ownerlessListed(page)).toEqual([listedOwnerless("Plans", ownerEmail, "Private", 1)]);
  const { data: listed } = (await (await listedAnswer).json()) as { data: OwnerlessNotebook[] };
  await Promise.all(listed.map((l) => expectOwnerlessListed(db, l)));
  await expect(page.getByText("Nothing yet.", { exact: true })).toBeVisible();
  const took = await takeOverWith(page, plans.id, "Plans", who(displayNameOf(ownerEmail), ownerEmail));
  expect(took.status()).toBe(200);
  // Private as before: the editor keeps the role, and no one else came in.
  expect(await took.json()).toMatchObject({
    id: plans.id,
    name: "Plans",
    workspace_access: "none",
    role: "admin",
    member_count: 2,
  });

  await expect(page.getByRole("status")).toHaveText("Plans taken over. Open it");
  await expect(page.getByText("No ownerless notebooks.", { exact: true })).toBeVisible();
  await expect.poll(() => notebookGroups(page, "Acme")).toEqual({ "Team notebooks": ["Plans"] });
  await expect
    .poll(() => auditLog(page))
    .toEqual([
      `${who(displayNameOf(adminEmail), adminEmail)} took over Plans; former owner: ${who(displayNameOf(ownerEmail), ownerEmail)}.`,
    ]);
  await page.getByRole("status").getByRole("link", { name: "Open it", exact: true }).click();
  await expect(notebookHeading(page, "Plans")).toBeVisible();
  await expect(page).toHaveURL(notebookPath(slug, plans.id));
  await expectOwned(db, plans.id);
  await expectNotebookMember(db, plans.id, adminId, { role: "admin", active: true, writerId: adminId });
  await expectNotebookMember(db, plans.id, mateId, { role: "editor", active: true, writerId: ownerId });
  await expectAuditEvents(db, workspace.id, [
    { action: "taken_over", notebookId: plans.id, notebookName: "Plans", formerOwnerId: ownerId, actorId: adminId },
  ]);

  // A member: the workspace's settings have no ownerless notebooks, and their address says they are the admins'.
  const matePage = await anotherPage(mateTokens);
  await matePage.goto(`/${slug}/settings/general`);
  const settings = matePage.getByRole("navigation", { name: "Workspace settings", exact: true });
  await expect(settings.getByRole("link", { name: "Members", exact: true })).toBeVisible();
  await expect(settings.getByRole("link", { name: "Ownerless notebooks", exact: true })).toHaveCount(0);
  await matePage.goto(ownerlessPath(slug));
  await expect(
    matePage.getByText("Only the workspace's admins see its ownerless notebooks.", { exact: true })
  ).toBeVisible();
});
