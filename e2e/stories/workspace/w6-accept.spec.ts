import { accountIdOf } from "../../fixtures/assert/identity";
import { expectInvitation, expectMembership } from "../../fixtures/assert/workspace";
import { bearer, displayNameOf, emailFor, password, register, registerOnboarded } from "../../fixtures/auth";
import { formError, signInWith } from "../../fixtures/auth-pages";
import { failedToLoad } from "../../fixtures/browser";
import { acceptWith, linkTo } from "../../fixtures/invitation-pages";
import { accept, invite, joinAs, preview, previewOf, tryAccept } from "../../fixtures/invitations";
import { leave, memberOf } from "../../fixtures/members";
import { expect, test } from "../../fixtures/test";
import { nervewikiUsers } from "../../fixtures/admin";
import { workspaceHeading } from "../../fixtures/workspace-pages";
import { createWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// W6, accepting (M2/P3 design 3.3): the link's token and the invitee's
// address let them in, as no role can.

/** The link with another token: a character of its middle changed. */
function tampered(token: string): string {
  const i = token.length - 10;
  return token.slice(0, i) + (token[i] === "A" ? "B" : "A") + token.slice(i + 1);
}

test("W6 (API): the invitee previews the link, signs up and accepts it; another address, a wrong token, a used link are refused", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const email = emailFor(testInfo, "invitee");
  const invitation = await invite(api, pat, workspace.slug, email, "member");

  // Anyone holding the link sees what it invites to, never the address.
  const shown = await preview(api, invitation);
  expect([shown.response.status, shown.data]).toEqual([200, previewOf("Acme", workspace.slug, "member")]);
  const forged = { ...invitation, token: tampered(invitation.token) };
  const wrong = await Promise.all([preview(api, forged), tryAccept(api, pat, forged)]);
  expect(wrong.map((a) => [a.response.status, a.error?.code])).toEqual([
    [404, "workspace.invitation_not_found"],
    [404, "workspace.invitation_not_found"],
  ]);

  // Someone signed in with another address is refused, and the link stays.
  const other = await register(api, emailFor(testInfo, "other"));
  const mismatch = await tryAccept(api, other.access_token, invitation);
  expect([mismatch.response.status, mismatch.error?.code]).toEqual([403, "workspace.invitation_email_mismatch"]);
  await expectInvitation(db, invitation.id, "pending", adminId);

  const session = await register(api, email);
  const joined = await accept(api, session.access_token, invitation);
  expect(joined).toEqual({ ...workspace, role: "member" });
  const userId = await accountIdOf(db, email);
  await expectInvitation(db, invitation.id, "accepted", userId);
  await expectMembership(db, workspace.id, userId, "member");
  const seen = await api.GET("/api/v0/workspaces", { headers: bearer(session.access_token) });
  expect(seen.data?.data).toEqual([{ ...workspace, role: "member" }]);

  // The link is used up.
  const again = await Promise.all([preview(api, invitation), tryAccept(api, session.access_token, invitation)]);
  expect(again.map((a) => [a.response.status, a.error?.code])).toEqual([
    [404, "workspace.invitation_not_found"],
    [404, "workspace.invitation_not_found"],
  ]);
});

test("W6 (API): a member's invitation keeps their role; after leaving, a new one brings them back as they first joined", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const slug = workspace.slug;
  const email = emailFor(testInfo, "guest");
  const guest = await joinAs(api, pat, slug, email, "guest");
  const userId = await accountIdOf(db, email);
  const joinedAt = (await memberOf(api, pat, slug, email)).created_at;

  // An invitation to an address a member takes afterwards: the server's
  // administrator gives it to the guest.
  const newEmail = emailFor(testInfo, "renamed");
  const toNewAddress = await invite(api, pat, slug, newEmail, "admin");
  await nervewikiUsers(db, ["set-email", "--email", email, "--new-email", newEmail]);
  expect((await accept(api, guest, toNewAddress)).role).toBe("guest");
  await expectInvitation(db, toNewAddress.id, "accepted", userId);
  await expectMembership(db, workspace.id, userId, "guest", joinedAt);

  expect((await leave(api, guest, slug)).response.status).toBe(204);
  await expectMembership(db, workspace.id, userId, "ended");
  const back = await invite(api, pat, slug, newEmail, "member");
  expect((await accept(api, guest, back)).role).toBe("member");
  await expectMembership(db, workspace.id, userId, "member", joinedAt);
});

test("W6 (page): the invitee opens the link signed out, sees what it invites to, signs in there, accepts and goes in; no address the page asks holds the token", async ({
  api,
  db,
  page,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const email = emailFor(testInfo, "invitee");
  // The invitee has a workspace before Acme by name: / would land there.
  await createWorkspace(api, (await registerOnboarded(api, email)).access_token, "Aardvark", slugFor(testInfo, "own"));
  const invitation = await invite(api, pat, workspace.slug, email, "member");
  const asked: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).pathname.startsWith("/api/")) {
      asked.push(request.url());
    }
  });

  await page.goto(linkTo(invitation));
  await expect(page.getByText("You are invited to join Acme.", { exact: true })).toBeVisible();
  await expect(page.getByText("Your role there: Member", { exact: true })).toBeVisible();
  expect((await signInWith(page, email, password)).status()).toBe(200);

  // The session changed, and the page stayed at its link.
  await expect(page.getByText(`Signed in as ${displayNameOf(email)} (${email}).`, { exact: true })).toBeVisible();
  await expect(page).toHaveURL(linkTo(invitation));
  expect((await acceptWith(page, invitation.id)).status()).toBe(200);
  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await expect(page).toHaveURL(`/${workspace.slug}`);

  const userId = await accountIdOf(db, email);
  await expectInvitation(db, invitation.id, "accepted", userId);
  await expectMembership(db, workspace.id, userId, "member");
  expect(asked.length).toBeGreaterThan(0);
  expect(asked.filter((address) => address.includes(invitation.token))).toEqual([]);
});

test("W6 (page): an account of another address is told so, signs out there, and the invitee signs in and accepts", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const email = emailFor(testInfo, "invitee");
  await registerOnboarded(api, email);
  const invitation = await invite(api, pat, workspace.slug, email, "guest");
  const page = await signedInPage(await registerOnboarded(api, emailFor(testInfo, "other")));
  await page.goto(linkTo(invitation));

  pageWatch.expectConsole({ errors: [failedToLoad(403)] });
  expect((await acceptWith(page, invitation.id)).status()).toBe(403);
  await expect(formError(page)).toHaveText(
    "This invitation was sent to another e-mail address. Sign in with that one to accept it."
  );
  await expectInvitation(db, invitation.id, "pending", adminId);
  await page.getByRole("button", { name: "Sign out", exact: true }).click();

  await expect(page.getByRole("button", { name: "Sign in", exact: true })).toBeVisible();
  await expect(page).toHaveURL(linkTo(invitation));
  expect((await signInWith(page, email, password)).status()).toBe(200);
  expect((await acceptWith(page, invitation.id)).status()).toBe(200);
  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await expectMembership(db, workspace.id, await accountIdOf(db, email), "guest");
});
