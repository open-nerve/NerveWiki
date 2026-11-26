import type { ApiClient, Page as WikiPage } from "@nervewiki/api-client";
import type { Locator, Page, Request } from "@playwright/test";

import { emailFor } from "../../fixtures/auth";
import { joinOnboarded } from "../../fixtures/invitations";
import { phoneWidth } from "../../fixtures/narrow";
import { goto } from "../../fixtures/navigation";
import { notebookHeading, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { openDrawer } from "../../fixtures/shell-pages";
import { expect, test } from "../../fixtures/test";
import {
  pageHeading,
  pageTree,
  quickSwitchFor,
  renameDialog,
  treeTitles,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

/** What happened as a key was held: the menus and the dialogs that came and went, the tree's new rows, the writes. */
type Held = { menus: [number, number]; dialogs: [number, number]; rows: number; writes: string[] };

/**
 * recordHeld records what happens in page from now: the DOM's changes as they are made, a menu that opened and closed
 * again counted, and the requests that write. Its result reads the record, two frames on. A repeat that did something
 * shows in it however it was timed, where an expectation waits for a state and reads it once (M13 closeout fix check
 * 1, review C's M13CC-F1-M1: a repeat let through opened the menu and made a page in Chromium, the step passing).
 */
async function recordHeld(page: Page): Promise<() => Promise<Held>> {
  const writes: string[] = [];
  const onRequest = (request: Request) => {
    const { pathname } = new URL(request.url());
    if (request.method() !== "GET" && pathname.startsWith("/api/")) {
      writes.push(`${request.method()} ${pathname}`);
    }
  };
  page.on("request", onRequest);
  await page.evaluate(() => {
    const held: Omit<Held, "writes"> = { menus: [0, 0], dialogs: [0, 0], rows: 0 };
    // oxlint-disable-next-line unicorn/consistent-function-scoping -- serialized into the page: it stays in the body
    const count = (node: Node, selector: string) =>
      node instanceof Element ? Number(node.matches(selector)) + node.querySelectorAll(selector).length : 0;
    const dialog = "[role=dialog], [role=alertdialog]";
    const tally = (changes: MutationRecord[]) => {
      for (const change of changes) {
        const tree = change.target instanceof Element && change.target.closest("nav[aria-label^='Pages of']") !== null;
        for (const node of change.addedNodes) {
          held.menus[0] += count(node, "[role=menu]");
          held.dialogs[0] += count(node, dialog);
          held.rows += tree ? count(node, "li") : 0;
        }
        for (const node of change.removedNodes) {
          held.menus[1] += count(node, "[role=menu]");
          held.dialogs[1] += count(node, dialog);
        }
      }
    };
    const observer = new MutationObserver(tally);
    observer.observe(document.body, { childList: true, subtree: true });
    Object.assign(window, {
      heldRecord: () => {
        tally(observer.takeRecords());
        observer.disconnect();
        return held;
      },
    });
  });
  return async () => {
    const held = await page.evaluate(async () => {
      await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      return (window as unknown as { heldRecord: () => Omit<Held, "writes"> }).heldRecord();
    });
    page.off("request", onRequest);
    return { ...held, writes };
  };
}

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
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(button).toBeFocused();

  // Enter held on a dialog's Cancel closes it; its repeats fall on the row's button the focus goes back to, and press
  // nothing there: they opened its menu, then made a page by its first item (M13 closeout, review B's M13CB-M1,
  // decision 967). The key's first press, then its repeats once the focus is back, then let go: the dialog closed, no
  // menu nor dialog opened meanwhile, no row, no write.
  await button.press("Enter");
  await menu.getByRole("menuitem", { name: "Move to…", exact: true }).click();
  const move = page.getByRole("dialog", { name: "Move Notes", exact: true });
  await move.getByRole("button", { name: "Cancel", exact: true }).focus();
  const cancelled = await recordHeld(page);
  await page.keyboard.down("Enter");
  await expect(move).toBeHidden();
  await expect(button).toBeFocused();
  await repeats(page);
  await page.keyboard.up("Enter");
  expect(await cancelled()).toEqual({ menus: [0, 0], dialogs: [0, 1], rows: 0, writes: [] });
  await expect(button).toBeFocused();
  await expect(menu).toBeHidden();

  // Enter held on the row's button opens its menu, and its repeats, the first item focused, press none of its items:
  // they made a page by New subpage, the menu gone, the focus on its heading (M13 closeout fix check 1, review B's
  // M13CB-F1-M1). The first press, then its repeats once the menu has the focus, as a system repeats a key after a
  // while.
  const opened = await recordHeld(page);
  await page.keyboard.down("Enter");
  const first = menu.getByRole("menuitem", { name: "New subpage", exact: true });
  await expect(first).toBeFocused();
  await repeats(page);
  await page.keyboard.up("Enter");
  expect(await opened()).toEqual({ menus: [1, 0], dialogs: [0, 0], rows: 0, writes: [] });
  await expect(first).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(menu).toBeHidden();
  await expect(button).toBeFocused();

  // Enter held on an item that opens a dialog opens it once, and its repeats press nothing in it: they sent Rename's
  // form, which closed, and in WebKit Move's, a write (M13 closeout fix check 2, review C's M13CC-F2-M1). The menu
  // opened by the pointer, the item focused; the key's first press, its repeats once the dialog has the focus, let go:
  // the dialog open still, nothing written.
  const heldOn = async (item: string, opens: Locator) => {
    await button.click();
    await menu.getByRole("menuitem", { name: item, exact: true }).focus();
    const pressed = await recordHeld(page);
    await page.keyboard.down("Enter");
    await expect(opens.locator(":focus")).toHaveCount(1);
    await repeats(page);
    await page.keyboard.up("Enter");
    expect(await pressed(), item).toEqual({ menus: [0, 1], dialogs: [1, 0], rows: 0, writes: [] });
    await expect(opens).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(opens).toBeHidden();
    await expect(button).toBeFocused();
  };
  await heldOn("Move to…", move);
  await heldOn("Rename", renameDialog(page, "Notes"));
  await expect(pageHeading(page, "Roadmap")).toBeVisible();
  expect(await treeTitles(page, "Plans")).toEqual(["Roadmap", "Notes"]);
});

