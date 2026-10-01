import { accountIdOf } from "../../fixtures/assert/identity";
import { expectInvitation, expectMembership } from "../../fixtures/assert/workspace";
import { bearer, emailFor } from "../../fixtures/auth";
import { invite, joinAs, tryAccept } from "../../fixtures/invitations";
import { leave, memberOf, removeMember } from "../../fixtures/members";
import { expect, test } from "../../fixtures/test";
import { nervewikiUsers } from "../../fixtures/admin";
import { newTeam } from "../../fixtures/workspaces";

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
