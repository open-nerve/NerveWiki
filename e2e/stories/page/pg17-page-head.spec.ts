import type { Locator, Page } from "@playwright/test";

import { bearer } from "../../fixtures/auth";
import { countAnswers } from "../../fixtures/browser";
import { holdStream } from "../../fixtures/events";
import { expectFits } from "../../fixtures/narrow";
import { goto, reload } from "../../fixtures/navigation";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, writeContent } from "../../fixtures/pages";
import { holdAnswer } from "../../fixtures/settings-pages";
import { expect, test } from "../../fixtures/test";
import { editorContent, editStatus, pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// PG17, the page's head and its right column in the three browsers (M13/P4/S1, decision 879): who edited the page
// last, in the head, coming late with Edit kept where it is; the right column beside the page, the window's height,
// its outline marking the section read (a folded callout's heading never, the heading clicked at the page's end), the
// column following it into view until the reader moves in it; the line read kept as the page is written above it; the
// column folded away, the page's column as wide and centred, folded still once reloaded, after the page below xl. The
// page's version only: the cells check the interface's behaviour, writing nothing but their data and one write through
// the API (for the update in place), the API's and MCP's versions exempt (v0.1 design 13.4, item 3).

/** A value of 97 characters without a break: the properties' strip breaks it in its chip (decision 864). */
const long = "x".repeat(97);

/** The page's content: frontmatter, then 40 parts, the 20th ending in a folded callout whose first line is a heading. */
function content(): string {
  const parts = Array.from({ length: 40 }, (_, at) => {
    const n = at + 1;
    const title = `## Part ${String(n).padStart(2, "0")}\n\n`;
    const body = "filler\n\n".repeat(n <= 38 ? 12 : 2);
    return title + body + (n === 20 ? "> [!note]- Folded\n> ## Hidden\n> inside\n\n" : "");
  });
  return `---\nstatus: draft\nowner: "[[Hub]]"\ntags: [alpha, beta]\ncode: ${long}\n---\n\n${parts.join("")}`;
}

/** afterFrames settles once page has drawn two frames: a layout's scroll anchoring is done by then. */
function afterFrames(page: Page): Promise<unknown> {
  return page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
}

/** top is where element's top is in its window, in CSS pixels: of one not shown (in a closed details) too. */
function top(element: Locator): Promise<number> {
  return element.evaluate((each) => each.getBoundingClientRect().top);
}

/** scrollTo scrolls page's window so that element's top is at y. */
function scrollTo(element: Locator, y: number): Promise<void> {
  return element.evaluate((each, at) => {
    window.scrollBy(0, each.getBoundingClientRect().top - at);
  }, y);
}

/**
 * holdVersions holds the answers to page's reads of the page id's versions (whatever their query), as holdAnswer does
 * (fixtures/settings-pages.ts): the server answers at once, the page gets them once the function returned is called.
 */
async function holdVersions(page: Page, id: string): Promise<() => void> {
  let release: (() => void) | undefined;
  const released = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route(
    (url) => url.pathname === `/api/v0/pages/${id}/versions`,
    async (route) => {
      if (route.request().method() !== "GET") {
        await route.fallback();
        return;
      }
      const response = await route.fetch();
      await released;
      await route.fulfill({ response });
    }
  );
  return () => release?.();
}

/** box is element's box, which it has. */
async function box(element: Locator): Promise<{ x: number; y: number; width: number; height: number }> {
  const placed = await element.boundingBox();
  if (placed === null) {
    throw new Error(`${String(element)} has no box`);
  }
  return placed;
}

test("PG17 (page): the page's head says who edited the page last, Edit kept where it is as that comes; beside it the right column, the window's height, outlines the page and marks the section read, a folded callout's heading never, following it into view until the reader moves in it, the heading clicked marked at the page's end; keeps the line read where it is as the page is written above it, and folds away, the page's column as wide and centred, folded still once reloaded; narrower it follows the page, and at 375 wide the properties wrap", async ({
  api,
  signedInPage,
}, testInfo) => {
  test.slow();
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const me = await api.GET("/api/v0/me", { headers: bearer(pat) });
  const name = me.data?.display_name ?? "";
  expect(name.length, "the admin's display name").toBeGreaterThan(0);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const doc = await createPage(api, pat, notebook.id, "Doc", null, content());
  const page = await signedInPage(tokens);
  await page.setViewportSize({ width: 1440, height: 900 });
  const viewPath = `/api/v0/pages/${doc.id}/view`;
  const viewReads = countAnswers(page, "GET", viewPath);
  const letStreamIn = await holdStream(page);
  const main = page.getByRole("main");
  const article = page.getByRole("article", { name: "Doc" });
  const column = page.getByRole("complementary", { name: "About this page" });
  const outline = column.getByRole("navigation", { name: "Outline" });
  const item = (title: string) => outline.getByRole("link", { name: title, exact: true });
  const heading = (title: string) => article.getByRole("heading", { level: 2, name: title, exact: true });
  const windowScroll = () => page.evaluate(() => window.scrollY);
  const columnScroll = () => column.evaluate((each) => each.scrollTop);
  try {
    await goto(page, wikiPagePath(workspace.slug, notebook.id, doc.id));
    await expect(pageHeading(page, "Doc")).toBeVisible();
    await expect(heading("Part 40")).toBeAttached();
    letStreamIn();
    // The connection's refresh reads the view again: from then on another's write comes as an event.
    await expect.poll(viewReads).toBeGreaterThanOrEqual(2);
  } finally {
    letStreamIn();
  }

  // Who edited it last, its name in a bdi: a paragraph, no status (the column's Loading… is one, which is not counted).
  const edited = main.getByText(/^Edited by /);
  await expect(edited).toBeVisible();
  await expect(edited.locator("bdi")).toHaveText(name);
  await expect(main.getByRole("status").filter({ hasText: "Edited by" })).toHaveCount(0);

  // The right column, right of the page's column with the content area's 40px between, the window's height, held.
  const shown = await box(column);
  const body = await box(article);
  expect(shown.x, "the column's left").toBeGreaterThanOrEqual(body.x + body.width + 39);
  // WebKit places the held column a fraction of a pixel off at the page's end.
  expect(Math.abs(shown.y), "the column's top").toBeLessThanOrEqual(0.5);
  expect(Math.abs(shown.height - 900), "the column's height off the window's").toBeLessThanOrEqual(0.5);
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  expect(Math.abs((await box(column)).y), "the column's top at the page's end").toBeLessThanOrEqual(0.5);
  await page.evaluate(() => window.scrollTo(0, 0));

  // A heading of the outline clicked: it shows 16px from the top, focused, its item the location.
  await item("Part 05").click();
  await expect(heading("Part 05")).toBeFocused();
  await expect.poll(async () => Math.abs((await top(heading("Part 05"))) - 16)).toBeLessThanOrEqual(2);
  await expect(item("Part 05")).toHaveAttribute("aria-current", "location");
  await expect(outline.locator("[aria-current]")).toHaveCount(1);

  // Scrolled to, a section is read.
  await scrollTo(heading("Part 10"), 0);
  await expect(item("Part 10")).toHaveAttribute("aria-current", "location");

  // A folded callout's heading is never the section read (decision 887): its own box 5px above the line, Part 20 is.
  const hidden = article.locator("details h2");
  const line = await page.evaluate(() => window.innerHeight / 3);
  await scrollTo(hidden, line - 5);
  await expect(item("Part 20")).toHaveAttribute("aria-current", "location");
  await expect(item("Hidden")).toBeVisible();
  await expect(item("Hidden")).not.toHaveAttribute("aria-current");
  await expect(outline.locator("[aria-current]")).toHaveCount(1);

  // The page written above the section read: the line read stays where it is, the window scrolled (anchoring).
  await scrollTo(heading("Part 21"), 200);
  const read = heading("Part 21").locator("xpath=following-sibling::p[1]");
  const readAt = await top(read);
  const scrolledBefore = await windowScroll();
  const added = content().replace("## Part 01\n\n", "## Part 01\n\nA paragraph added above.\n\n");
  await writeContent(api, pat, doc.id, { content: added, base_revision: doc.revision });
  await expect(article.getByText("A paragraph added above.", { exact: true })).toBeAttached();
  await afterFrames(page);
  expect(Math.abs((await top(read)) - readAt), "how far the line read moved").toBeLessThanOrEqual(1);
  expect(await windowScroll()).toBeGreaterThan(scrolledBefore);

  // The column follows the section read into its view, as far as shows it, the window not moved (decision 886).
  const followedFrom = await columnScroll();
  await scrollTo(heading("Part 36"), 0);
  const scrolledTo = await windowScroll();
  await expect(item("Part 36")).toHaveAttribute("aria-current", "location");
  await afterFrames(page);
  expect(await columnScroll()).toBeGreaterThan(followedFrom);
  const columnBottom = (await box(column)).y + (await box(column)).height;
  const itemBox = await box(item("Part 36").locator(".."));
  const below = columnBottom - (itemBox.y + itemBox.height);
  expect(below, "the item's bottom above the column's").toBeGreaterThanOrEqual(0);
  expect(below, "the item's bottom from the column's").toBeLessThanOrEqual(2);
  expect(await windowScroll()).toBe(scrolledTo);

  // The reader scrolls the column: it follows no more, until the next page.
  const columnBox = await box(column);
  await page.mouse.move(columnBox.x + columnBox.width / 2, columnBox.y + 400);
  await page.mouse.wheel(0, -4000);
  await expect.poll(columnScroll).toBe(0);
  await scrollTo(heading("Part 38"), 0);
  await expect(item("Part 38")).toHaveAttribute("aria-current", "location");
  await afterFrames(page);
  expect(await columnScroll()).toBe(0);

  // At the page's end the heading clicked is marked, not the last one shown (decision 893).
  await item("Part 39").click();
  await expect
    .poll(() => page.evaluate(() => window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 1))
    .toBe(true);
  await expect(heading("Part 40")).toBeInViewport();
  await expect(heading("Part 39")).toBeFocused();
  await expect(item("Part 39")).toHaveAttribute("aria-current", "location");
  await expect(item("Part 40")).not.toHaveAttribute("aria-current");

  // Folded away: the page's column as wide, centred in the content area (the main less its 40px each side).
  const toggle = main.getByRole("button", { name: "About this page", exact: true });
  const width = (await box(article)).width;
  await toggle.focus();
  await page.keyboard.press("Enter");
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(column).toBeHidden();
  const folded = await box(article);
  const area = await box(main);
  expect(folded.width).toBe(width);
  expect(
    Math.abs(folded.x + folded.width / 2 - (area.x + area.width / 2)),
    "off the content area's centre"
  ).toBeLessThanOrEqual(1);
  await expect(toggle).toBeFocused();

  // Reloaded, it is folded still; shown again by its toggle.
  await reload(page);
  await expect(pageHeading(page, "Doc")).toBeVisible();
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(column).toBeHidden();
  await toggle.focus();
  await page.keyboard.press("Enter");
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await expect(column).toBeVisible();
  await page.keyboard.press("Enter");
  await expect(column).toBeHidden();

  // Below xl: no toggle, the column after the page, folded or not.
  await page.setViewportSize({ width: 1024, height: 800 });
  await expect(toggle).toBeHidden();
  await expect(column).toBeVisible();
  const after = await box(column);
  const above = await box(article);
  expect(after.y, "the column's top").toBeGreaterThan(above.y + above.height);

  // Who edited it, come late at 480 wide with a long name: Edit does not move (decisions 858, 894).
  await page.setViewportSize({ width: 480, height: 800 });
  const release = await holdVersions(page, doc.id);
  await reload(page);
  const edit = main.getByRole("button", { name: "Edit", exact: true });
  await expect(edit).toBeVisible();
  await expect(edited).toHaveCount(0);
  const before = await box(edit);
  release();
  await expect(edited).toBeVisible();
  await afterFrames(page);
  const now = await box(edit);
  expect(Math.abs(now.x - before.x), "how far Edit moved across").toBeLessThanOrEqual(0.5);
  expect(Math.abs(now.y - before.y), "how far Edit moved down").toBeLessThanOrEqual(0.5);

  // At 375 wide the properties wrap, a long value broken in its chip: the page fits, no region (decision 864).
  await page.setViewportSize({ width: 375, height: 800 });
  await expectFits(page);
  const strip = article.locator("table.nw-props > tbody");
  const chips = strip.locator(":scope > tr");
  const stripBox = await box(strip);
  const firstChip = await box(chips.first());
  expect(stripBox.height, "the strip's height").toBeGreaterThan(firstChip.height);
  expect((await box(chips.last())).y, "the last chip's top").not.toBe(firstChip.y);
  // The regions are marked by a ResizeObserver's callback: read at once, the count would pass before it (review C's M3).
  await afterFrames(page);
  await expect(article.getByRole("region")).toHaveCount(0);
});

/**
 * keepsItsLines checks, at width, that the title of the page doc shown in page does not move as the page is read
 * (arrived at by enter), its edit entered, its content awaited (no button in the head's right group), typed in and
 * saved, then read again; nor the status's right edge as it saves (decision 894). The window is at the page's top.
 */
async function keepsItsLines(page: Page, doc: { id: string }, width: number, enter: () => Promise<unknown>) {
  const main = page.getByRole("main");
  const edit = main.getByRole("button", { name: "Edit", exact: true });
  const titleAt = async () => (await box(pageHeading(page, "Doc"))).y;
  const statusRight = async () => {
    const shown = await box(editStatus(page));
    return shown.x + shown.width;
  };
  const still = async (when: string, at: number) =>
    expect(Math.abs((await titleAt()) - at), `${String(width)}: the title, ${when}`).toBeLessThanOrEqual(0.5);
  await page.setViewportSize({ width, height: 800 });
  await enter();
  await expect(main.getByText(/^Edited by /)).toBeVisible();
  const at = await titleAt();

  // In the edit, its content not read yet: no button in the head's right group (decision 894).
  const release = await holdAnswer(page, "GET", `/api/v0/pages/${doc.id}/content`);
  await edit.focus();
  await page.keyboard.press("Enter");
  await expect(edit).toHaveCount(0);
  await afterFrames(page);
  await still("the content awaited", at);
  release();
  await expect(editorContent(page)).toBeVisible();
  await expect(editStatus(page)).toHaveText("");
  await still("editing", at);

  await editorContent(page).press("End");
  await page.keyboard.type("x");
  await expect(editStatus(page)).toHaveText("Unsaved changes");
  await still("typed in", at);
  const right = await statusRight();

  // Saved by itself, the focus in the editor all along.
  await expect(editStatus(page)).toHaveText("Saved.", { timeout: 10_000 });
  await expect(editorContent(page)).toBeFocused();
  await still("saved", at);
  expect(Math.abs((await statusRight()) - right), `${String(width)}: the status's right`).toBeLessThanOrEqual(0.5);

  await page.keyboard.press("ControlOrMeta+e");
  await expect(edit).toBeVisible();
  await still("read again", at);
}

test("PG17 (page): the page's head keeps its lines whatever its mode, the title where it is as the page is read, its content awaited, edited, typed in and saved, at 375 and 820 wide", async ({
  api,
  signedInPage,
}, testInfo) => {
  test.slow();
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const doc = await createPage(api, pat, notebook.id, "Doc", null, "One\n\nTwo\n");
  const page = await signedInPage(tokens);

  await keepsItsLines(page, doc, 375, () => goto(page, wikiPagePath(workspace.slug, notebook.id, doc.id)));
  // 820 wide the left column is there, the head's row about 519px: the revised draft's one line reading, two editing.
  await keepsItsLines(page, doc, 820, () => reload(page));
});
