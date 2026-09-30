import { randomBytes } from "node:crypto";

import type { ApiClient, ApiTokenCreated, AuthTokens } from "@nervewiki/api-client";
import { expect, type BrowserContext, type Page, type TestInfo } from "@playwright/test";

/** A password that meets the rules: 8–128 characters, not a common one. */
export const password = "correct horse battery";

/**
 * An address of this run of this test: the tests of a worker share its
 * database, and --repeat-each runs a test again in the same worker.
 */
export function emailFor(testInfo: TestInfo, label = "user"): string {
  return `${label}-${testInfo.testId}-${testInfo.repeatEachIndex}-${testInfo.retry}@example.com`;
}

/** Signs email up through the API and returns the new session's tokens. */
export async function register(
  api: ApiClient,
  email: string,
  headers: Record<string, string> = {}
): Promise<AuthTokens> {
  const { data, error, response } = await api.POST("/api/v0/auth/register", {
    body: { email, password },
    headers,
  });
  expect(response.status, `register ${email}: ${JSON.stringify(error)}`).toBe(201);
  if (!data) {
    throw new Error(`register ${email} answered 201 without tokens`);
  }
  return data;
}

/** Signs email in through the API with the password of register and returns the new session's tokens. */
export async function login(api: ApiClient, email: string, headers: Record<string, string> = {}): Promise<AuthTokens> {
  const { data, error, response } = await api.POST("/api/v0/auth/login", {
    body: { email, password },
    headers,
  });
  expect(response.status, `login ${email}: ${JSON.stringify(error)}`).toBe(200);
  if (!data) {
    throw new Error(`login ${email} answered 200 without tokens`);
  }
  return data;
}

/** Exchanges refreshToken for the session's next tokens. */
export async function refresh(api: ApiClient, refreshToken: string): Promise<AuthTokens> {
  const { data, error, response } = await api.POST("/api/v0/auth/refresh", { body: { refresh_token: refreshToken } });
  expect(response.status, `refresh: ${JSON.stringify(error)}`).toBe(200);
  if (!data) {
    throw new Error("refresh answered 200 without tokens");
  }
  return data;
}

/** The Authorization header of a bearer token. */
export function bearer(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` };
}

/**
 * Creates a personal access token with credential (an access token or a
 * personal access token), confirming the password of register, and returns
 * the answer: the token itself, this once.
 */
export async function createToken(
  api: ApiClient,
  credential: string,
  body: { name: string; expires_at?: string }
): Promise<ApiTokenCreated> {
  const { data, error, response } = await api.POST("/api/v0/me/api-tokens", {
    body: { ...body, current_password: password },
    headers: bearer(credential),
  });
  expect(response.status, `create a token: ${JSON.stringify(error)}`).toBe(201);
  if (!data) {
    throw new Error("create a token answered 201 without the token");
  }
  return data;
}

/** The key of the session's record in the page's localStorage (M1/P5 design 3.2). */
const authKey = "nwiki.auth";

/** The session's record: the refresh token, and the login_id of the sign-in it came from. */
export interface AuthRecord {
  refresh_token: string;
  login_id: string;
}

/** The record a sign-in with tokens writes: a new login_id of 16 random bytes in hexadecimal. */
export function newRecord(tokens: AuthTokens): AuthRecord {
  return { refresh_token: tokens.refresh_token, login_id: randomBytes(16).toString("hex") };
}

/**
 * Signs the pages of context in with tokens at the nervewiki of baseURL (M1/P5 design 3.9): before the
 * first page there loads, its localStorage gets the record a sign-in writes (a new login_id), and the page
 * refreshes it itself; the access token never reaches the browser. Only that first load writes it: later
 * loads, reloads and other tabs find the record the pages keep, refreshed or removed, as in a browser.
 */
export async function signInContext(context: BrowserContext, baseURL: string, tokens: AuthTokens): Promise<void> {
  const record = newRecord(tokens);
  await context.addInitScript(
    ({ origin, key, text }) => {
      const seeded = `${key}.e2e-seeded`;
      if (window.location.origin !== origin || localStorage.getItem(seeded) !== null) {
        return;
      }
      localStorage.setItem(seeded, "1");
      localStorage.setItem(key, text);
    },
    { origin: new URL(baseURL).origin, key: authKey, text: JSON.stringify(record) }
  );
}

/** The session's record in the localStorage of page; null when there is none. */
export async function recordOf(page: Page): Promise<AuthRecord | null> {
  const text = await page.evaluate((key) => localStorage.getItem(key), authKey);
  return text === null ? null : (JSON.parse(text) as AuthRecord);
}

/**
 * Writes record into the localStorage of page as a tab that signs in does (M1/P5 design 3.2): in one
 * piece, holding the refresh lock, so that no refresh of another tab writes in between. The other tabs
 * get the storage event.
 */
export async function writeRecord(page: Page, record: AuthRecord): Promise<void> {
  await page.evaluate(
    async ({ key, text }) => {
      await navigator.locks.request("nwiki.auth.refresh", () => {
        localStorage.setItem(key, text);
      });
    },
    { key: authKey, text: JSON.stringify(record) }
  );
}

/** The display name an account gets at sign-up: what comes before the @ of its address, in lower case. */
export function displayNameOf(email: string): string {
  return email.trim().toLowerCase().slice(0, email.trim().indexOf("@"));
}

/** Records the step of onboarding for the account of accessToken, as the web app does once the step is done. */
export async function completeOnboarding(api: ApiClient, accessToken: string, step = "profile"): Promise<void> {
  const { response, error } = await api.POST("/api/v0/me/onboarding-steps", {
    body: { step },
    headers: bearer(accessToken),
  });
  expect(response.status, `record ${step}: ${JSON.stringify(error)}`).toBe(200);
}
