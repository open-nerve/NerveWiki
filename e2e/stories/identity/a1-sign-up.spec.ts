import { expectNewAccount, expectNewSession } from "../../fixtures/assert/identity";
import { bearer, emailFor, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";

// A1, a new account (M1 design 3).

test("A1 (API): a caller signs up and gets a session", async ({ api, db }, testInfo) => {
  const email = ` ${emailFor(testInfo, "Alice")} `;
  const userAgent = "nervewiki-e2e/A1";

  const tokens = await register(api, email, { "User-Agent": userAgent });

  expect(tokens.token_type).toBe("Bearer");
  expect(tokens.access_token_expires_in).toBe(15 * 60);
  const userId = await expectNewAccount(db, email);
  await expectNewSession(db, userId, { email, refreshToken: tokens.refresh_token, userAgent, ip: "127.0.0.1" });

  // The access token works at once.
  const me = await api.GET("/api/v0/me", { headers: bearer(tokens.access_token) });
  expect(me.response.status).toBe(200);
  expect(me.data).toEqual({
    id: userId,
    email: email.trim().toLowerCase(),
    display_name: `alice-${testInfo.testId}-${testInfo.repeatEachIndex}-${testInfo.retry}`,
    onboarding_steps: [],
  });
});
