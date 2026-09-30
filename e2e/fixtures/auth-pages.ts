import type { Locator, Page, Response } from "@playwright/test";

import { answerTo } from "./browser";

// The sign-in and sign-up pages as a user works them (M1/P5 design 3.6).

/** Fills the sign-in form of page with email and password, sends it, and resolves the API's answer. */
export async function signInWith(page: Page, email: string, password: string): Promise<Response> {
  return sendCredentials(page, "Sign in", "/api/v0/auth/login", email, password);
}

/** Fills the sign-up form of page with email and password, sends it, and resolves the API's answer. */
export async function signUpWith(page: Page, email: string, password: string): Promise<Response> {
  return sendCredentials(page, "Sign up", "/api/v0/auth/register", email, password);
}

async function sendCredentials(
  page: Page,
  button: string,
  path: string,
  email: string,
  password: string
): Promise<Response> {
  await emailField(page).fill(email);
  await passwordField(page).fill(password);
  const answer = answerTo(page, "POST", path);
  await page.getByRole("button", { name: button, exact: true }).click();
  return answer;
}

export function emailField(page: Page): Locator {
  return page.getByLabel("E-mail address", { exact: true });
}

/** The password field; exact, as its Show password button is labelled too. */
export function passwordField(page: Page): Locator {
  return page.getByLabel("Password", { exact: true });
}

/** The error above the form. */
export function formError(page: Page): Locator {
  return page.getByRole("alert");
}

/** The user menu of page, which shows the signed-in account's display name. */
export function accountMenu(page: Page, displayName: string): Locator {
  return page.getByRole("button", { name: displayName, exact: true });
}

/** Signs page out through its user menu, the account of displayName being signed in. */
export async function signOutThroughMenu(page: Page, displayName: string): Promise<void> {
  await accountMenu(page, displayName).click();
  await page.getByRole("menuitem", { name: "Sign out" }).click();
}
