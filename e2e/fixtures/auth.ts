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

/** The Authorization header of a bearer token. */
export function bearer(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` };
}
