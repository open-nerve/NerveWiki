import { expectRevoked, sessionOf } from "../../fixtures/assert/identity";
import { bearer, emailFor, refresh, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";

// A6, sign-out (M1 design 3).

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
