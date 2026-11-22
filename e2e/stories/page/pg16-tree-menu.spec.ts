import type { ApiClient, Page as WikiPage } from "@nervewiki/api-client";

import { emailFor } from "../../fixtures/auth";
import { joinOnboarded } from "../../fixtures/invitations";
import { phoneWidth } from "../../fixtures/narrow";
import { goto } from "../../fixtures/navigation";
import { notebookHeading, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, pageTree, quickSwitchFor, renameDialog, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// PG16, the page tree in the three browsers (M13/P2/S2; M12-polish item 14): a row's menu, by the pointer and by the
// keyboard, and its rename's dialog, the focus back on the row's button each time; the row of the page shown brought
// into the left column's view, which scrolls itself, the window not (decisions 779, 789, 791). The page's version
// only: the cells check the interface's behaviour and write nothing, the API's and MCP's versions exempt (v0.1
// design 13.4, item 3).

test("PG16 (page): a row's menu in the tree opens at its button, by the pointer and by the keyboard; Escape closes it, Rename opens its dialog, which Escape cancels; the focus goes back to the row's button each time", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const page = await signedInPage(
    await joinOnboarded(api, adminPat, workspace.slug, emailFor(testInfo, "editor"), "member")
  );
  // Open to the workspace to write: the editor writes by the workspace's access.
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Plans", "editor");
  const roadmap = await createPage(api, adminPat, notebook.id, "Roadmap");
  await createPage(api, adminPat, notebook.id, "Notes");
  await goto(page, wikiPagePath(workspace.slug, notebook.id, roadmap.id));
  await expect(pageHeading(page, "Roadmap")).toBeVisible();
  const button = pageTree(page, "Plans").getByRole("button", { name: "Actions for Notes", exact: true });
  const menu = page.getByRole("menu", { name: "Actions for Notes", exact: true });

  await button.click();
  await expect(menu).toBeVisible();
  expect(await menu.getByRole("menuitem").allTextContents()).toEqual(["New subpage", "Rename", "Move to…", "Delete"]);
  await page.keyboard.press("Escape");
  await expect(menu).toBeHidden();
  await expect(button).toBeFocused();

  await page.keyboard.press("Enter");
  await expect(menu).toBeVisible();
  await expect(menu.getByRole("menuitem", { name: "New subpage", exact: true })).toBeFocused();
  await menu.getByRole("menuitem", { name: "Rename", exact: true }).click();
  const dialog = renameDialog(page, "Notes");
  const title = dialog.getByLabel("Title", { exact: true });
  await expect(title).toBeFocused();
  await expect(title).toHaveValue("Notes");
  // The dialog's cancel: it has Save alone.
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(button).toBeFocused();
});

/** Creates the pages titled titles in the notebook notebookId, one after the other, each last at its root. */
async function createInOrder(
  api: ApiClient,
  pat: string,
  notebookId: string,
  titles: readonly string[],
  content: (title: string) => string | undefined
): Promise<WikiPage[]> {
  const [title, ...rest] = titles;
  if (title === undefined) {
    return [];
  }
  const page = await createPage(api, pat, notebookId, title, null, content(title));
  return [page, ...(await createInOrder(api, pat, notebookId, rest, content))];
}

test("PG16 (page): the page shown has its row brought into the left column, which scrolls itself; the window does not scroll", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const titles = Array.from({ length: 60 }, (_, at) => `Page ${String(at + 1).padStart(2, "0")}`);
  // The last one taller than the window, which could scroll then. From md: up the column sticks, all of it in view,
  // and the row is brought in as the tree arrives, before the page's body has made the document tall: a
  // scrollIntoView would not scroll the window here either (M13/P2/S2's m24, in the three engines); at a phone's width
  // it would, below.
  const long = Array.from({ length: 200 }, (_, at) => `Line ${String(at + 1)}.`).join("\n\n");
  const pages = await createInOrder(api, pat, notebook.id, titles, (title) =>
    title === "Page 60" ? `${long}\n` : undefined
  );
  const last = pages.at(-1);
  if (last === undefined) {
    throw new Error("no page was created");
  }
  const page = await signedInPage(tokens);
  const area = page.locator("[data-sidebar-scroll]");
  const row = (title: string) => pageTree(page, "Plans").getByRole("link", { name: title, exact: true });
  const windowScroll = () => page.evaluate(() => window.scrollY);

  await goto(page, wikiPagePath(workspace.slug, notebook.id, last.id));
  await expect(pageHeading(page, "Page 60")).toBeVisible();
  await expect(row("Page 60")).toBeInViewport();
  expect(await area.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
  expect(await windowScroll()).toBe(0);

  // Then twice by the quick switch, keys pressed before each (review B-I1).
  expect(await quickSwitchFor(page, "Page 01")).toEqual([["Page 01", ""]]);
  await page.keyboard.press("Enter");
  await expect(pageHeading(page, "Page 01")).toBeVisible();
  await expect(row("Page 01")).toBeInViewport();
  expect(await quickSwitchFor(page, "Page 60")).toEqual([["Page 60", ""]]);
  await page.keyboard.press("Enter");
  await expect(pageHeading(page, "Page 60")).toBeVisible();
  await expect(row("Page 60")).toBeInViewport();
  expect(await windowScroll()).toBe(0);

  // At a phone's width the column comes before the page and is no scroller of its own (until the drawer, M13/P2/S3):
  // the window is left at the top, the row far below it, where a scrollIntoView would scroll the window. By the
  // notebook's home: Chromium and WebKit load the same address over itself in its entry, the arrival kept, and the
  // heading took the focus again, the window scrolled to it.
  await page.setViewportSize({ width: phoneWidth, height: 812 });
  await goto(page, notebookPath(workspace.slug, notebook.id));
  await expect(notebookHeading(page, "Plans")).toBeVisible();
  await goto(page, wikiPagePath(workspace.slug, notebook.id, last.id));
  await expect(pageHeading(page, "Page 60")).toBeVisible();
  await expect(row("Page 60")).toBeAttached();
  expect(await windowScroll()).toBe(0);
});
