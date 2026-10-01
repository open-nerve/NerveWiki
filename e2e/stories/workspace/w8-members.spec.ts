import type { ApiClient, AuthTokens, WorkspaceRole } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectMembership } from "../../fixtures/assert/workspace";
import { displayNameOf, emailFor, registerOnboarded } from "../../fixtures/auth";
import { accept, invite, joinAs } from "../../fixtures/invitations";
import { changeRoleWith, membersListed, roleOf } from "../../fixtures/member-pages";
import { listMembers, memberOf, updateMember } from "../../fixtures/members";
import { expect, test } from "../../fixtures/test";
import { createWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// W8, the members (M2/P2 design 3.2, 3.6), who join by invitation.

test("W8 (API): the members list hides the addresses from a guest; the admin changes a role, not their own", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, pat, workspace } = await newTeam(api, testInfo);
  const slug = workspace.slug;
  const memberEmail = emailFor(testInfo, "member");
  const guestEmail = emailFor(testInfo, "guest");
  const member = await joinAs(api, pat, slug, memberEmail, "member");
  const guest = await joinAs(api, pat, slug, guestEmail, "guest");

  // By when they joined; the display names are the addresses' local parts.
  const byAdmin = await listMembers(api, pat, slug);
  expect(byAdmin.map((m) => [m.email, m.role])).toEqual([
    [adminEmail, "admin"],
    [memberEmail, "member"],
    [guestEmail, "guest"],
  ]);
  expect((await listMembers(api, guest, slug)).map((m) => [m.display_name, m.email, m.role])).toEqual(
    byAdmin.map((m) => [m.display_name, null, m.role])
  );
  expect(await listMembers(api, member, slug)).toEqual(byAdmin);

  const guestMembership = await memberOf(api, pat, slug, guestEmail);
  const promoted = await updateMember(api, pat, guestMembership.id, "member");
  expect([promoted.response.status, promoted.data]).toEqual([200, { ...guestMembership, role: "member" }]);
  await expectMembership(db, workspace.id, await accountIdOf(db, guestEmail), "member", guestMembership.created_at);

  const own = await updateMember(api, pat, (await memberOf(api, pat, slug, adminEmail)).id, "member");
  const byMember = await updateMember(api, member, guestMembership.id, "guest");
  expect([own, byMember].map((a) => [a.response.status, a.error?.code])).toEqual([
    [409, "workspace.own_membership"],
    [403, "forbidden"],
  ]);
});

/** Registers email, onboarded, which joins the workspace of slug as role by an invitation of adminCredential. */
async function joinOnboarded(
  api: ApiClient,
  adminCredential: string,
  slug: string,
  email: string,
  role: WorkspaceRole
): Promise<AuthTokens> {
  const tokens = await registerOnboarded(api, email);
  await accept(api, tokens.access_token, await invite(api, adminCredential, slug, email, role));
  return tokens;
}

test("W8 (page): the admin sees the members, changes a guest's role, and has no control of their own", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const adminEmail = emailFor(testInfo, "admin");
  const tokens = await registerOnboarded(api, adminEmail);
  const workspace = await createWorkspace(api, tokens.access_token, "Acme", slugFor(testInfo));
  const memberEmail = emailFor(testInfo, "member");
  const guestEmail = emailFor(testInfo, "guest");
  await joinOnboarded(api, tokens.access_token, workspace.slug, memberEmail, "member");
  await joinOnboarded(api, tokens.access_token, workspace.slug, guestEmail, "guest");
  const page = await signedInPage(tokens);

  await page.goto(`/${workspace.slug}/settings/members`);

  // By when they joined, each with the address; the admin's own row marked, with no control.
  await expect.poll(() => membersListed(page)).toHaveLength(3);
  expect((await membersListed(page)).map(([name, line]) => [name, line?.split(" · ")[0]])).toEqual([
    [`${displayNameOf(adminEmail)}You`, adminEmail],
    [displayNameOf(memberEmail), memberEmail],
    [displayNameOf(guestEmail), guestEmail],
  ]);
  await expect(roleOf(page, displayNameOf(adminEmail))).toHaveCount(0);
  await expect(roleOf(page, displayNameOf(guestEmail))).toHaveValue("guest");

  const guestMembership = await memberOf(api, tokens.access_token, workspace.slug, guestEmail);
  expect((await changeRoleWith(page, displayNameOf(guestEmail), "member")).status()).toBe(200);
  await expect(roleOf(page, displayNameOf(guestEmail))).toHaveValue("member");
  await expectMembership(db, workspace.id, await accountIdOf(db, guestEmail), "member", guestMembership.created_at);
});

test("W8 (page): a guest sees the members and their roles, without their addresses", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { adminEmail, pat, workspace } = await newTeam(api, testInfo);
  const guestEmail = emailFor(testInfo, "guest");
  const page = await signedInPage(await joinOnboarded(api, pat, workspace.slug, guestEmail, "guest"));

  await page.goto(`/${workspace.slug}/settings/members`);

  await expect.poll(() => membersListed(page)).toHaveLength(2);
  expect((await membersListed(page)).map(([name, line]) => [name, line?.includes("@")])).toEqual([
    [displayNameOf(adminEmail), false],
    [`${displayNameOf(guestEmail)}You`, false],
  ]);
  await expect(page.getByRole("combobox")).toHaveCount(0);
  await expect(page.getByText("Admin", { exact: true })).toBeVisible();
});
