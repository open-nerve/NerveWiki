import { expectRevoked, forgedFrom, sessionOf } from "../../fixtures/assert/identity";
import { bearer, emailFor, recordOf, refresh, register } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { profileStep } from "../../fixtures/onboarding-pages";
import { expect, test } from "../../fixtures/test";

// A5, reuse detection (M1 design 3).

test("A5 (page): once a copy of the page's refresh token was used, the page's next refresh ends the session", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const page = await signedInPage(await register(api, emailFor(testInfo)));
  await page.goto("/onboarding");
  await expect(profileStep(page)).toBeVisible();

  // Someone with a copy of the page's refresh token uses it first: nervewiki rotates the session to them.
  const held = (await recordOf(page))?.refresh_token ?? "";
  await refresh(api, held);
  // The page's next refresh, as it loads again, presents the retired token: nervewiki ends the session,
  // and the page goes to the sign-in page, which comes back here; the record is gone.
  await page.reload();
  await expect(page.getByRole("heading", { level: 1, name: "Sign in" })).toBeVisible();
  await expect(page).toHaveURL("/sign-in?next=%2Fonboarding");
  expect(await recordOf(page)).toBeNull();
  await expectRevoked(db, held, "reuse_detected");
  // The one request that failed is that refresh, which Chromium reports on the console as well.
  expect(pageWatch.apiFailures).toEqual(["401 POST /api/v0/auth/refresh"]);
  pageWatch.expectConsole({ errors: [failedToLoad(401)] });
});

test("A5 (API): a retired refresh token revokes its session; a forged older generation does not", async ({
  api,
  db,
}, testInfo) => {
  const first = await register(api, emailFor(testInfo));
  const second = await refresh(api, first.refresh_token);
  const refused = async (token: string) => {
    const { response, error } = await api.POST("/api/v0/auth/refresh", { body: { refresh_token: token } });
    expect(response.status).toBe(401);
    expect(error?.code).toBe("identity.refresh_token_invalid");
  };

  // A forged older generation proves nothing: 401, the session goes on.
  const before = await sessionOf(db, second.refresh_token);
  await refused(forgedFrom(first.refresh_token));
  expect(await sessionOf(db, second.refresh_token)).toEqual(before);

  // The real retired token comes back: someone else holds a copy.
  await refused(first.refresh_token);
  await expectRevoked(db, first.refresh_token, "reuse_detected");

  // Every token of the session fails from now on.
  await refused(second.refresh_token);
  const me = await api.GET("/api/v0/me", { headers: bearer(second.access_token) });
  expect(me.response.status).toBe(401);
});
