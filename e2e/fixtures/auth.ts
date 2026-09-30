import type { ApiClient, AuthTokens } from "@nervewiki/api-client";
import { expect, type TestInfo } from "@playwright/test";

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
