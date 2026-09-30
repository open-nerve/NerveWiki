import type { AuthTokens } from "@nervewiki/api-client";

import { expectNewAccount, expectNewSession } from "../../fixtures/assert/identity";
import { bearer, emailFor, password, recordOf, register } from "../../fixtures/auth";
import { signUpWith } from "../../fixtures/auth-pages";
import { expect, test } from "../../fixtures/test";

// A1, a new account (M1 design 3).

test("A1 (API): a caller signs up and gets a session", async ({ api, db }, testInfo) => {
  const email = ` ${emailFor(testInfo, "Alice")} `;
  const userAgent = "nervewiki-e2e/A1";

  const tokens = await register(api, email, { "User-Agent": userAgent });

  expect(tokens.token_type).toBe("Bearer");
  expect(tokens.access_token_expires_in).toBe(15 * 60);
  const userId = await expectNewAccount(db, email);
  await expectNewSession(db, userId, {
    email,
    accessToken: tokens.access_token,
    refreshToken: tokens.refresh_token,
    userAgent,
    ip: "127.0.0.1",
  });

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

test("A1 (page): a visitor signs up, lands on onboarding, and the browser keeps only the session's record", async ({
  page,
  db,
}, testInfo) => {
  const email = emailFor(testInfo, "Alice");
  const console: string[] = [];
  const addresses: string[] = [];
  page.on("console", (message) => void console.push(message.text()));
  page.on("request", (request) => void addresses.push(request.url()));
  await page.goto("/sign-up");
  await page.evaluate(() => Object.assign(window, { nervewikiE2eDocument: "sign-up" }));

  const answer = await signUpWith(page, email, password);

  expect(answer.status()).toBe(201);
  const tokens = (await answer.json()) as AuthTokens;
  await expect(page).toHaveURL("/onboarding");
  await expect(page.getByRole("heading", { level: 1, name: "Your name" })).toBeVisible();
  // The app went there itself: the document is still the one that signed up.
  expect(await page.evaluate(() => (window as { nervewikiE2eDocument?: string }).nervewikiE2eDocument)).toBe("sign-up");
  await expectNewAccount(db, email);

  // The session lives in the record alone: its refresh token and a login_id of the tab's making. The
  // access token stays in the page's memory; neither token is anywhere else.
  const record = JSON.parse((await recordOf(page)) ?? "null") as Record<string, string>;
  expect(Object.keys(record).toSorted()).toEqual(["login_id", "refresh_token"]);
  expect(record.refresh_token).toBe(tokens.refresh_token);
  expect(record.login_id).toMatch(/^[0-9a-f]{32}$/);
  expect(await page.context().cookies()).toEqual([]);
  const stored = await page.evaluate(() => [...Object.entries(localStorage), ...Object.entries(sessionStorage)]);
  for (const token of [tokens.access_token, tokens.refresh_token]) {
    const holders = stored.filter(([, value]) => value.includes(token)).map(([key]) => key);
    expect(holders).toEqual(token === tokens.refresh_token ? ["nwiki.auth"] : []);
    expect(page.url()).not.toContain(token);
    expect(addresses.filter((address) => address.includes(token))).toEqual([]);
    expect(console.filter((text) => text.includes(token))).toEqual([]);
  }
});
