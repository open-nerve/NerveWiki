import { createClient } from "@nervewiki/api-client";

import { accountIdOf, countIdentity, expectNewSession, expectNothingAdded } from "../../fixtures/assert/identity";
import { bearer, emailFor, login, password, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";

// A3, sign-in (M1 design 3).

test("A3 (API): a caller signs in; a wrong password and an unknown address answer alike", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo, "Alice");
  await register(api, email);
  const userAgent = "nervewiki-e2e/A3";

  // The address in another case and with blanks around it still signs in.
  const tokens = await login(api, ` ${email.toUpperCase()} `, { "User-Agent": userAgent });

  expect(tokens.token_type).toBe("Bearer");
  expect(tokens.access_token_expires_in).toBe(15 * 60);
  await expectNewSession(db, await accountIdOf(db, email), {
    email,
    accessToken: tokens.access_token,
    refreshToken: tokens.refresh_token,
    userAgent,
    ip: "127.0.0.1",
  });
  const me = await api.GET("/api/v0/me", { headers: bearer(tokens.access_token) });
  expect(me.response.status).toBe(200);

  const before = await countIdentity(db);
  const refusals = await Promise.all([
    api.POST("/api/v0/auth/login", { body: { email, password: "Wr0ng-password" } }),
    api.POST("/api/v0/auth/login", { body: { email: emailFor(testInfo, "nobody"), password } }),
  ]);
  for (const { response, error } of refusals) {
    expect(response.status).toBe(401);
    expect(error?.code).toBe("identity.invalid_credentials");
    expect(error?.detail).toBe("The e-mail address or the password is incorrect.");
  }

  // A body without a password breaks the contract: the platform's 400.
  const missing = await api.POST("/api/v0/auth/login", {
    // @ts-expect-error -- the request leaves out a required field on purpose
    body: { email },
  });
  expect(missing.response.status).toBe(400);
  expect(missing.error?.code).toBe("bad_request");
  expect(missing.error?.errors?.map((e) => ({ field: e.field, code: e.code }))).toEqual([
    { field: "password", code: "required" },
  ]);
  await expectNothingAdded(db, before);
});

test("A3 (API): behind a trusted proxy, the session records the client the proxy forwarded", async ({
  db,
  nervewikiWith,
}, testInfo) => {
  const proxied = createClient({
    baseUrl: (await nervewikiWith(db.url, { env: { NWIKI_SERVER__TRUSTED_PROXIES: "127.0.0.0/8" } })).baseURL,
  });
  const email = emailFor(testInfo);
  await register(proxied, email);
  const userAgent = "nervewiki-e2e/A3-proxy";

  const tokens = await login(proxied, email, { "User-Agent": userAgent, "X-Forwarded-For": "198.51.100.23" });

  await expectNewSession(db, await accountIdOf(db, email), {
    email,
    accessToken: tokens.access_token,
    refreshToken: tokens.refresh_token,
    userAgent,
    ip: "198.51.100.23",
  });
});
