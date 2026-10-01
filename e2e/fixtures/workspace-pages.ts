import type { Workspace } from "@nervewiki/api-client";
import { expect, type Locator, type Page } from "@playwright/test";

import { answerTo } from "./browser";

// The workspace pages as a user works them (M2/P5 design 3.10).

/** The heading of a workspace's home page: its name. */
export function workspaceHeading(page: Page, name: string): Locator {
  return page.getByRole("heading", { level: 1, name, exact: true });
}

/** Expects page on the creation page, where / lands an account without a workspace (at baseURL, if given). */
export async function expectCreatePage(page: Page, baseURL = ""): Promise<void> {
  await expect(page).toHaveURL(`${baseURL}/create-workspace`);
  await expect(page.getByRole("heading", { level: 1, name: "Create a workspace" })).toBeVisible();
}

export function nameField(page: Page): Locator {
  return page.getByLabel("Name", { exact: true });
}

export function slugField(page: Page): Locator {
  return page.getByLabel("Address", { exact: true });
}

/**
 * Fills the creation form of page (the creation page's, or onboarding's, whose button is "Create and
 * continue"), sends it, and resolves the status and the body of the creation's answer.
 */
export async function createWorkspaceWith(
  page: Page,
  { name, slug, button = "Create workspace" }: { name: string; slug?: string; button?: string }
): Promise<{ status: number; created: Workspace }> {
  await nameField(page).fill(name);
  if (slug !== undefined) {
    await slugField(page).fill(slug);
  }
  const answer = answerTo(page, "POST", "/api/v0/workspaces");
  await page.getByRole("button", { name: button, exact: true }).click();
  const response = await answer;
  return { status: response.status(), created: (await response.json()) as Workspace };
}

/** The switcher of page, which shows the workspace shown, named shown. */
export function switcher(page: Page, shown: string): Locator {
  return page.getByRole("complementary", { name: "Workspace" }).getByRole("button", { name: shown, exact: true });
}

/** Opens the switcher of page, which shows the workspace shown, and goes to the workspace named name. */
export async function switchWorkspace(page: Page, shown: string, name: string): Promise<void> {
  await switcher(page, shown).click();
  await page.getByRole("menuitemradio", { name, exact: true }).click();
}

/** The names of the workspaces the switcher of page lists, and the other choices it offers; it closes again. */
export async function switcherChoices(page: Page, shown: string): Promise<{ workspaces: string[]; others: string[] }> {
  await switcher(page, shown).click();
  await expect(page.getByRole("menuitemradio").first()).toBeVisible();
  const choices = {
    workspaces: await page.getByRole("menuitemradio").allTextContents(),
    others: await page.getByRole("menuitem").allTextContents(),
  };
  await page.keyboard.press("Escape");
  return choices;
}

/** Fills the name on the general settings page of page, saves it, and resolves the status and body of the answer. */
export async function renameWorkspaceWith(
  page: Page,
  slug: string,
  name: string
): Promise<{ status: number; renamed: Workspace }> {
  await nameField(page).fill(name);
  const answer = answerTo(page, "PATCH", `/api/v0/workspaces/${slug}`);
  await page.getByRole("button", { name: "Save", exact: true }).click();
  const response = await answer;
  return { status: response.status(), renamed: (await response.json()) as Workspace };
}

/**
 * Opens the deletion of the general settings page of page, checks that it cannot be confirmed before slug is
 * typed, types it, confirms, and resolves the status of the answer.
 */
export async function deleteWorkspaceWith(page: Page, slug: string): Promise<number> {
  await page.getByRole("button", { name: "Delete workspace", exact: true }).click();
  const dialog = page.getByRole("alertdialog");
  const confirm = dialog.getByRole("button", { name: "Delete", exact: true });
  await expect(confirm).toBeDisabled();
  await dialog.getByLabel(`Type ${slug} to confirm`, { exact: true }).fill(slug);
  const answer = answerTo(page, "DELETE", `/api/v0/workspaces/${slug}`);
  await confirm.click();
  return (await answer).status();
}
