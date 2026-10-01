import type { Notebook, NotebookMember } from "@nervewiki/api-client";
import type { Locator, Page, Response } from "@playwright/test";

import { answerTo } from "./browser";
import { roleOf } from "./member-pages";

// The notebooks' pages as a user works them (M3/P4 design 3.3, 3.4). A
// notebook's members list, its rows and role menus read as a workspace's
// do: see member-pages.ts.

/** The navigation of the workspace named name on page: its own pages, then its notebooks. */
export function workspaceNav(page: Page, name: string): Locator {
  return page.getByRole("navigation", { name, exact: true });
}

/** The groups of notebooks the left column of the workspace named workspace shows: each one's names, by group. */
export async function notebookGroups(page: Page, workspace: string): Promise<Record<string, string[]>> {
  const nav = workspaceNav(page, workspace);
  const groups = await Promise.all(
    ["My notebooks", "Team notebooks"].map(async (name) => {
      const list = nav.getByRole("list", { name, exact: true });
      return (await list.count()) === 0 ? [] : [[name, await list.getByRole("link").allTextContents()] as const];
    })
  );
  return Object.fromEntries(groups.flat());
}

/** The heading of a notebook's home, its name. */
export function notebookHeading(page: Page, name: string): Locator {
  return page.getByRole("heading", { level: 1, name, exact: true });
}

/**
 * Creates a notebook named name from the left column of the workspace named workspace (slug), its access
 * as the dialog words it ("Workspace can read") or private; resolves the creation's answer.
 */
export async function createNotebookWith(
  page: Page,
  workspace: { name: string; slug: string },
  name: string,
  access?: string
): Promise<{ status: number; created: Notebook }> {
  await workspaceNav(page, workspace.name).getByRole("button", { name: "New notebook", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New notebook", exact: true });
  await dialog.getByLabel("Name", { exact: true }).fill(name);
  if (access !== undefined) {
    await dialog.getByRole("radio", { name: access, exact: true }).check();
  }
  const answer = answerTo(page, "POST", `/api/v0/workspaces/${workspace.slug}/notebooks`);
  await dialog.getByRole("button", { name: "Create", exact: true }).click();
  const response = await answer;
  return { status: response.status(), created: (await response.json()) as Notebook };
}

/** The address of a page of the notebook id in the workspace of slug: its home, or one of its settings. */
export function notebookPath(slug: string, id: string, settings?: "general" | "members"): string {
  return `/${slug}/notebooks/${id}${settings === undefined ? "" : `/settings/${settings}`}`;
}

/** The form of the name on a notebook's general page. */
function renameForm(page: Page): Locator {
  return page.locator("form", { has: page.getByLabel("Name", { exact: true }) });
}

/** Renames the notebook id on its general page, saved; resolves the status and body of the answer. */
export async function renameNotebookWith(
  page: Page,
  id: string,
  name: string
): Promise<{ status: number; renamed: Notebook }> {
  await renameForm(page).getByLabel("Name", { exact: true }).fill(name);
  const answer = answerTo(page, "PATCH", `/api/v0/notebooks/${id}`);
  await renameForm(page).getByRole("button", { name: "Save", exact: true }).click();
  const response = await answer;
  return { status: response.status(), renamed: (await response.json()) as Notebook };
}

/** Chooses the access of the notebook id on its general page, as the page words it, and saves it; resolves the answer. */
export async function changeAccessWith(page: Page, id: string, access: string): Promise<Response> {
  const form = page.locator("form", { has: page.getByRole("group", { name: "Workspace access", exact: true }) });
  await form.getByRole("radio", { name: access, exact: true }).check();
  const answer = answerTo(page, "PATCH", `/api/v0/notebooks/${id}`);
  await form.getByRole("button", { name: "Save", exact: true }).click();
  return answer;
}

/** Deletes the notebook id named name on its general page once its name is typed; resolves the answer's status. */
export async function deleteNotebookWith(page: Page, id: string, name: string): Promise<number> {
  await page.getByRole("button", { name: "Delete notebook", exact: true }).click();
  const dialog = page.getByRole("alertdialog");
  await dialog.getByLabel(`Type ${name} to confirm`, { exact: true }).fill(name);
  const answer = answerTo(page, "DELETE", `/api/v0/notebooks/${id}`);
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  return (await answer).status();
}

/**
 * Adds the member the select names member (see who) to the notebook id with role, as the page words it;
 * resolves the status and body of the answer.
 */
export async function addNotebookMemberWith(
  page: Page,
  id: string,
  member: string,
  role: string
): Promise<{ status: number; added: NotebookMember }> {
  await page.getByLabel("Member", { exact: true }).selectOption({ label: member });
  await page.getByLabel("Role", { exact: true }).selectOption({ label: role });
  const answer = answerTo(page, "POST", `/api/v0/notebooks/${id}/members`);
  await page.getByRole("button", { name: "Add", exact: true }).click();
  const response = await answer;
  return { status: response.status(), added: (await response.json()) as NotebookMember };
}

/** Chooses role, as the menu words it, for the notebook member the button names member (see who); resolves the answer. */
export async function changeNotebookRoleWith(page: Page, member: string, role: string): Promise<Response> {
  await roleOf(page, member).click();
  const answer = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      new URL(response.url()).pathname.startsWith("/api/v0/notebook-members/")
  );
  await page.getByRole("menuitemradio", { name: role, exact: true }).click();
  return answer;
}

/** Removes the notebook member who (see who), whose membership is id, once confirmed; resolves the answer's status. */
export async function removeNotebookMemberWith(page: Page, member: string, id: string): Promise<number> {
  await page.getByRole("button", { name: `Remove ${member}`, exact: true }).click();
  const answer = answerTo(page, "DELETE", `/api/v0/notebook-members/${id}`);
  await page.getByRole("alertdialog").getByRole("button", { name: "Remove", exact: true }).click();
  return (await answer).status();
}

/** Leaves the notebook id once confirmed, and resolves the answer's status; the dialog stays on a refusal. */
export async function leaveNotebookWith(page: Page, id: string): Promise<number> {
  await page.getByRole("button", { name: "Leave notebook", exact: true }).click();
  const answer = answerTo(page, "POST", `/api/v0/notebooks/${id}/leave`);
  await page.getByRole("alertdialog").getByRole("button", { name: "Leave", exact: true }).click();
  return (await answer).status();
}
