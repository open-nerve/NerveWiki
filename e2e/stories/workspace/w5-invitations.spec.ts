import { accountIdOf } from "../../fixtures/assert/identity";
import { expectInvitation, expectPendingInvitation } from "../../fixtures/assert/workspace";
import { bearer, emailFor, registerOnboarded } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { copyLinkWith, inviteWith, linkTo, withdrawWith } from "../../fixtures/invitation-pages";
import { accept, invite, joinAs, listInvitations, preview, withdraw } from "../../fixtures/invitations";
import { membersList } from "../../fixtures/member-pages";
import { expect, test } from "../../fixtures/test";
import { createWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

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

test("W5 (page): the admin invites an address and copies its link; an address refused says why; a withdrawn link no longer works", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const adminEmail = emailFor(testInfo, "admin");
  const tokens = await registerOnboarded(api, adminEmail);
  const adminId = await accountIdOf(db, adminEmail);
  const { slug } = await createWorkspace(api, tokens.access_token, "Acme", slugFor(testInfo));
  const page = await signedInPage(tokens);
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto(`/${slug}/settings/members`);

  const email = emailFor(testInfo, "invitee");
  const { status, invitation } = await inviteWith(page, slug, email.toUpperCase(), "guest");
  expect([status, invitation?.email, invitation?.role]).toEqual([201, email, "guest"]);
  if (invitation === undefined) {
    throw new Error("the invitation was not answered");
  }
  await expectPendingInvitation(db, invitation, adminId);
  await expect(page.getByText(`Invited ${email}. Copy the link and send it to them.`, { exact: true })).toBeVisible();
  // The link is the list's, with the token in its fragment.
  expect(await copyLinkWith(page, email)).toBe(linkTo(invitation, new URL(page.url()).origin));

  pageWatch.expectConsole({ errors: [failedToLoad(422), failedToLoad(422)] });
  expect((await inviteWith(page, slug, email)).status).toBe(422);
  await expect(
    page.getByText("Already invited: copy the link of that invitation below.", { exact: true })
  ).toBeVisible();
  expect((await inviteWith(page, slug, adminEmail)).status).toBe(422);
  await expect(page.getByText("Already a member of this workspace.", { exact: true })).toBeVisible();

  expect(await withdrawWith(page, email, invitation.id)).toBe(204);
  await expect(page.getByText("No invitations pending.", { exact: true })).toBeVisible();
  await expectInvitation(db, invitation.id, "deleted", adminId);
  const link = await preview(api, invitation);
  expect([link.response.status, link.error?.code]).toEqual([404, "workspace.invitation_not_found"]);
});

test("W5 (page): a member sees no invitations, which the page does not ask for", async ({
  api,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  await invite(api, pat, workspace.slug, emailFor(testInfo, "pending"), "member");
  const email = emailFor(testInfo, "member");
  const tokens = await registerOnboarded(api, email);
  await accept(api, tokens.access_token, await invite(api, pat, workspace.slug, email, "member"));
  const page = await signedInPage(tokens);

  await page.goto(`/${workspace.slug}/settings/members`);

  await expect(membersList(page).getByRole("listitem")).toHaveCount(2);
  await expect(page.getByRole("heading", { name: "Invitations" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Invite", exact: true })).toHaveCount(0);
  expect(pageWatch.apiRequests.filter((request) => request.endsWith("/invitations"))).toEqual([]);
});
