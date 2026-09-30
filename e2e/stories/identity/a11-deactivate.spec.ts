import { accountIdOf, expectDeactivated } from "../../fixtures/assert/identity";
import { bearer, createToken, emailFor, login, password, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";

// A11, deactivation (M1 design 3); activating again is the administrator's command (P4).

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
