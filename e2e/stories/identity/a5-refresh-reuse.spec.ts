import { expectRevoked, forgedFrom, sessionOf } from "../../fixtures/assert/identity";
import { bearer, emailFor, refresh, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";

// A5, reuse detection (M1 design 3).

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
