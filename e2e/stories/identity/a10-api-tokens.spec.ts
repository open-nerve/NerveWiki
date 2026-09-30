import {
  accountIdOf,
  countIdentity,
  expectNewToken,
  expectNothingAdded,
  expectTokenRevoked,
  expectTokenUsed,
} from "../../fixtures/assert/identity";
import { bearer, createToken, emailFor, password, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";

// A10, personal access tokens (M1 design 3).

test("A10 (API): a token asks for the password, is shown once, is listed without itself and stops when revoked", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  const session = await register(api, email);
  const userId = await accountIdOf(db, email);
  const list = async (credential: string) => {
    const { response, data } = await api.GET("/api/v0/me/api-tokens", { headers: bearer(credential) });
    expect(response.status).toBe(200);
    return data?.data ?? [];
  };
  const status = async (credential: string) =>
    (await api.GET("/api/v0/me", { headers: bearer(credential) })).response.status;

  // A wrong password creates nothing.
  const before = await countIdentity(db);
  const wrong = await api.POST("/api/v0/me/api-tokens", {
    body: { name: "CI", current_password: "Wr0ng-password" },
    headers: bearer(session.access_token),
  });
  expect(wrong.response.status).toBe(422);
  expect(wrong.error?.code).toBe("identity.current_password_incorrect");
  await expectNothingAdded(db, before);

  const created = await createToken(api, session.access_token, { name: " CI deploy " });
  expect(created.name).toBe("CI deploy");
  expect(created.expires_at).toBeNull();
  await expectNewToken(db, userId, created);

  // The list has the token without the token itself, never used so far.
  expect(await list(session.access_token)).toEqual([
    { id: created.id, name: "CI deploy", expires_at: null, last_used_at: null, created_at: created.created_at },
  ]);

  // A token acts as the account, and creates tokens itself; the newest is listed first.
  expect(await status(created.token)).toBe(200);
  await expectTokenUsed(db, created.id);
  const second = await createToken(api, created.token, { name: "backup" });
  await expectNewToken(db, userId, second);
  const listed = await list(second.token);
  expect(listed.map((t) => t.id)).toEqual([second.id, created.id]);
  expect(listed[1]?.last_used_at).not.toBeNull();
  expect(JSON.stringify(listed)).not.toContain(created.token);

  // Another account cannot see or revoke it.
  const stranger = await register(api, emailFor(testInfo, "stranger"));
  expect(await list(stranger.access_token)).toEqual([]);
  const foreign = await api.DELETE("/api/v0/api-tokens/{token_id}", {
    params: { path: { token_id: created.id } },
    headers: bearer(stranger.access_token),
  });
  expect(foreign.response.status).toBe(404);
  expect(foreign.error?.code).toBe("identity.api_token_not_found");

  // Revoked, it stops at once; revoking again finds nothing.
  const revoke = () =>
    api.DELETE("/api/v0/api-tokens/{token_id}", {
      params: { path: { token_id: created.id } },
      headers: bearer(session.access_token),
    });
  expect((await revoke()).response.status).toBe(204);
  await expectTokenRevoked(db, created.id);
  expect(await status(created.token)).toBe(401);
  const again = await revoke();
  expect(again.response.status).toBe(404);
  expect(again.error?.code).toBe("identity.api_token_not_found");
  expect((await list(session.access_token)).map((t) => t.id)).toEqual([second.id]);
});

test("A10 (API): a token stops when it expires", async ({ api, db }, testInfo) => {
  const email = emailFor(testInfo);
  const session = await register(api, email);

  // The past is refused.
  const past = await api.POST("/api/v0/me/api-tokens", {
    body: { name: "late", expires_at: new Date(Date.now() - 60_000).toISOString(), current_password: password },
    headers: bearer(session.access_token),
  });
  expect(past.response.status).toBe(422);
  expect(past.error?.errors?.map((e) => ({ field: e.field, code: e.code }))).toEqual([
    { field: "expires_at", code: "out_of_range" },
  ]);

  const expiresAt = new Date(Date.now() + 5_000);
  const created = await createToken(api, session.access_token, { name: "short", expires_at: expiresAt.toISOString() });
  await expectNewToken(db, await accountIdOf(db, email), created);
  const status = async () => (await api.GET("/api/v0/me", { headers: bearer(created.token) })).response.status;

  expect(await status()).toBe(200);
  await expect.poll(status, { timeout: 15_000 }).toBe(401);
  expect(Date.now()).toBeGreaterThanOrEqual(expiresAt.getTime());
});
