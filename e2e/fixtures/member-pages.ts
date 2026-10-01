import type { WorkspaceRole } from "@nervewiki/api-client";
import type { Locator, Page, Response } from "@playwright/test";

import { answerTo } from "./browser";

// The members page as a user works it (M2/P6 design 3.3).

/** The list of the members on the members page of page. */
export function membersList(page: Page): Locator {
  return page.getByRole("list", { name: "Members", exact: true });
}

/** The members listed on page: each one's name (with "You" after the account's own) and the line under it. */
export function membersListed(page: Page): Promise<string[][]> {
  return membersList(page)
    .getByRole("listitem")
    .evaluateAll((items) => items.map((item) => [...item.querySelectorAll("p")].map((p) => p.textContent ?? "")));
}

/** The select of the role of the member named name: an admin's, for another member. */
export function roleOf(page: Page, name: string): Locator {
  return page.getByRole("combobox", { name: `Role of ${name}`, exact: true });
}

/** Chooses role for the member named name, and resolves the answer of the change. */
export async function changeRoleWith(page: Page, name: string, role: WorkspaceRole): Promise<Response> {
  const answer = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      new URL(response.url()).pathname.startsWith("/api/v0/workspace-members/")
  );
  await roleOf(page, name).selectOption(role);
  return answer;
}

/** Removes the member named name, whose membership is id, once confirmed, and resolves the answer's status. */
export async function removeMemberWith(page: Page, name: string, id: string): Promise<number> {
  await page.getByRole("button", { name: `Remove ${name}`, exact: true }).click();
  const answer = answerTo(page, "DELETE", `/api/v0/workspace-members/${id}`);
  await page.getByRole("alertdialog").getByRole("button", { name: "Remove", exact: true }).click();
  return (await answer).status();
}

/** Leaves the workspace of slug once confirmed, and resolves the answer's status; the dialog stays on a refusal. */
export async function leaveWith(page: Page, slug: string): Promise<number> {
  await page.getByRole("button", { name: "Leave workspace", exact: true }).click();
  const answer = answerTo(page, "POST", `/api/v0/workspaces/${slug}/leave`);
  await page.getByRole("alertdialog").getByRole("button", { name: "Leave", exact: true }).click();
  return (await answer).status();
}
