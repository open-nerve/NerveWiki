import { expectInvitation, expectPendingInvitation } from "../../fixtures/assert/workspace";
import { bearer, emailFor } from "../../fixtures/auth";
import { invite, joinAs, listInvitations, preview, withdraw } from "../../fixtures/invitations";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// W5, inviting (M2/P3 design 3.3): what an admin does with invitations, and
// what a member and a guest cannot.

test("W5 (API): the admin invites, lists the links and withdraws one; a member and a guest cannot", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat, workspace } = await newTeam(api, testInfo);
  const slug = workspace.slug;
  const email = emailFor(testInfo, "invitee");

  const invitation = await invite(api, pat, slug, `  ${email.toUpperCase()} `, "guest");
  expect(invitation).toEqual({
    id: expect.any(String),
    email,
    role: "guest",
    token: expect.stringMatching(/^nwk_inv_[\w-]{22}$/),
    created_at: expect.any(String),
  });
  await expectPendingInvitation(db, invitation, adminId);
  // The token is not stored: the list makes it again, the same.
  expect(await listInvitations(api, pat, slug)).toEqual([invitation]);

  // An address with a pending invitation, and an active member's, are refused on the email field.
  const refused = await Promise.all(
    [email, adminEmail].map((address) =>
      api.POST("/api/v0/workspaces/{slug}/invitations", {
        params: { path: { slug } },
        body: { email: address, role: "member" },
        headers: bearer(pat),
      })
    )
  );
  expect(refused.map((r) => [r.response.status, r.error?.errors?.map((e) => [e.field, e.code])])).toEqual([
    [422, [["email", "duplicate"]]],
    [422, [["email", "not_allowed"]]],
  ]);

  const others = [
    await joinAs(api, pat, slug, emailFor(testInfo, "member"), "member"),
    await joinAs(api, pat, slug, emailFor(testInfo, "guest"), "guest"),
  ];
  const answers = await Promise.all(
    others.flatMap((credential) => [
      api.GET("/api/v0/workspaces/{slug}/invitations", { params: { path: { slug } }, headers: bearer(credential) }),
      api.POST("/api/v0/workspaces/{slug}/invitations", {
        params: { path: { slug } },
        body: { email: emailFor(testInfo, "another"), role: "member" },
        headers: bearer(credential),
      }),
      api.DELETE("/api/v0/workspace-invitations/{workspace_invitation_id}", {
        params: { path: { workspace_invitation_id: invitation.id } },
        headers: bearer(credential),
      }),
    ])
  );
  expect(answers.map((a) => [a.response.status, a.error?.code])).toEqual(answers.map(() => [403, "forbidden"]));
  // The accepted invitations are no longer pending.
  expect(await listInvitations(api, pat, slug)).toEqual([invitation]);

  await withdraw(api, pat, invitation.id);
  await expectInvitation(db, invitation.id, "deleted", adminId);
  expect(await listInvitations(api, pat, slug)).toEqual([]);
  const link = await preview(api, invitation);
  expect([link.response.status, link.error?.code]).toEqual([404, "workspace.invitation_not_found"]);
});
