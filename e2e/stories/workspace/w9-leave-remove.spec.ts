import { accountIdOf } from "../../fixtures/assert/identity";
import { expectInvitation, expectMembership } from "../../fixtures/assert/workspace";
import { bearer, displayNameOf, emailFor, registerOnboarded } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { accept, invite, joinAs, tryAccept } from "../../fixtures/invitations";
import { leaveWith, membersListed, removeMemberWith, who } from "../../fixtures/member-pages";
import { leave, memberOf, removeMember } from "../../fixtures/members";
import { expect, test } from "../../fixtures/test";
import { nervewikiUsers } from "../../fixtures/admin";
import { expectCreatePage, workspaceHeading } from "../../fixtures/workspace-pages";
import { createWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// W9, memberships ending (M2/P2 design 3.2; M2/P3 design 3.4): removed by an
// admin, or left; the pending invitations to the address go with them.

test("W9 (API): the admin removes a member, a guest leaves, the only admin cannot; an invitation pending to the removed one goes too", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const slug = workspace.slug;
  const memberEmail = emailFor(testInfo, "member");
  const guestEmail = emailFor(testInfo, "guest");
  const member = await joinAs(api, pat, slug, memberEmail, "member");
  const guest = await joinAs(api, pat, slug, guestEmail, "guest");
  const memberId = await accountIdOf(db, memberEmail);

  // The member takes an address with an invitation still pending: the
  // server's administrator gives it to them.
  const newEmail = emailFor(testInfo, "renamed");
  const pending = await invite(api, pat, slug, newEmail, "admin");
  await nervewikiUsers(db, ["set-email", "--email", memberEmail, "--new-email", newEmail]);

  const removed = await removeMember(api, pat, (await memberOf(api, pat, slug, newEmail)).id);
  expect(removed.response.status).toBe(204);
  await expectMembership(db, workspace.id, memberId, "ended");
  await expectInvitation(db, pending.id, "deleted", adminId);
  // No longer a member, and the link cannot bring them back.
  const after = await Promise.all([
    api.GET("/api/v0/workspaces/{slug}", { params: { path: { slug } }, headers: bearer(member) }),
    tryAccept(api, member, pending),
  ]);
  expect(after.map((a) => [a.response.status, a.error?.code])).toEqual([
    [404, "workspace.not_found"],
    [404, "workspace.invitation_not_found"],
  ]);

  expect((await leave(api, guest, slug)).response.status).toBe(204);
  await expectMembership(db, workspace.id, await accountIdOf(db, guestEmail), "ended");
  const gone = await api.GET("/api/v0/workspaces", { headers: bearer(guest) });
  expect(gone.data).toEqual({ data: [] });

  const sole = await leave(api, pat, slug);
  expect([sole.response.status, sole.error?.code]).toEqual([409, "workspace.sole_admin"]);
  await expectMembership(db, workspace.id, adminId, "admin");
});

test("W9 (page): the admin removes the other member, whose invitation pending goes too; then, the only admin, cannot leave", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const adminEmail = emailFor(testInfo, "admin");
  const tokens = await registerOnboarded(api, adminEmail);
  const adminId = await accountIdOf(db, adminEmail);
  const workspace = await createWorkspace(api, tokens.access_token, "Acme", slugFor(testInfo));
  const memberEmail = emailFor(testInfo, "member");
  await joinAs(api, tokens.access_token, workspace.slug, memberEmail, "member");
  const memberId = await accountIdOf(db, memberEmail);
  // The member takes an address with an invitation still pending, as in the API version.
  const newEmail = emailFor(testInfo, "renamed");
  const pending = await invite(api, tokens.access_token, workspace.slug, newEmail, "admin");
  await nervewikiUsers(db, ["set-email", "--email", memberEmail, "--new-email", newEmail]);
  const membership = await memberOf(api, tokens.access_token, workspace.slug, newEmail);
  const page = await signedInPage(tokens);
  await page.goto(`/${workspace.slug}/settings/members`);

  const invitations = page.getByRole("list", { name: "Invitations", exact: true });
  await expect(invitations.getByText(newEmail, { exact: true })).toBeVisible();
  expect(await removeMemberWith(page, who(membership.display_name, newEmail), membership.id)).toBe(204);
  await expect.poll(() => membersListed(page)).toEqual([[`${displayNameOf(adminEmail)}You`, expect.any(String)]]);
  // The invitation pending to the address leaves the page with the member.
  await expect(page.getByText("No invitations pending.", { exact: true })).toBeVisible();
  await expectMembership(db, workspace.id, memberId, "ended");
  await expectInvitation(db, pending.id, "deleted", adminId);

  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  expect(await leaveWith(page, workspace.slug)).toBe(409);
  await expect(page.getByRole("alertdialog").getByRole("alert")).toHaveText(
    "You are the workspace's only admin. Make another member an admin first, or, if no one else is in it, delete the workspace."
  );
  await expect(page).toHaveURL(`/${workspace.slug}/settings/members`);
  await expectMembership(db, workspace.id, adminId, "admin");
});

test("W9 (page): a guest leaves and lands where / sends them; the workspace is no longer theirs", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const guestEmail = emailFor(testInfo, "guest");
  const tokens = await registerOnboarded(api, guestEmail);
  await accept(api, tokens.access_token, await invite(api, pat, workspace.slug, guestEmail, "guest"));
  const page = await signedInPage(tokens);
  await page.goto(`/${workspace.slug}/settings/members`);

  expect(await leaveWith(page, workspace.slug)).toBe(204);

  await expectCreatePage(page);
  await expectMembership(db, workspace.id, await accountIdOf(db, guestEmail), "ended");
  await page.goto(`/${workspace.slug}`);
  await expect(page.getByRole("heading", { level: 1, name: "Page not found" })).toBeVisible();
  await expect(workspaceHeading(page, "Acme")).toHaveCount(0);
});