/** repeats has the key held, Enter, repeat three times. */
async function repeats(page: Page): Promise<void> {
  for (let repeat = 0; repeat < 3; repeat += 1) {
    // oxlint-disable-next-line no-await-in-loop -- a key's repeats, one after the other
    await page.keyboard.down("Enter");
  }
}

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
  // the room it leaves below the row tells, below.
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

  // The column's middle scrolls, which has Firefox make it a stop of Tab, nameless, between Search and Home: it is none
  // (M13 closeout, review A's M13CA-M3). Playwright's WebKit on macOS tabs to fields alone, as the system has it.
  if (testInfo.project.name !== "webkit") {
    await page.getByRole("search").getByRole("button", { name: "Search", exact: true }).focus();
    await page.keyboard.press("Tab");
    await expect(page.locator("[data-shell]").getByRole("link", { name: "Home", exact: true })).toBeFocused();
  }

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

  // At a phone's width the column is the drawer (M13/P2/S3), fixed: a scrollIntoView would no more scroll the window
  // than the reveal does, closed or open (a probe of the three engines), and what tells the reveal's own from one is
  // the 8px it leaves below the row, which the drawer opened shows: the row is brought in as it was closed, inert and
  // off the screen. By the notebook's home: Chromium and WebKit load the same address over itself in its entry, the
  // arrival kept, and the heading took the focus again.
  await page.setViewportSize({ width: phoneWidth, height: 812 });
  await goto(page, notebookPath(workspace.slug, notebook.id));
  await expect(notebookHeading(page, "Plans")).toBeVisible();
  await goto(page, wikiPagePath(workspace.slug, notebook.id, last.id));
  await expect(pageHeading(page, "Page 60")).toBeVisible();
  await openDrawer(page);
  await expect(row("Page 60")).toBeInViewport();
  const below = await row("Page 60").evaluate((link) => {
    const column = link.closest("[data-sidebar-scroll]");
    return column === null ? undefined : column.getBoundingClientRect().bottom - link.getBoundingClientRect().bottom;
  });
  expect(below, "the room below the row").toBeGreaterThanOrEqual(7);
  expect(below, "the room below the row").toBeLessThanOrEqual(9);
  expect(await windowScroll()).toBe(0);
});
