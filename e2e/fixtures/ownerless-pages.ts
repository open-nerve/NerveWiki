import { expect, type Locator, type Page, type Response } from "@playwright/test";

import { displayNameOf } from "./auth";
import { answerTo } from "./browser";

// The ownerless notebooks page as a workspace admin works it (M3/P5 design
// 3.3): the list, taking over, deleting, and the audit log.

/** The address of the ownerless notebooks page of the workspace of slug. */
export function ownerlessPath(slug: string): string {
  return `/${slug}/settings/ownerless`;
}

/** The list of the ownerless notebooks on page. */
function ownerlessList(page: Page): Locator {
  return page.getByRole("list", { name: "Ownerless notebooks", exact: true });
}

/** The ownerless notebooks listed on page: each one's name, its former owner, and the line of its details. */
export function ownerlessListed(page: Page): Promise<string[][]> {
  return ownerlessList(page)
    .getByRole("listitem")
    .evaluateAll((items) => items.map((item) => [...item.querySelectorAll("p")].map((p) => p.textContent ?? "")));
}

/**
 * What ownerlessListed reads of the notebook named name whose former owner is email: its access as the page
 * words it, the members it has left, and dates and a size of any value.
 */
export function listedOwnerless(name: string, email: string, access: string, members: number): unknown[] {
  return [
    name,
    `Former owner: ${displayNameOf(email)} (${email})`,
    expect.stringMatching(
      new RegExp(`^${access} · Members left: ${members} · Ownerless since .+ · Last activity .+ · Size .+$`)
    ),
  ];
}

/** Takes over the ownerless notebook id named name, and resolves the answer. */
export async function takeOverWith(page: Page, id: string, name: string): Promise<Response> {
  const answer = answerTo(page, "POST", `/api/v0/ownerless-notebooks/${id}/take-over`);
  await page.getByRole("button", { name: `Take over ${name}`, exact: true }).click();
  return answer;
}

/** Deletes the ownerless notebook id named name once its name is typed, and resolves the answer's status. */
export async function deleteOwnerlessWith(page: Page, id: string, name: string): Promise<number> {
  await page.getByRole("button", { name: `Delete ${name}`, exact: true }).click();
  const dialog = page.getByRole("alertdialog");
  await dialog.getByLabel(`Type ${name} to confirm`, { exact: true }).fill(name);
  const answer = answerTo(page, "DELETE", `/api/v0/ownerless-notebooks/${id}`);
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  return (await answer).status();
}

/** What the audit log on page says, an event a line, the newest first. */
export function auditLog(page: Page): Promise<string[]> {
  return page
    .getByRole("list", { name: "Audit log", exact: true })
    .getByRole("listitem")
    .evaluateAll((items) => items.map((item) => item.querySelector("p")?.textContent ?? ""));
}

/** Loads the audit log's next page, of the workspace of slug, and resolves the answer: the read with a cursor. */
export async function loadMoreWith(page: Page, slug: string): Promise<Response> {
  const answer = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      response.request().method() === "GET" &&
      url.pathname === `/api/v0/workspaces/${slug}/notebook-audit-events` &&
      url.searchParams.has("cursor")
    );
  });
  await page.getByRole("button", { name: "Load more", exact: true }).click();
  return answer;
}
