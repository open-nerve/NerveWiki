import type { ApiClient } from "@nervewiki/api-client";

import { accountIdOf, accountOf, countIdentity, expectNewAccount } from "../../fixtures/assert/identity";
import { bearer, createToken, emailFor, login, password, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";
import { nervewikiUsers, nervewikiUsersFails } from "../../fixtures/users";

// A12, the server administrator's commands (M1 design 3, M1/P4 design 3.6–3.7): nervewiki users on the
// worker's database, which its nervewiki serves. Only the command line does these: there is no page or API
// version. nervewikiUsers checks every run's output and logs, at debug, for the password.

const newPassword = "N3w-Passw0rd!";

/** The status of GET /me with credential. */
async function meStatus(api: ApiClient, credential: string): Promise<number> {
  return (await api.GET("/api/v0/me", { headers: bearer(credential) })).response.status;
}

test("A12: users create makes an account that signs in, with no session of its own", async ({ api, db }, testInfo) => {
  const email = emailFor(testInfo);

  const created = await nervewikiUsers(db, ["create", "--email", email.toUpperCase()], password);

  const userId = await expectNewAccount(db, email);
  expect(created).toBe(`created ${email} (${userId})\n`);
  expect(await db.query("SELECT id FROM auth_sessions WHERE user_id = $1", [userId])).toHaveLength(0);
  await login(api, email);
});

test("A12: users reset-password revokes every session and token of the account, and of it alone", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  const first = await register(api, email);
  const second = await login(api, email);
  const token = await createToken(api, first.access_token, { name: "A12" });
  const other = await register(api, emailFor(testInfo, "other"));
  const otherToken = await createToken(api, other.access_token, { name: "other" });
  const before = await accountOf(db, await accountIdOf(db, email));

  expect(await nervewikiUsers(db, ["reset-password", "--email", email], newPassword)).toBe(
    `password reset for ${email}: revoked 2 sessions, 1 API token\n`
  );

  expect((await accountOf(db, await accountIdOf(db, email))).password).not.toBe(before.password);
  expect(
    await Promise.all([first.access_token, second.access_token, token.token].map((c) => meStatus(api, c)))
  ).toEqual([401, 401, 401]);
  const refreshed = await api.POST("/api/v0/auth/refresh", { body: { refresh_token: second.refresh_token } });
  expect(refreshed.response.status).toBe(401);
  expect((await api.POST("/api/v0/auth/login", { body: { email, password } })).response.status).toBe(401);
  expect((await api.POST("/api/v0/auth/login", { body: { email, password: newPassword } })).response.status).toBe(200);
  expect(await Promise.all([other.access_token, otherToken.token].map((c) => meStatus(api, c)))).toEqual([200, 200]);
});

test("A12: users set-email ends the sessions; the tokens keep working; the new address signs in", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  const session = await register(api, email);
  const token = await createToken(api, session.access_token, { name: "A12" });
  const newEmail = emailFor(testInfo, "renamed");

  expect(await nervewikiUsers(db, ["set-email", "--email", email, "--new-email", newEmail.toUpperCase()])).toBe(
    `e-mail changed to ${newEmail}: revoked 1 session\n`
  );

  expect(await meStatus(api, session.access_token)).toBe(401);
  const me = await api.GET("/api/v0/me", { headers: bearer(token.token) });
  expect(me.response.status).toBe(200);
  expect(me.data?.email).toBe(newEmail);
  expect((await api.POST("/api/v0/auth/login", { body: { email, password } })).response.status).toBe(401);
  await login(api, newEmail);
});

test("A12: a refused command exits 1 with one line and changes nothing", async ({ api, db }, testInfo) => {
  const email = emailFor(testInfo);
  const other = emailFor(testInfo, "other");
  await register(api, email);
  await register(api, other);
  const before = await countIdentity(db);
  const hash = (await accountOf(db, await accountIdOf(db, email))).password;

  await nervewikiUsersFails(db, ["activate", "--email", emailFor(testInfo, "nobody")], "The account does not exist.");
  await nervewikiUsersFails(
    db,
    ["set-email", "--email", email, "--new-email", email.toUpperCase()],
    "The account has this e-mail address already."
  );
  await nervewikiUsersFails(
    db,
    ["set-email", "--email", email, "--new-email", other],
    "An account with this e-mail address already exists."
  );
  await nervewikiUsersFails(
    db,
    ["reset-password", "--email", email],
    "the password must be at least 8 characters",
    "short"
  );

  expect(await countIdentity(db)).toEqual(before);
  expect((await accountOf(db, await accountIdOf(db, email))).password).toBe(hash);
  await login(api, email);
});
