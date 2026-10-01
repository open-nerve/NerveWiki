import { accountIdOf } from "../../fixtures/assert/identity";
import { expectMembership } from "../../fixtures/assert/workspace";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { listMembers, memberOf, updateMember } from "../../fixtures/members";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

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
