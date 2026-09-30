import { accountIdOf, accountOf, expectDeactivated } from "../../fixtures/assert/identity";
import {
  bearer,
  createToken,
  emailFor,
  login,
  password,
  recordOf,
  register,
  registerOnboarded,
} from "../../fixtures/auth";
import { formError, signInWith } from "../../fixtures/auth-pages";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import { expect, test } from "../../fixtures/test";
import { nervewikiUsers } from "../../fixtures/users";

// A11, deactivation (M1 design 3); activating again is the administrator's command (M1/P4 design 3.6).

test("A11 (API): a token deactivates the account: every session ends, the tokens stop, sign-in answers 403", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  const first = await register(api, email);
  const second = await login(api, email);
  const userId = await accountIdOf(db, email);
  const token = await createToken(api, first.access_token, { name: "A11" });
  const other = await createToken(api, first.access_token, { name: "other" });

  const { response } = await api.POST("/api/v0/me/deactivate", { headers: bearer(token.token) });
  expect(response.status).toBe(204);
  await expectDeactivated(db, userId);

  const answers = await Promise.all(
    [token.token, other.token, first.access_token, second.access_token].map((credential) =>
      api.GET("/api/v0/me", { headers: bearer(credential) })
    )
  );
  expect(answers.map((a) => a.response.status)).toEqual([401, 401, 401, 401]);
  const refreshed = await api.POST("/api/v0/auth/refresh", { body: { refresh_token: second.refresh_token } });
  expect(refreshed.response.status).toBe(401);
  const signIn = await api.POST("/api/v0/auth/login", { body: { email, password } });
  expect(signIn.response.status).toBe(403);
  expect(signIn.error?.code).toBe("identity.account_deactivated");
});

test("A11 (command line): users activate brings the tokens back; the sessions stay ended; sign-in works", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  const session = await register(api, email);
  const userId = await accountIdOf(db, email);
  const token = await createToken(api, session.access_token, { name: "A11" });
  const other = await createToken(api, session.access_token, { name: "other" });
  const { response } = await api.POST("/api/v0/me/deactivate", { headers: bearer(token.token) });
  expect(response.status).toBe(204);

  expect(await nervewikiUsers(db, ["activate", "--email", email])).toBe(
    `activated ${email}: 2 API tokens are usable again\n`
  );
  expect((await accountOf(db, userId)).is_active).toBe(true);

  const answers = await Promise.all(
    [token.token, other.token, session.access_token].map((credential) =>
      api.GET("/api/v0/me", { headers: bearer(credential) })
    )
  );
  expect(answers.map((a) => a.response.status)).toEqual([200, 200, 401]);
  const refreshed = await api.POST("/api/v0/auth/refresh", { body: { refresh_token: session.refresh_token } });
  expect(refreshed.response.status).toBe(401);
  await login(api, email);
  expect(await nervewikiUsers(db, ["activate", "--email", email])).toBe(`${email} is already active\n`);
});

test("A11 (page): the settings deactivate the account: every session and token stops; activated, it signs in again", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const email = emailFor(testInfo);
  const own = await registerOnboarded(api, email);
  const other = await login(api, email);
  const token = await createToken(api, own.access_token, { name: "A11" });
  const userId = await accountIdOf(db, email);
  const status = async (credential: string) =>
    (await api.GET("/api/v0/me", { headers: bearer(credential) })).response.status;
  const page = await signedInPage(own);
  await page.goto("/settings/security");

  await page.getByRole("button", { name: "Deactivate account" }).click();
  const dialog = page.getByRole("alertdialog", { name: "Deactivate your account?" });
  await expect(dialog).toContainText("Only the server's administrator can activate the account again.");
  const deactivated = answerTo(page, "POST", "/api/v0/me/deactivate");
  await dialog.getByRole("button", { name: "Deactivate", exact: true }).click();
  expect((await deactivated).status()).toBe(204);

  // This browser forgets the session and goes to sign in; every session and token is over.
  await expect(page.getByRole("heading", { level: 1, name: "Sign in" })).toBeVisible();
  await expect(page).toHaveURL("/sign-in?next=%2Fsettings%2Fsecurity");
  expect(await recordOf(page)).toBeNull();
  await expectDeactivated(db, userId);
  expect([await status(other.access_token), await status(token.token)]).toEqual([401, 401]);
  expect((await signInWith(page, email, password)).status()).toBe(403);
  await expect(formError(page)).toHaveText(
    "This account is deactivated. Ask the server's administrator to activate it."
  );
  pageWatch.expectConsole({ errors: [failedToLoad(403)] });

  // The administrator activates it: it signs in again, back where it was, and the token works.
  expect(await nervewikiUsers(db, ["activate", "--email", email])).toBe(
    `activated ${email}: 1 API token is usable again\n`
  );
  expect((await signInWith(page, email, password)).status()).toBe(200);
  await expect(page).toHaveURL("/settings/security");
  expect(await status(token.token)).toBe(200);
});

test("A11 (page): a deactivation the server refuses keeps the dialog, its reason and the session", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const email = emailFor(testInfo);
  const page = await signedInPage(await registerOnboarded(api, email));
  const userId = await accountIdOf(db, email);
  await page.route("**/api/v0/me/deactivate", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/problem+json",
      json: { status: 503, code: "server_busy", title: "Service Unavailable" },
    })
  );
  await page.goto("/settings/security");

  await page.getByRole("button", { name: "Deactivate account" }).click();
  const dialog = page.getByRole("alertdialog", { name: "Deactivate your account?" });
  await dialog.getByRole("button", { name: "Deactivate", exact: true }).click();

  await expect(dialog.getByRole("alert")).toHaveText("The server is busy. Try again in a moment.");
  await expect(dialog.getByRole("button", { name: "Deactivate", exact: true })).toBeEnabled();
  await expect(page).toHaveURL("/settings/security");
  expect(await recordOf(page)).not.toBeNull();
  expect((await accountOf(db, userId)).is_active).toBe(true);
  pageWatch.expectConsole({ errors: [failedToLoad(503)] });
});
