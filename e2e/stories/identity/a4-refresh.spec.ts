import { expectRefreshed } from "../../fixtures/assert/identity";
import { bearer, emailFor, login, refresh, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";

// A4, refresh (M1 design 3).

test("A4 (API): each refresh uses the last token; the generation counts up and the session end stays", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  await register(api, email);
  const first = await login(api, email);

  const second = await refresh(api, first.refresh_token);
  const third = await refresh(api, second.refresh_token);
  const fourth = await refresh(api, third.refresh_token);

  // The access token holds only sub, sid and exp in whole seconds: within
  // one second it can come out the same, so only the refresh token must
  // differ.
  for (const [before, after] of [
    [first, second],
    [second, third],
    [third, fourth],
  ] as const) {
    expect(after.refresh_token).not.toBe(before.refresh_token);
    expect(after.refresh_token_expires_at).toBe(first.refresh_token_expires_at);
  }
  await expectRefreshed(db, fourth.refresh_token, 3, first.refresh_token_expires_at);
  const me = await api.GET("/api/v0/me", { headers: bearer(fourth.access_token) });
  expect(me.response.status).toBe(200);
});
