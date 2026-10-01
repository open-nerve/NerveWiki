import type { WorkspaceInvitation, WorkspaceRole } from "@nervewiki/api-client";
import { expect, type Page, type Response } from "@playwright/test";

import { answerTo } from "./browser";

// The invitations of the members page, and the invitation page, as a user
// works them (M2/P6 design 3.3, 3.4).

/**
 * Invites email as role on the members page of page, of the workspace of slug; resolves the status of the
 * answer, and the invitation it holds when it is 201.
 */
export async function inviteWith(
  page: Page,
  slug: string,
  email: string,
  role: WorkspaceRole = "member"
): Promise<{ status: number; invitation: WorkspaceInvitation | undefined }> {
  await page.getByLabel("E-mail address", { exact: true }).fill(email);
  await page.getByLabel("Role", { exact: true }).selectOption(role);
  const answer = answerTo(page, "POST", `/api/v0/workspaces/${slug}/invitations`);
  await page.getByRole("button", { name: "Invite", exact: true }).click();
  const response = await answer;
  const status = response.status();
  return { status, invitation: status === 201 ? ((await response.json()) as WorkspaceInvitation) : undefined };
}

/**
 * Copies the link of the invitation to email on the members page of page, and resolves what the clipboard
 * holds then: the page's context must be granted clipboard-read and clipboard-write.
 */
export async function copyLinkWith(page: Page, email: string): Promise<string> {
  await page.getByRole("button", { name: `Copy link: ${email}`, exact: true }).click();
  await expect(page.getByText(`Copied the link of the invitation to ${email}.`, { exact: true })).toBeVisible();
  return page.evaluate(() => navigator.clipboard.readText());
}

/** Withdraws the invitation id, to email, once confirmed, and resolves the answer's status. */
export async function withdrawWith(page: Page, email: string, id: string): Promise<number> {
  await page.getByRole("button", { name: `Withdraw the invitation to ${email}`, exact: true }).click();
  const answer = answerTo(page, "DELETE", `/api/v0/workspace-invitations/${id}`);
  await page.getByRole("alertdialog").getByRole("button", { name: "Withdraw", exact: true }).click();
  return (await answer).status();
}

/** The address of the page of invitation's link, at baseURL if given. */
export function linkTo(invitation: Pick<WorkspaceInvitation, "id" | "token">, baseURL = ""): string {
  return `${baseURL}/invitations/${invitation.id}#${invitation.token}`;
}

/** Accepts the invitation id on its page, signed in, and resolves the answer. */
export async function acceptWith(page: Page, id: string): Promise<Response> {
  const answer = answerTo(page, "POST", `/api/v0/workspace-invitations/${id}/accept`);
  await page.getByRole("button", { name: "Accept invitation", exact: true }).click();
  return answer;
}
