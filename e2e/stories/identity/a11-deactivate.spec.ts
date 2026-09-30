import { accountIdOf, accountOf, expectDeactivated } from "../../fixtures/assert/identity";
import { bearer, createToken, emailFor, login, password, register } from "../../fixtures/auth";
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
