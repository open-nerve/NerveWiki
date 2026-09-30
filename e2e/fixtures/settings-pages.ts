import type { Page, Response } from "@playwright/test";

import { answerTo } from "./browser";

// The settings pages as a user works them (M1/P6 design 3.8).

/** Fills the security page's password form of page, sends it, and resolves the status of its answer. */
export async function changePasswordWith(page: Page, current: string, next: string): Promise<number> {
  await page.getByLabel("Current password", { exact: true }).fill(current);
  await page.getByLabel("New password", { exact: true }).fill(next);
  const answer = answerTo(page, "POST", "/api/v0/me/change-password");
  await page.getByRole("button", { name: "Change password", exact: true }).click();
  return (await answer).status();
}

/** Opens the creating dialog of the tokens page, fills it and sends it; resolves the answer. */
export async function createTokenWith(
  page: Page,
  { name, password, expiry }: { name: string; password: string; expiry?: string }
): Promise<Response> {
  await page.getByRole("button", { name: "Create token" }).click();
  const dialog = page.getByRole("dialog", { name: "Create an access token" });
  await dialog.getByLabel("Name", { exact: true }).fill(name);
  if (expiry !== undefined) {
    await dialog.getByLabel("Expires", { exact: true }).selectOption({ label: expiry });
  }
  await dialog.getByLabel("Current password", { exact: true }).fill(password);
  const answer = answerTo(page, "POST", "/api/v0/me/api-tokens");
  await dialog.getByRole("button", { name: "Create", exact: true }).click();
  return answer;
}

/**
 * Holds the answers to page's requests of method to path: the server handles
 * each request at once, the page gets every answer only when the function
 * returned is called.
 */
export async function holdAnswer(page: Page, method: string, path: string): Promise<() => void> {
  let release: (() => void) | undefined;
  const released = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route(`**${path}`, async (route) => {
    if (route.request().method() !== method) {
      await route.fallback();
      return;
    }
    const response = await route.fetch();
    await released;
    await route.fulfill({ response });
  });
  return () => release?.();
}
