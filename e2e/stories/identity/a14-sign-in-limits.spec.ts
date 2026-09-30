import { createClient } from "@nervewiki/api-client";

import { countIdentity, expectNothingAdded } from "../../fixtures/assert/identity";
import { bearer, createToken, emailFor, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";

// A14, sign-in limits (M1 design 3, M1/P2 design 3.2, 3.3).

/** A bucket that regains a unit a minute: 60 seconds, or 59 once a second has passed since its first unit went. */
function expectAMinute(retryAfter: string | null): void {
  expect(["59", "60"], `Retry-After ${retryAfter}`).toContain(retryAfter);
}

test("A14 (API): sign-ins are limited per client IP and address, then per client IP", async ({
  db,
  nervewikiWith,
}, testInfo) => {
  // login_ip 3, login_ip_email 2, each regaining a unit a minute.
  const limited = createClient({
    baseUrl: (
      await nervewikiWith(db.url, {
        env: {
          NWIKI_RATELIMIT__LOGIN_IP__PER_MINUTE: "1",
          NWIKI_RATELIMIT__LOGIN_IP__BURST: "3",
          NWIKI_RATELIMIT__LOGIN_IP_EMAIL__PER_MINUTE: "1",
          NWIKI_RATELIMIT__LOGIN_IP_EMAIL__BURST: "2",
        },
      })
    ).baseURL,
  });
  const email = emailFor(testInfo);
  await register(limited, email);
  const before = await countIdentity(db);
  const attempt = async (address: string, want: number) => {
    const { response, error } = await limited.POST("/api/v0/auth/login", {
      body: { email: address, password: "Wr0ng-password" },
    });
    expect(response.status, address).toBe(want);
    if (want === 429) {
      expect(error?.code).toBe("rate_limited");
      expectAMinute(response.headers.get("Retry-After"));
    }
  };

  // One address fails up to login_ip_email's burst, then is refused.
  await attempt(email, 401);
  await attempt(email, 401);
  await attempt(email, 429);
  // That refusal took nothing from login_ip: another address gets its last
  // unit, and then login_ip refuses every address.
  await attempt(emailFor(testInfo, "other"), 401);
  await attempt(emailFor(testInfo, "third"), 429);

  await expectNothingAdded(db, before);
});

test("A14 (API): tokens that fail empty the gate before authentication; a valid one costs nothing", async ({
  db,
  nervewikiWith,
}, testInfo) => {
  // auth_failure 2, regaining a unit a minute.
  const gated = createClient({
    baseUrl: (
      await nervewikiWith(db.url, {
        env: { NWIKI_RATELIMIT__AUTH_FAILURE__PER_MINUTE: "1", NWIKI_RATELIMIT__AUTH_FAILURE__BURST: "2" },
      })
    ).baseURL,
  });
  const tokens = await register(gated, emailFor(testInfo));
  const me = (token: string) => gated.GET("/api/v0/me", { headers: bearer(token) });

  // A valid token gives its unit back once authenticated: one after another, more of them than
  // the burst pass. (At once, no more than the burst would be authenticating: the unit is taken
  // before the token is looked at.)
  expect((await me(tokens.access_token)).response.status).toBe(200);
  expect((await me(tokens.access_token)).response.status).toBe(200);
  expect((await me(tokens.access_token)).response.status).toBe(200);
  // Two failed credentials empty it: a revoked personal access token fails like any other.
  const revoked = await createToken(gated, tokens.access_token, { name: "A14" });
  const revoke = await gated.DELETE("/api/v0/api-tokens/{token_id}", {
    params: { path: { token_id: revoked.id } },
    headers: bearer(tokens.access_token),
  });
  expect(revoke.response.status).toBe(204);
  expect((await me(revoked.token)).response.status).toBe(401);
  expect((await me("not-a-token")).response.status).toBe(401);

  // Then every token from this client is turned away before it is looked at, a valid one too.
  for (const { response, error } of await Promise.all([me("not-a-token"), me(tokens.access_token)])) {
    expect(response.status).toBe(429);
    expect(error?.code).toBe("rate_limited");
    expectAMinute(response.headers.get("Retry-After"));
  }
});

test("A14 (API): an expired access token is the cue to refresh, not a failure: the gate keeps its units", async ({
  db,
  nervewikiWith,
}, testInfo) => {
  // Access tokens live a second; auth_failure 2, regaining a unit a minute.
  const gated = createClient({
    baseUrl: (
      await nervewikiWith(db.url, {
        env: {
          NWIKI_AUTH__ACCESS_TOKEN_TTL: "1s",
          NWIKI_RATELIMIT__AUTH_FAILURE__PER_MINUTE: "1",
          NWIKI_RATELIMIT__AUTH_FAILURE__BURST: "2",
        },
      })
    ).baseURL,
  });
  const tokens = await register(gated, emailFor(testInfo));
  const me = (token: string) => gated.GET("/api/v0/me", { headers: bearer(token) });

  // Until it expires the token answers 200, which costs nothing; then 401.
  await expect.poll(async () => (await me(tokens.access_token)).response.status, { timeout: 10_000 }).toBe(401);

  // The expiry took no unit: two failed credentials still get their 401, the third is turned away.
  expect((await me("not-a-token")).response.status).toBe(401);
  expect((await me("still-not-a-token")).response.status).toBe(401);
  expect((await me("not-a-token")).response.status).toBe(429);
});
