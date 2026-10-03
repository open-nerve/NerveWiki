import type { Page as WikiPage } from "@nervewiki/api-client";
import { expect, type Locator, type Page, type Response } from "@playwright/test";

import { answerTo } from "./browser";

// A notebook's pages as a user works them (M4/P5 design 3.5–3.7, 3.10;
// M4/P6 design 3.12): the page tree in the left column, its menus, dialogs
// and dragging, a page's shell, the quick switch, and the source editor.
// fixtures/pages.ts is the API's.

/** The address of the page id of the notebook notebookId in the workspace of slug. */
export function wikiPagePath(slug: string, notebookId: string, id: string): string {
  return `/${slug}/notebooks/${notebookId}/pages/${id}`;
}

/** The page tree of the notebook named notebook, in the left column. */
export function pageTree(page: Page, notebook: string): Locator {
  return page.getByRole("navigation", { name: `Pages of ${notebook}`, exact: true });
}

/** The titles the tree of the notebook named notebook shows, from the top down: the open pages' children too. */
export function treeTitles(page: Page, notebook: string): Promise<string[]> {
  return pageTree(page, notebook).getByRole("link").allTextContents();
}

/** The row of the page titled title in the tree of the notebook named notebook: its link, menu and drop target. */
function treeRow(page: Page, notebook: string, title: string): Locator {
  return pageTree(page, notebook).getByRole("link", { name: title, exact: true }).locator("..");
}

/** The heading of a page's shell, its title. */
export function pageHeading(page: Page, title: string): Locator {
  return page.getByRole("heading", { level: 1, name: title, exact: true });
}

/** The answer to a creation of a page in the notebook notebookId, and the page it created. */
async function created(answer: Promise<Response>): Promise<{ status: number; created: WikiPage }> {
  const response = await answer;
  return { status: response.status(), created: (await response.json()) as WikiPage };
}

/** Creates a page with the New page of the tree of notebook (its id and name); resolves the answer. */
export function newPageWith(page: Page, notebook: { id: string; name: string }) {
  const answer = answerTo(page, "POST", `/api/v0/notebooks/${notebook.id}/pages`);
  return pageTree(page, notebook.name)
    .getByRole("button", { name: "New page", exact: true })
    .click()
    .then(() => created(answer));
}

/** Chooses item in the menu of the page titled title in the tree of the notebook named notebook. */
export async function choosePageAction(page: Page, notebook: string, title: string, item: string): Promise<void> {
  await pageTree(page, notebook)
    .getByRole("button", { name: `Actions for ${title}`, exact: true })
    .click();
  await page.getByRole("menuitem", { name: item, exact: true }).click();
}

/** Creates a subpage of the page titled title with its menu in the tree of notebook; resolves the answer. */
export async function newSubpageWith(page: Page, notebook: { id: string; name: string }, title: string) {
  const answer = answerTo(page, "POST", `/api/v0/notebooks/${notebook.id}/pages`);
  await choosePageAction(page, notebook.name, title, "New subpage");
  return created(answer);
}

/** The rename dialog of the page titled title. */
export function renameDialog(page: Page, title: string): Locator {
  return page.getByRole("dialog", { name: `Rename ${title}`, exact: true });
}

/**
 * Renames the page id titled title to name in its dialog, which its menu opens in the tree of the notebook named
 * notebook; resolves the answer. The dialog stays open on a refusal.
 */
export async function renamePageWith(
  page: Page,
  notebook: string,
  id: string,
  title: string,
  name: string
): Promise<Response> {
  if (!(await renameDialog(page, title).isVisible())) {
    await choosePageAction(page, notebook, title, "Rename");
  }
  const dialog = renameDialog(page, title);
  await dialog.getByLabel("Title", { exact: true }).fill(name);
  const answer = answerTo(page, "PATCH", `/api/v0/nodes/${id}`);
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  return answer;
}

/** The move dialog of the page titled title. */
export function moveDialog(page: Page, title: string): Locator {
  return page.getByRole("dialog", { name: `Move ${title}`, exact: true });
}

/** The options of the select labelled label in dialog, as it words them. */
export function optionsOf(dialog: Locator, label: string): Promise<string[]> {
  return dialog.getByLabel(label, { exact: true }).locator("option").allTextContents();
}

/**
 * Moves the page id titled title in the move dialog, which its menu opens in the tree of the notebook named
 * notebook: under parent, as the dialog words it (the ancestors' titles and its own, joined by " / "), at
 * position ("First", "After …", "Last"). Resolves the answer; the dialog stays open on a refusal.
 */
export async function movePageWith(
  page: Page,
  notebook: string,
  id: string,
  title: string,
  parent: string,
  position: string
): Promise<Response> {
  if (!(await moveDialog(page, title).isVisible())) {
    await choosePageAction(page, notebook, title, "Move to…");
  }
  const dialog = moveDialog(page, title);
  await dialog.getByLabel("Parent page", { exact: true }).selectOption({ label: parent });
  await dialog.getByLabel("Position", { exact: true }).selectOption({ label: position });
  const answer = answerTo(page, "POST", `/api/v0/nodes/${id}/move`);
  await dialog.getByRole("button", { name: "Move", exact: true }).click();
  return answer;
}

/** Where a drop on a page's row goes: before the page, after it, or into it, as its last child. */
export type DropPlace = "before" | "after" | "into";

