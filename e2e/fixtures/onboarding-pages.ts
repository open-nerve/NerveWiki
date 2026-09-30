import type { Locator, Page } from "@playwright/test";

import { answerTo } from "./browser";

// The onboarding page as a user works it (M1/P5 design 3.7).

/** The heading of the profile step, the first of onboarding. */
export function profileStep(page: Page): Locator {
  return page.getByRole("heading", { level: 1, name: "Your name" });
}

export function displayNameField(page: Page): Locator {
  return page.getByLabel("Display name", { exact: true });
}

/**
 * Presses Continue on the profile step of page and resolves the status of the step's record (the name,
 * when changed, goes out first), or "signed out" when the tab lands on the sign-in page instead: its
 * session ended, and the step will not be recorded.
 */
export async function saveProfileStep(page: Page): Promise<number | "signed out"> {
  const recorded = answerTo(page, "POST", "/api/v0/me/onboarding-steps").then((response) => response.status());
  const signedOut = page
    .getByRole("heading", { level: 1, name: "Sign in" })
    .waitFor()
    .then(() => "signed out" as const);
  // The outcome that does not come is left waiting until the page closes.
  for (const outcome of [recorded, signedOut]) {
    outcome.catch(() => {});
  }
  await page.getByRole("button", { name: "Continue" }).click();
  return Promise.race([recorded, signedOut]);
}
