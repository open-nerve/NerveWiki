import {
  accountIdOf,
  accountOf,
  countIdentity,
  expectNothingAdded,
  expectPasswordChanged,
} from "../../fixtures/assert/identity";
import {
  bearer,
  createToken,
  displayNameOf,
  emailFor,
  login,
  password,
  recordOf,
  register,
  registerOnboarded,
} from "../../fixtures/auth";
import { accountMenu } from "../../fixtures/auth-pages";
import { failedToLoad, noteOf } from "../../fixtures/browser";
import { changePasswordWith } from "../../fixtures/settings-pages";
import { expect, test } from "../../fixtures/test";

// A7, change the password (M1 design 3).

test("A7 (API): a new password ends the other sessions, keeps the caller's own and leaves the tokens working", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  const own = await register(api, email);
  const other = await login(api, email);
  const userId = await accountIdOf(db, email);
  const token = await createToken(api, own.access_token, { name: "A7" });
  const changePassword = (credential: string, current: string, next: string) =>
    api.POST("/api/v0/me/change-password", {
      body: { current_password: current, new_password: next },
      headers: bearer(credential),
    });
  const status = async (credential: string) =>
    (await api.GET("/api/v0/me", { headers: bearer(credential) })).response.status;

  // A wrong current password changes nothing.
  const before = await countIdentity(db);
  const hashBefore = (await accountOf(db, userId)).password;
  const wrong = await changePassword(own.access_token, "Wr0ng-password", "new horse battery");
  expect(wrong.response.status).toBe(422);
  expect(wrong.error?.code).toBe("identity.current_password_incorrect");
  expect((await accountOf(db, userId)).password).toBe(hashBefore);
  expect(await status(other.access_token)).toBe(200);
  await expectNothingAdded(db, before);

  // With the session's access token: the other session ends, this one goes on.
  const changed = await changePassword(own.access_token, password, "new horse battery");
  expect(changed.response.status).toBe(204);
  await expectPasswordChanged(db, userId, hashBefore, own.refresh_token);
  expect(await status(other.access_token)).toBe(401);
  const refreshed = await api.POST("/api/v0/auth/refresh", { body: { refresh_token: other.refresh_token } });
  expect(refreshed.response.status).toBe(401);
  expect(await status(own.access_token)).toBe(200);
  expect(await status(token.token)).toBe(200);

  // The old password no longer signs in; the new one does.
  const old = await api.POST("/api/v0/auth/login", { body: { email, password } });
  expect(old.response.status).toBe(401);
  const signedIn = await api.POST("/api/v0/auth/login", { body: { email, password: "new horse battery" } });
  expect(signedIn.response.status).toBe(200);

  // With the token, which belongs to no session: every session ends, the token goes on.
  const hashBetween = (await accountOf(db, userId)).password;
  const again = await changePassword(token.token, "new horse battery", "third horse battery");
  expect(again.response.status).toBe(204);
  await expectPasswordChanged(db, userId, hashBetween);
  expect(await status(own.access_token)).toBe(401);
  expect(await status(signedIn.data?.access_token ?? "")).toBe(401);
  expect(await status(token.token)).toBe(200);
});

test("A7 (page): the settings change the password: the other sessions end, this one and the tokens go on", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const email = emailFor(testInfo);
  const own = await registerOnboarded(api, email);
  const other = await login(api, email);
  const token = await createToken(api, own.access_token, { name: "deploy" });
  const userId = await accountIdOf(db, email);
  const status = async (credential: string) =>
    (await api.GET("/api/v0/me", { headers: bearer(credential) })).response.status;
  const page = await signedInPage(own);
  await page.goto("/settings/security");
  const current = page.getByLabel("Current password", { exact: true });
  const next = page.getByLabel("New password", { exact: true });

  // A wrong current password and a common new one show under their fields; nothing changes.
  const hashBefore = (await accountOf(db, userId)).password;
  expect(await changePasswordWith(page, "Wr0ng-password", "new horse battery")).toBe(422);
  await expect.poll(() => noteOf(current)).toBe("The current password is incorrect.");
  expect(await changePasswordWith(page, password, "password")).toBe(422);
  await expect.poll(() => noteOf(next)).toBe("Too common, or too close to the e-mail address.");
  expect((await accountOf(db, userId)).password).toBe(hashBefore);
  pageWatch.expectConsole({ errors: [failedToLoad(422), failedToLoad(422)] });

  expect(await changePasswordWith(page, password, "new horse battery")).toBe(204);
  await expect(page.getByRole("status")).toContainText("Password changed.");
  await expect(current).toHaveValue("");
  await expect(next).toHaveValue("");

  // The page's own session goes on; the other one is over; the token works.
  await expectPasswordChanged(db, userId, hashBefore, (await recordOf(page))?.refresh_token);
  expect(await status(other.access_token)).toBe(401);
  const refreshed = await api.POST("/api/v0/auth/refresh", { body: { refresh_token: other.refresh_token } });
  expect(refreshed.response.status).toBe(401);
  await page.reload();
  await expect(accountMenu(page, displayNameOf(email))).toBeVisible();
  await expect(page).toHaveURL("/settings/security");
  expect(await status(token.token)).toBe(200);
  const old = await api.POST("/api/v0/auth/login", { body: { email, password } });
  expect(old.response.status).toBe(401);
});
