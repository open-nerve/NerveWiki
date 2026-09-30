import type { Locator, Page, Response } from "@playwright/test";

// The settings pages as a user works them (M1/P6 design 3.8).

/** The next answer page gets to method path. */
export function answerTo(page: Page, method: string, path: string): Promise<Response> {
  return page.waitForResponse(
    (response) => response.request().method() === method && new URL(response.url()).pathname === path
  );
}

/** The note under field: its problem when it has one, else its hint; null without either. */
export function noteOf(field: Locator): Promise<string | null> {
  return field.evaluate((input) => {
    const id = input.getAttribute("aria-describedby");
    return id === null ? null : (document.getElementById(id)?.textContent ?? null);
  });
}

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
 * Holds the answer to the page's next method path: the server handles the
 * request at once, the page gets its answer only when the function returned
 * is called.
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