/**
 * Drags the page titled title onto the row of the page titled target in the tree of the notebook named
 * notebook, at place: the top quarter of the row is before it, the bottom quarter after it, the middle into it.
 */
export async function dragPage(
  page: Page,
  notebook: string,
  title: string,
  target: string,
  place: DropPlace
): Promise<void> {
  const row = treeRow(page, notebook, target);
  const box = await row.boundingBox();
  if (box === null) {
    throw new Error(`the row of ${target} is not shown`);
  }
  const y = { before: 2, after: box.height - 2, into: box.height / 2 }[place];
  await treeRow(page, notebook, title).dragTo(row, { targetPosition: { x: box.width / 2, y } });
}

/** The breadcrumbs of a page's shell, from the notebook down to the page: each step's text, without the separators. */
export function breadcrumbs(page: Page): Promise<string[]> {
  return page
    .getByRole("navigation", { name: "Breadcrumb", exact: true })
    .getByRole("listitem")
    .evaluateAll((items) =>
      items.map((item) =>
        [...item.querySelectorAll(":scope > :not([aria-hidden])")].map((step) => step.textContent ?? "").join("")
      )
    );
}

/** The subpages a page's shell lists, by title. */
export function subpages(page: Page): Promise<string[]> {
  return page
    .getByRole("main")
    .getByRole("list", { name: "Subpages", exact: true })
    .getByRole("link")
    .allTextContents();
}

/** The quick switch's dialog. */
function quickSwitch(page: Page): Locator {
  return page.getByRole("dialog", { name: "Go to a page", exact: true });
}

/**
 * Opens the quick switch with Ctrl+O (Cmd+O on macOS) and types query; resolves the pages it lists, each as its
 * title and its ancestors' ("A / B", "" at the root).
 */
export async function quickSwitchFor(page: Page, query: string): Promise<string[][]> {
  await page.keyboard.press("ControlOrMeta+o");
  const field = quickSwitch(page).getByRole("combobox", { name: "Page title", exact: true });
  await field.fill(query);
  return quickSwitch(page)
    .getByRole("option")
    .evaluateAll((options) =>
      options.map((option) => [...option.querySelectorAll("span")].map((span) => span.textContent ?? ""))
    );
}

/** The source editor's content, where the text is typed (M4/P6 design 3.5). */
export function editorContent(page: Page): Locator {
  return page.getByRole("textbox", { name: "Page content", exact: true });
}

/** Opens the editor of the page shown with Edit; resolves once its content can be typed into, the focus in it. */
export async function startEditing(page: Page): Promise<Locator> {
  await page.getByRole("main").getByRole("button", { name: "Edit", exact: true }).click();
  const content = editorContent(page);
  await expect(content).toBeFocused();
  return content;
}

/**
 * Presses Edit on a page someone else holds, or the account elsewhere (M5/P4 design 3.6): the reading view stays,
 * with the note that says who, which takes the focus; no editor opens.
 */
export async function editRefused(page: Page, holderText: string): Promise<void> {
  await page.getByRole("main").getByRole("button", { name: "Edit", exact: true }).click();
  await expect(page.getByText(holderText, { exact: true })).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.activeElement?.textContent ?? "")).toContain(holderText);
  await expect(editorContent(page)).toHaveCount(0);
  await expect(page.getByRole("main").getByRole("article")).toBeVisible();
}

/** The banner of an edit whose session is lost: why it saves no more (M5/P4 design 3.8). */
export function lostBanner(page: Page): Locator {
  return page.getByRole("main").getByRole("alert");
}

/** The edit's status: unsaved, saving, saved, or why not. */
export function editStatus(page: Page): Locator {
  return page.getByRole("main").getByRole("status");
}

/** Saves with Ctrl+S (Cmd+S on macOS); resolves once the edit says it is saved. */
export async function saveEdit(page: Page): Promise<void> {
  await page.keyboard.press("ControlOrMeta+s");
  await expect(editStatus(page)).toHaveText("Saved.");
}

/** A write of a page's content the editor sent, as answered: its status, and the problem's code if refused. */
export interface ContentWrite {
  status: number;
  code?: string;
}

/**
 * The editor's writes of the page id's content from now on: all() resolves those answered so far, in order; saved()
 * the page the last answered, which the server must have saved.
 */
export function contentWrites(page: Page, id: string) {
  const answers: Response[] = [];
  page.on("response", (response) => {
    if (response.request().method() === "PUT" && new URL(response.url()).pathname === `/api/v0/pages/${id}/content`) {
      answers.push(response);
    }
  });
  return {
    all: (): Promise<ContentWrite[]> =>
      Promise.all(
        answers.map(async (answer) =>
          answer.ok()
            ? { status: answer.status() }
            : { status: answer.status(), code: ((await answer.json()) as { code: string }).code }
        )
      ),
    saved: async (): Promise<WikiPage> => {
      const last = answers.at(-1);
      if (last?.status() !== 200) {
        throw new Error(`the last write of ${id} was not saved: ${last?.status()}`);
      }
      return (await last.json()) as WikiPage;
    },
  };
}

/** The conflict of a save refused because the page changed (M4/P6 design 3.8). */
export function conflictRegion(page: Page): Locator {
  return page.getByRole("region", { name: "This page changed while you edited it", exact: true });
}
