import type { Page } from "@playwright/test";

import {
  accountIdOf,
  expectDisplayName,
  expectOnboardingSteps,
  expectRevoked,
  sessionOf,
} from "../../fixtures/assert/identity";
import {
  bearer,
  displayNameOf,
  emailFor,
  login,
  newRecord,
  password,
  recordOf,
  refresh,
  register,
  registerOnboarded,
  writeRecord,
} from "../../fixtures/auth";
import { accountMenu, signInWith, signOutThroughMenu } from "../../fixtures/auth-pages";
import { expectQuietPage, followAccessToken, watchPage, type PageWatch } from "../../fixtures/browser";
import { displayNameField, profileStep, saveProfileStep, workspaceStep } from "../../fixtures/onboarding-pages";
import { expect, test } from "../../fixtures/test";

// A6, sign-out (M1 design 3).

/** A page the tabs show while signed in: the app's 404, which the sign-in page comes back to. */
const where = "/acme";
const signInPage = `/sign-in?next=${encodeURIComponent(where)}`;

/** Opens where in tab, signed in as the account of email. */
async function openSignedIn(tab: Page, email: string): Promise<void> {
  await tab.goto(where);
  await expect(accountMenu(tab, displayNameOf(email))).toBeVisible();
  await expect(tab.getByRole("heading", { level: 1, name: "Page not found" })).toBeVisible();
}

test("A6 (page): signing out in one tab signs every tab out", async ({ api, db, context, signedInPage }, testInfo) => {
  const email = emailFor(testInfo);
  const tabA = await signedInPage(await registerOnboarded(api, email));
  const sentAccessToken = followAccessToken(tabA);
  await openSignedIn(tabA, email);
  const tabB = await context.newPage();
  const watchB = await watchPage(tabB);
  await openSignedIn(tabB, email);
  const held = (await recordOf(tabA))?.refresh_token ?? "";
  // The access token tab A sent last works until the sign-out.
  const accessToken = sentAccessToken();
  expect(accessToken, "tab A sent an access token").not.toBe("");
  expect((await api.GET("/api/v0/me", { headers: bearer(accessToken) })).response.status).toBe(200);

  await signOutThroughMenu(tabA, displayNameOf(email));

  // Both tabs are on the sign-in page, which comes back to where they were; the record is gone.
  await Promise.all(
    [tabA, tabB].map(async (tab) => {
      await expect(tab.getByRole("heading", { level: 1, name: "Sign in" })).toBeVisible();
      await expect(tab).toHaveURL(signInPage);
    })
  );
  expect(await recordOf(tabB)).toBeNull();
  await expectRevoked(db, held, "logout");
  // The session's access token fails on the next request.
  expect((await api.GET("/api/v0/me", { headers: bearer(accessToken) })).response.status).toBe(401);
  await expectQuiet(tabB, watchB);
});

test("A6 (page): another tab signs another account in without signing out: every tab goes on as that account", async ({
  api,
  db,
  context,
  signedInPage,
}, testInfo) => {
  const x = emailFor(testInfo, "x");
  const y = emailFor(testInfo, "y");
  const tabA = await signedInPage(await register(api, x));
  await register(api, y);
  await tabA.goto("/onboarding");
  await expect(accountMenu(tabA, displayNameOf(x))).toBeVisible();
  await expect(profileStep(tabA)).toBeVisible();
  const xHeld = (await recordOf(tabA))?.refresh_token ?? "";

  // Tab B, any page of the site, keeps a sign-in of Y as a tab does: a new record, written under the
  // refresh lock, with a new login_id.
  const tabB = await context.newPage();
  await tabB.goto("/favicon.svg");
  await writeRecord(tabB, newRecord(await login(api, y)));

  // Tab A follows: it shows Y, and what it writes from now on is Y's.
  await expect(accountMenu(tabA, displayNameOf(y))).toBeVisible();
  await expect(accountMenu(tabA, displayNameOf(x))).toHaveCount(0);
  await expect(displayNameField(tabA)).toHaveValue(displayNameOf(y));
  await displayNameField(tabA).fill("Yvonne");
  expect(await saveProfileStep(tabA)).toBe(200);
  await expect(workspaceStep(tabA)).toBeVisible();
  // Both of the step's writes went to Y, the name and then the step; X's account is as it was.
  const [xId, yId] = [await accountIdOf(db, x), await accountIdOf(db, y)];
  await expectDisplayName(db, yId, "Yvonne");
  await expectOnboardingSteps(db, yId, ["profile"]);
  await expectDisplayName(db, xId, displayNameOf(x));
  await expectOnboardingSteps(db, xId, []);
  // X's session is left as it was: nobody signed it out.
  expect((await sessionOf(db, xHeld)).revoked_at).toBeNull();
});

test("A6 (page): another tab signs out, then signs another account in: every tab comes in as that account", async ({
  api,
  db,
  context,
  signedInPage,
}, testInfo) => {
  const x = emailFor(testInfo, "x");
  const y = emailFor(testInfo, "y");
  const tabA = await signedInPage(await registerOnboarded(api, x));
  await registerOnboarded(api, y);
  await openSignedIn(tabA, x);
  const tabB = await context.newPage();
  const watchB = await watchPage(tabB);
  await openSignedIn(tabB, x);
  const xHeld = (await recordOf(tabB))?.refresh_token ?? "";

  await signOutThroughMenu(tabB, displayNameOf(x));
  await expect(tabA).toHaveURL(signInPage);
  await expect(tabB).toHaveURL(signInPage);
  await expectRevoked(db, xHeld, "logout");

  // Tab B signs Y in; tab A sees the record appear and comes in as Y too, back to where it was.
  expect((await signInWith(tabB, y, password)).status()).toBe(200);
  await Promise.all(
    [tabB, tabA].map(async (tab) => {
      await expect(accountMenu(tab, displayNameOf(y))).toBeVisible();
      await expect(tab).toHaveURL(where);
    })
  );
  await expectQuiet(tabB, watchB);
});

/** Checks that nothing went wrong in a second tab: no failed API request, and a quiet page. */
async function expectQuiet(tab: Page, watch: PageWatch): Promise<void> {
  expect(watch.apiFailures).toEqual([]);
  await expectQuietPage(tab, watch);
}

test("A6 (API): logout ends the session; the previous generation's logout changes nothing", async ({
  api,
  db,
}, testInfo) => {
  const first = await register(api, emailFor(testInfo));
  const second = await refresh(api, first.refresh_token);
  const logout = async (token: string) => {
    const { response } = await api.POST("/api/v0/auth/logout", { body: { refresh_token: token } });
    expect(response.status).toBe(204);
  };

  // The previous generation: 204, and the session goes on (M1/P2 design 3.6).
  const before = await sessionOf(db, second.refresh_token);
  await logout(first.refresh_token);
  expect(await sessionOf(db, second.refresh_token)).toEqual(before);

  // The current one ends the session; its access token fails on the next request.
  await logout(second.refresh_token);
  await expectRevoked(db, second.refresh_token, "logout");
  const me = await api.GET("/api/v0/me", { headers: bearer(second.access_token) });
  expect(me.response.status).toBe(401);
  const { response } = await api.POST("/api/v0/auth/refresh", { body: { refresh_token: second.refresh_token } });
  expect(response.status).toBe(401);

  // Once more: the same answer, nothing changes.
  const ended = await sessionOf(db, second.refresh_token);
  await logout(second.refresh_token);
  expect(await sessionOf(db, second.refresh_token)).toEqual(ended);
});
