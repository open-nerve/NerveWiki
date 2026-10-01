import { createClient } from "@nervewiki/api-client";

import { accountIdOf, countIdentity, expectNothingAdded, expectOnboardingSteps } from "../../fixtures/assert/identity";
import { expectInvitation, expectMembership } from "../../fixtures/assert/workspace";
import { createToken, emailFor, password, register } from "../../fixtures/auth";
import { formError, signUpWith } from "../../fixtures/auth-pages";
import { failedToLoad } from "../../fixtures/browser";
import { acceptWith, linkTo } from "../../fixtures/invitation-pages";
import { accept, invite } from "../../fixtures/invitations";
import { saveProfileStep, stepRecorded } from "../../fixtures/onboarding-pages";
import { expect, test } from "../../fixtures/test";
import { workspaceHeading } from "../../fixtures/workspace-pages";
import { createWorkspace, slugFor } from "../../fixtures/workspaces";

// W7, signing up by an invitation while sign-up is closed (M2/P3 design 3.6).

test("W7 (API): with sign-up closed, only the invited address registers, by its link, then accepts", async ({
  api,
  db,
  nervewikiWith,
}, testInfo) => {
  // The admin registers on the worker's nervewiki, whose sign-up is open; the
  // closed one shares its database, and a personal access token works on
  // both. The links are the closed one's: each nervewiki of the stories
  // signs with a key of its own.
  const closed = createClient({
    baseUrl: (await nervewikiWith(db.url, { env: { NWIKI_AUTH__SIGNUP_ENABLED: "false" } })).baseURL,
  });
  const adminEmail = emailFor(testInfo, "admin");
  const pat = (await createToken(api, (await register(api, adminEmail)).access_token, { name: "W7" })).token;
  const adminId = await accountIdOf(db, adminEmail);
  const workspace = await createWorkspace(closed, pat, "Acme", slugFor(testInfo));
  const email = emailFor(testInfo, "invitee");
  const invitation = await invite(closed, pat, workspace.slug, email, "member");
  const link = { id: invitation.id, token: invitation.token };

  const before = await countIdentity(db);
  const refused = await Promise.all([
    closed.POST("/api/v0/auth/register", { body: { email, password } }),
    closed.POST("/api/v0/auth/register", { body: { email: emailFor(testInfo, "other"), password, invitation: link } }),
  ]);
  expect(refused.map((r) => [r.response.status, r.error?.code])).toEqual([
    [403, "identity.signup_disabled"],
    [403, "identity.signup_disabled"],
  ]);
  await expectNothingAdded(db, before);

  // The invited address registers, in any case; registering does not accept.
  const registered = await closed.POST("/api/v0/auth/register", {
    body: { email: email.toUpperCase(), password, invitation: link },
  });
  expect(registered.response.status, JSON.stringify(registered.error)).toBe(201);
  await expectInvitation(db, invitation.id, "pending", adminId);
  const session = registered.data?.access_token ?? "";
  expect((await accept(closed, session, invitation)).role).toBe("member");
  await expectMembership(db, workspace.id, await accountIdOf(db, email), "member");
});

test("W7 (page): with sign-up closed, the invited address signs up on the link's page, accepts, and goes in through onboarding; another address is refused there", async ({
  api,
  db,
  nervewikiWith,
  page,
  pageWatch,
}, testInfo) => {
  const { baseURL } = await nervewikiWith(db.url, { env: { NWIKI_AUTH__SIGNUP_ENABLED: "false" } });
  const closed = createClient({ baseUrl: baseURL });
  const pat = (await createToken(api, (await register(api, emailFor(testInfo, "admin"))).access_token, { name: "W7" }))
    .token;
  const workspace = await createWorkspace(closed, pat, "Acme", slugFor(testInfo));
  const email = emailFor(testInfo, "invitee");
  const invitation = await invite(closed, pat, workspace.slug, email, "member");
  await page.goto(linkTo(invitation, baseURL));
  await expect(page.getByText("You are invited to join Acme.", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Sign up", exact: true }).click();

  const before = await countIdentity(db);
  pageWatch.expectConsole({ errors: [failedToLoad(403)] });
  expect((await signUpWith(page, emailFor(testInfo, "other"), password)).status()).toBe(403);
  await expect(formError(page)).toHaveText(
    "This server takes new accounts only for the addresses invited: sign up with the one the invitation was sent to."
  );
  await expectNothingAdded(db, before);

  expect((await signUpWith(page, email, password)).status()).toBe(201);
  const userId = await accountIdOf(db, email);
  // Signing up does not accept.
  await expect(page.getByRole("button", { name: "Accept invitation", exact: true })).toBeVisible();
  await expectInvitation(db, invitation.id, "pending", await accountIdOf(db, emailFor(testInfo, "admin")));
  expect((await acceptWith(page, invitation.id)).status()).toBe(200);

  // The new account onboards first; its workspace step goes on by itself.
  const recorded = stepRecorded(page, "workspace");
  expect(await saveProfileStep(page)).toBe(200);
  expect((await recorded).status()).toBe(200);
  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await expect(page).toHaveURL(`${baseURL}/${workspace.slug}`);
  await expectMembership(db, workspace.id, userId, "member");
  await expectOnboardingSteps(db, userId, ["profile", "workspace"]);
});
