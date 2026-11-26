import type { Locator, Page } from "@playwright/test";

import { bearer, emailFor } from "../../fixtures/auth";
import { countAnswers } from "../../fixtures/browser";
import { grantClipboardPermissions, readClipboardText } from "../../fixtures/clipboard";
import { holdStream } from "../../fixtures/events";
import { joinAs } from "../../fixtures/invitations";
import { expectFits, phoneWidth } from "../../fixtures/narrow";
import { goto, reload } from "../../fixtures/navigation";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, openSession, writeContent } from "../../fixtures/pages";
import { holdAnswer } from "../../fixtures/settings-pages";
import { expect, test } from "../../fixtures/test";
import { editorContent, editStatus, pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// PG17, the page's head and its right column in the three browsers (M13/P4/S1, decision 879): who edited the page last,
// in the head, coming late with Edit kept where it is; the right column beside the page, the window's height, its
// outline marking the section read (a folded callout's heading never, the heading clicked at the page's end), the
// column following it into view until the reader moves in it; the line read kept as the page is written above it; the
// column folded away, the page's column as wide and centred, folded still once reloaded, after the page below xl; long
// names broken, the lock's holder's in its note, a version's author's in its line (review B's P4S1B-F1); a code block's
// head, its language and Copy, a done task item and its blocks, a property's map's chip, a callout's title and its
// blocks, a list item's (M13/P4/S2, decisions 869, 872, 890, 901–903; the fix checks); the edit's outline's items
// (M13/P4/S3's review) and its first jump down a long page; the trail's separators each on the line of a name, the
// right column no stop of Tab (M13 closeout). The page's version only: the cells check the interface's behaviour,
// writing nothing but their data and one write through the API (for the update in place), the API's and MCP's versions
// exempt (v0.1 design 13.4, item 3).

/** A value of 97 characters without a break: the properties' strip breaks it in its chip (decision 864). */
const long = "x".repeat(97);

/**
 * unbroken is a long name of who's: hyphens before digits, as a name taken from an address may have them, where Firefox
 * breaks no line (UAX #14's HY × NU), what shows it broken anywhere in it (review B's P4S1B-F1).
 */
function unbroken(who: string): string {
  return `${who}-${"2026-11-24-".repeat(7)}0`;
}

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
  // The admin's long name is shortened alone, the time after it kept whole (decision 899).
  const cut = await edited.evaluate((sentence) => {
    const shown = sentence.querySelector("bdi");
    return {
      sentence: sentence.scrollWidth - sentence.clientWidth,
      name: shown === null ? 0 : shown.scrollWidth - shown.clientWidth,
    };
  });
  expect(cut.sentence, "how much of the sentence is cut").toBeLessThanOrEqual(0);
  expect(cut.name, "how much of the name is cut").toBeGreaterThan(0);
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

  // The column scrolls itself, which has Firefox make it a stop of Tab after the page's last control: it is none, the
  // next stop its first summary (M13 closeout, review A's M13CA-M3). Playwright's WebKit on macOS tabs to fields alone.
  if (testInfo.project.name !== "webkit") {
    expect(await column.evaluate((each) => each.scrollHeight > each.clientHeight), "the column scrolls").toBe(true);
    await main.getByRole("button", { name: "Upload", exact: true }).focus();
    await page.keyboard.press("Tab");
    await expect(column.locator("summary").first()).toBeFocused();
    await page.evaluate(() => window.scrollTo(0, 0));
  }

  // A heading of the outline clicked: it shows 16px from the top, focused, its item the location.
  await item("Part 05").click();
  await expect(heading("Part 05")).toBeFocused();
  await expect.poll(async () => Math.abs((await top(heading("Part 05"))) - 16)).toBeLessThanOrEqual(2);
  await expect(item("Part 05")).toHaveAttribute("aria-current", "location");
  await expect(outline.locator("[aria-current]")).toHaveCount(1);

  // Scrolled to, a section is read: its heading 100px down, above the line a third of the window down (Part 09's would
  // be read at a line at the window's top).
  await scrollTo(heading("Part 10"), 100);
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

  // Reloaded, it is folded still; shown again by its toggle. The address keeps the anchor clicked: the view's arrival
  // gives its heading the focus, which is awaited, or a view come late would take it from the toggle between its two
  // presses (review C's P4S1C-F1-I1).
  await reload(page);
  await expect(pageHeading(page, "Doc")).toBeVisible();
  await expect(heading("Part 39")).toBeFocused();
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

  // Who edited it, come late at 480 wide with a long name: Edit does not move (decisions 858, 894). The page opened at
  // its address without the anchor clicked: the view's arrival at it would scroll the window to its heading, Edit with
  // it, as WebKit had it once at load.
  await page.setViewportSize({ width: 480, height: 800 });
  const release = await holdVersions(page, doc.id);
  await goto(page, wikiPagePath(workspace.slug, notebook.id, doc.id));
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
  // Laid out as a strip, the table keeps a table's roles, which propertyRoles gives (decision 864).
  await expect(article.locator("table.nw-props").first()).toHaveAttribute("role", "table");
  await expect(strip).toHaveAttribute("role", "rowgroup");
});

/**
 * keepsItsLines checks, at width, that the title of the page doc shown in page does not move as the page is read
 * (arrived at by enter), its edit entered, its content awaited (no button in the head's right group), typed in and
 * saved, then read again; nor the status's right edge as it saves (decision 894). The window is at the page's top.
 * Edited, the outline's items are as high as read, a hand over them, the long heading's cut at two lines: the page has
 * a heading "Short" and one of a long line.
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
  const outline = page
    .getByRole("complementary", { name: "About this page" })
    .getByRole("navigation", { name: "Outline" });
  const item = (of: Locator) => box(of.locator("xpath=ancestor::li[1]"));
  const read = await item(outline.getByRole("link", { name: "Short", exact: true }));

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

  // The edit's outline: an item as high as read, a hand over it (review B's P4S3B-M1, N2); a long heading cut at two
  // lines, in WebKit too, which lays out no button as the clamp's box (3c75954d; review C's P4S3C-M1).
  const short = outline.getByRole("button", { name: "Short", exact: true });
  await expect(short).toBeVisible();
  const edited = await item(short);
  expect(Math.abs(edited.height - read.height), `${String(width)}: an item's height, edited`).toBeLessThanOrEqual(0.5);
  expect(await short.evaluate((button) => getComputedStyle(button).cursor), `${String(width)}: the item's cursor`).toBe(
    "pointer"
  );
  const cut = outline.getByRole("button", { name: /^A long heading/ });
  const lineHeight = Number.parseFloat(await cut.evaluate((button) => getComputedStyle(button).lineHeight));
  const clamped = await box(cut);
  expect(clamped.height, `${String(width)}: the long heading's lines (line ${String(lineHeight)})`).toBeLessThanOrEqual(
    2 * lineHeight + 0.5
  );
  expect(clamped.height, `${String(width)}: the long heading wraps`).toBeGreaterThan(1.5 * lineHeight);

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

test("PG17 (page): the page's head keeps its lines whatever its mode, the title where it is as the page is read, its content awaited, edited, typed in and saved, at 375 and 820 wide; edited, the outline's items are as high as read, a long heading cut at two lines", async ({
  api,
  signedInPage,
}, testInfo) => {
  test.slow();
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const heading = "A long heading ".repeat(30);
  const doc = await createPage(api, pat, notebook.id, "Doc", null, `One\n\n## Short\n\n## ${heading}\n\nTwo\n`);
  const page = await signedInPage(tokens);

  await keepsItsLines(page, doc, 375, () => goto(page, wikiPagePath(workspace.slug, notebook.id, doc.id)));
  // 820 wide the left column is there, the head's row about 519px: the revised draft's one line reading, two editing.
  await keepsItsLines(page, doc, 820, () => reload(page));
});

test("PG17 (page): long names break, the lock's holder's in its note, a version's author's in its line: the page and its version fit at 375 wide", async ({
  api,
  signedInPage,
}, testInfo) => {
  test.slow();
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const name = async (credential: string, as: string) => {
    const { response } = await api.PATCH("/api/v0/me", { body: { display_name: as }, headers: bearer(credential) });
    expect(response.status, `${as}: the name changed`).toBe(200);
  };
  await name(pat, unbroken("admin"));
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans", "editor");
  const doc = await createPage(api, pat, notebook.id, "Doc", null, "One\n");
  await writeContent(api, pat, doc.id, { content: "One\n\nTwo\n", base_revision: doc.revision });
  const editor = await joinAs(api, pat, workspace.slug, emailFor(testInfo, "editor"), "member");
  await name(editor, unbroken("editor"));
  await openSession(api, editor, doc.id);
  const page = await signedInPage(tokens);
  await page.setViewportSize({ width: phoneWidth, height: 800 });
  const main = page.getByRole("main");
  const note = main.getByRole("status").filter({ hasText: `${unbroken("editor")} is editing this page.` });
  const path = wikiPagePath(workspace.slug, notebook.id, doc.id);
  const fitsAt = async (shown: string, ...boxes: Locator[]) => {
    await goto(page, shown);
    await expect(note).toBeVisible();
    await expectFits(page, note, ...boxes);
  };
  await fitsAt(path);
  // The version's page: the same note, and the version's author in the line under its title.
  await fitsAt(`${path}/versions/1`, main.getByRole("link", { name: unbroken("admin") }));
});

/**
 * A long page's Markdown, as review B's probe had it (M13 closeout, M13CB-I1): frontmatter, a table, lists, tasks,
 * callouts, code, diagrams, maths, then twelve parts. Its editor opened at the page's top has not drawn the lines from
 * the tasks on: their heights are its estimates.
 */
const guide = [
  "---",
  "status: draft",
  'owner: "[[Hub]]"',
  "tags: [architecture, backend]",
  "reviewers:",
  "  - Ada",
  "  - Bob",
  "---",
  "",
  "Nerve Wiki is a **Go** program with *PostgreSQL* and a React page, with `inline code`, ==a highlight==, a link to [[Hub]], one to [[Missing page]] not written yet, an [outside link](https://example.com) and a #tagged word.",
  "",
  "## Modules",
  "",
  "| Module | Duty | Depends |",
  "|---|---|---|",
  "| `identity` | Accounts, sessions, tokens | — |",
  "| `workspace` | Workspaces and members | identity |",
  "| `page` | Pages, versions, the edit's lock | notebook |",
  "",
  "### Lists",
  "",
  "- one",
  "- two",
  "  - nested",
  "  - nested again",
  "- three",
  "",
  "1. first",
  "2. second",
  "3. third",
  "",
  "## Tasks",
  "",
  "- [x] Modules talk through ports only",
  "  - a step under the done one",
  "- [ ] A cache for the read path (v0.2)",
  "- [x] A done item whose text is long enough to wrap onto a second line at a phone's width, and then some more",
  "",
  "## Callouts",
  "",
  "> [!note] A note",
  "> The note's body, with a [[Hub]] link.",
  "",
  "> [!warning] Careful",
  "> The warning's body.",
  "",
  "> [!tip]- Folded tip",
  "> ## Hidden heading",
  "> inside",
  "",
  "> A plain quote.",
  "",
  "## Code",
  "",
  "```go",
  "func (s *Service) Create(ctx context.Context, in CreateInput) (Page, error) {",
  "\tif err := in.Validate(); err != nil {",
  "\t\treturn Page{}, err // fail fast",
  "\t}",
  "\treturn s.repo.Insert(ctx, in)",
  "}",
  "```",
  "",
  "```",
  `a block with no language and a long line ${"0123456789".repeat(14)}`,
  "```",
  "",
  "## Diagram",
  "",
  "```mermaid",
  "flowchart LR",
  "  A[Browser] --> B[HTTP] --> C[Use case] --> D[(PostgreSQL)]",
  "```",
  "",
  "## Math",
  "",
  "$E = mc^2$ inline, and a block:",
  "",
  "$$",
  "\\int_0^1 x^2 \\, dx = \\frac{1}{3}",
  "$$",
  "",
  "---",
  "",
  ...Array.from({ length: 12 }, (_, at) => `## Part ${String(at + 1)}\n\n${"Filler text of a part. ".repeat(20)}\n`),
].join("\n");

/**
 * settledTop is where the editor's line whose text is text settles in the window: read a frame at a time, the line
 * found anew each time (CodeMirror draws its lines anew), until its top is the same five frames running. By its place,
 * not by a time waited; NaN when the line is not drawn.
 */
function settledTop(page: Page, text: string): Promise<number> {
  return page.evaluate(async (wanted) => {
    let last = Number.NaN;
    for (let frame = 0, same = 0; frame < 300 && same < 5; frame += 1) {
      // oxlint-disable-next-line no-await-in-loop -- a frame at a time
      await new Promise((resolve) => requestAnimationFrame(resolve));
      const line = [...document.querySelectorAll(".cm-content .cm-line")].find((each) => each.textContent === wanted);
      const at = line === undefined ? Number.NaN : line.getBoundingClientRect().top;
      same = at === last ? same + 1 : 0;
      last = at;
    }
    return last;
  }, text);
}

// The edit's outline's first jump from the page's top, to a heading its editor has not drawn: the lines it passed
// were CodeMirror's estimates, drawn and measured as it got there, which left the heading's line above the window,
// the cursor on it unseen (M13 closeout, review B's M13CB-I1). The line settles at the window's top: 16px down
// (decision 876), within [0, 48] whatever a font makes of it.
test("PG17 (page): edited from the page's top, the outline's first jump to a heading not drawn yet leaves its line at the window's top", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const doc = await createPage(api, pat, notebook.id, "Guide", null, guide);
  const page = await signedInPage(tokens);
  await page.setViewportSize({ width: 1440, height: 900 });
  const main = page.getByRole("main");
  const outline = page
    .getByRole("complementary", { name: "About this page" })
    .getByRole("navigation", { name: "Outline" });
  const jumpTo = async (heading: string) => {
    await goto(page, wikiPagePath(workspace.slug, notebook.id, doc.id));
    await expect(pageHeading(page, "Guide")).toBeVisible();
    await main.getByRole("button", { name: "Edit", exact: true }).click();
    await outline.getByRole("button", { name: heading, exact: true }).click();
    const settled = await settledTop(page, `## ${heading}`);
    expect(settled, `where ## ${heading} settles`).toBeGreaterThanOrEqual(0);
    expect(settled, `where ## ${heading} settles`).toBeLessThanOrEqual(48);
    // Left by going to the page anew, not by Done: Done far down a long page makes Firefox warn of a scroll-linked
    // positioning effect, with this jump or without it, as scrolling did before (M13/P4 design 9, for v0.2).
  };
  await jumpTo("Tasks");
  await jumpTo("Callouts");
});

/**
 * A long page of prose, as review B's probe had it (M13 closeout fix check 1, M13CB-F1-N1): forty sections, each a
 * paragraph that wraps over several lines, a Chinese one every third, and a list. Its editor opened at the page's top
 * has estimated the heights of the lines it has not drawn; drawn as a scroll shows them, they change.
 */
const prose = Array.from({ length: 40 }, (_, at) => {
  const n = String(at + 1);
  const en =
    `A paragraph of section ${n} that runs long enough to wrap over several lines at any width, in a proportional face, with words of uneven lengths like interoperability, a, of, configuration and so on. `.repeat(
      3
    );
  const zh = "这是一段用来让编辑器估计行高的中文段落，没有换行地一直写下去，行宽不同时折成的行数也不同。".repeat(4);
  return `## Section ${n}\n\n${en}\n\n${(at + 1) % 3 === 0 ? `${zh}\n\n` : ""}- an item ${n}\n- another item, longer, ${"word ".repeat(30)}\n`;
}).join("\n");

/**
 * scrollAfterJump scrolls the window by `by` once the outline's jump has put the line whose text is text at its place,
 * in [0, 48] below the window's top or, a line the window cannot bring so far up, the window at its foot; six frames on,
 * as review B's probe scrolled 100ms after the jump. Then it reads the line's top on the screen a frame at a time for
 * 1.2s, past the realigning's second: NaN in a frame the editor has not drawn it in.
 */
function scrollAfterJump(
  page: Page,
  text: string,
  by: number
): Promise<{ inPlace: boolean; placed: number; went: number; placedTop: number; tops: number[] }> {
  return page.evaluate(
    async ([wanted, distance]) => {
      // oxlint-disable-next-line unicorn/consistent-function-scoping -- serialized into the page: it stays in the body
      const frame = () => new Promise((resolve) => requestAnimationFrame(resolve));
      const lineTop = () =>
        [...document.querySelectorAll(".cm-content .cm-line")]
          .find((each) => each.textContent === wanted)
          ?.getBoundingClientRect().top ?? Number.NaN;
      const placed = () => {
        const lineAt = lineTop();
        const root = document.documentElement;
        const foot = window.scrollY + window.innerHeight >= root.scrollHeight - 1;
        return (lineAt >= 0 && lineAt <= 48) || (foot && lineAt >= 0 && lineAt < window.innerHeight);
      };
      for (let at = 0; at < 300 && !placed(); at += 1) {
        // oxlint-disable-next-line no-await-in-loop -- a frame at a time
        await frame();
      }
      for (let at = 0; at < 6; at += 1) {
        // oxlint-disable-next-line no-await-in-loop -- a frame at a time
        await frame();
      }
      const where = {
        inPlace: placed(),
        placed: window.scrollY,
        placedTop: lineTop(),
        went: Number.NaN,
        tops: [] as number[],
      };
      window.scrollBy(0, distance);
      where.went = window.scrollY;
      const start = performance.now();
      while (performance.now() - start < 1200) {
        // oxlint-disable-next-line no-await-in-loop -- a frame at a time
        await frame();
        where.tops.push(lineTop());
      }
      return where;
    },
    [text, by] as const
  );
}

/**
 * expectStayed expects the jump's line to have stayed away from where the jump put it on the screen, each frame by more
 * than a quarter of the scroll `by`, a frame it is not drawn in away too. A scroll leaves the line about the scroll's
 * distance off, the editor's measuring of the lines the scroll shows keeping them where they are on the screen: in the
 * three engines 500px for 500px down, 1886px for 1500px up in Prose and 1301px in Guide; the realigning brought it back
 * to within 1px. Not the window's scrollY: the measuring changes the page's height, so the realigning brought the window
 * back to the line 523px to 955px past where the jump had left it, and a scroll up of 600px in Guide that stayed came
 * within 91px of it (fix check 3, review B's M13CB-F3-N1).
 */
function expectStayed({ placedTop, tops }: { placedTop: number; tops: number[] }, by: number): void {
  expect(
    tops.filter((lineAt) => Math.abs(lineAt - placedTop) <= Math.abs(by) / 4),
    `the line's top from ${String(placedTop)}`
  ).toEqual([]);
}

// The outline's jump puts its line back at the window's top for a second, as the lines it passed are measured
// (M13CB-I1). A scroll of the window meanwhile that came with no wheel, press, touch or key, a script's here, as the
// browser's find or a screen reader's come, ended nothing: Firefox brought the window back to the line from it, the
// lines it showed measured (M13 closeout fix check 1, review B's M13CB-F1-N1). Once the line has been at its place, a
// scroll that is not the editor's ends the realigning, and the window stays where it went. The last section's line the
// window cannot bring to its top: at the window's foot it is at its place, and a scroll up from there stays too; the
// three engines brought the window back to the foot (fix check 2, review B's M13CB-F2-N1, review C's M13CC-F2-N1). The
// line's place on the screen tells it, not the window's: back at the foot is past where the jump left the window, the
// page grown by the lines measured (fix check 3, review B's M13CB-F3-N1).
test("PG17 (page): a scroll of the window after the outline's jump that is not the reader's wheel, press, touch or key stays where it went", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const doc = await createPage(api, pat, notebook.id, "Prose", null, prose);
  const page = await signedInPage(tokens);
  await page.setViewportSize({ width: 1440, height: 900 });
  const main = page.getByRole("main");
  const jumpTo = async (heading: string) => {
    await goto(page, wikiPagePath(workspace.slug, notebook.id, doc.id));
    await expect(pageHeading(page, "Prose")).toBeVisible();
    await main.getByRole("button", { name: "Edit", exact: true }).click();
    await page
      .getByRole("complementary", { name: "About this page" })
      .getByRole("navigation", { name: "Outline" })
      .getByRole("button", { name: heading, exact: true })
      .click();
  };

  await jumpTo("Section 15");
  const down = await scrollAfterJump(page, "## Section 15", 500);
  expect(down.inPlace, "the line at its place").toBe(true);
  expect(down.went - down.placed, "the scroll down").toBe(500);
  expectStayed(down, 500);

  await jumpTo("Section 40");
  const up = await scrollAfterJump(page, "## Section 40", -1500);
  expect(up.inPlace, "the line at its place, the window at its foot").toBe(true);
  expect(up.placed - up.went, "the scroll up").toBe(1500);
  expectStayed(up, -1500);
});

/**
 * separatorsApart lists the trail's separators that are not on the line of their step's name, by their boxes: a "/"
 * against the name's character next to it in its item (M13 closeout, decision 968). Firefox broke the line after one
 * that stood before its step, leaving it alone on a line (review B's M13CB-M2). With widths, the trail's list is made
 * each of them wide in turn, a pixel apart, then given its own back: a name broken anywhere that fills its line to
 * within a separator's width put the separator alone on the next line, in the three engines, at widths a font decides.
 */
function separatorsApart(page: Page, widths: readonly number[] = []): Promise<string[]> {
  return page
    .getByRole("main")
    .getByRole("navigation", { name: "Breadcrumb", exact: true })
    .evaluate((nav, sweep) => {
      const list = nav.querySelector("ol");
      const apartNow = () => {
        const apart: string[] = [];
        for (const separator of nav.querySelectorAll("li [aria-hidden]")) {
          const item = separator.closest("li");
          const texts: Text[] = [];
          const walker = document.createTreeWalker(item ?? nav, NodeFilter.SHOW_TEXT);
          for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
            if (!separator.contains(node) && (node.textContent ?? "").trim() !== "") {
              texts.push(node as Text);
            }
          }
          // The name's character next to it: the first of the name after it, or the last of the one before it.
          const after = texts.find(
            (text) => separator.compareDocumentPosition(text) & Node.DOCUMENT_POSITION_FOLLOWING
          );
          const before = texts.findLast(
            (text) => separator.compareDocumentPosition(text) & Node.DOCUMENT_POSITION_PRECEDING
          );
          const name = after ?? before;
          const glyph = document.createRange();
          if (name !== undefined) {
            const at = after === undefined ? name.length - 1 : 0;
            glyph.setStart(name, at);
            glyph.setEnd(name, at + 1);
          }
          const drawn = [...glyph.getClientRects()].find((rect) => rect.width > 0);
          const mark = separator.getBoundingClientRect();
          const middle = mark.top + mark.height / 2;
          if (drawn === undefined || middle < drawn.top || middle > drawn.bottom) {
            apart.push(item?.textContent ?? "");
          }
        }
        return apart;
      };
      if (sweep.length === 0 || list === null) {
        return apartNow();
      }
      const found: string[] = [];
      for (const width of sweep) {
        list.style.maxWidth = `${String(width)}px`;
        found.push(...apartNow().map((item) => `${String(width)}px: ${item}`));
      }
      list.style.maxWidth = "";
      return found;
    }, widths);
}

/** Every width of the trail's list from 120 to 1200px, a pixel apart. */
const trailWidths = Array.from({ length: 1081 }, (_, at) => 120 + at);

// The trail's separators each on the line of their step's name, at any width, the page's trail and its version's: a
// long name broken in it left a "/" alone on a line in Firefox, at every width (review B's M13CB-M2). Each ends its
// step now, in its link, glued to the name's last character (decision 968): the trail is checked at every width of
// its own too, a pixel apart, which a name filling its line to within a separator's width shows in any engine.
test("PG17 (page): the trail keeps each separator on the line of its step's name, the page's and its version's, at 320, 375, 768 and 1440 wide and the trail at every width", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const parent = await createPage(
    api,
    pat,
    notebook.id,
    `Supercalifragilistic${"expialidocious".repeat(4)}`,
    null,
    "P\n"
  );
  const doc = await createPage(api, pat, notebook.id, unbroken("notes"), parent.id, "One\n");
  await writeContent(api, pat, doc.id, { content: "One\n\nTwo\n", base_revision: doc.revision });
  const page = await signedInPage(tokens);
  const path = wikiPagePath(workspace.slug, notebook.id, doc.id);
  const keptIn = async (shown: string) => {
    await goto(page, shown);
    await expect(page.getByRole("main").locator("[aria-current=page]")).toBeVisible();
    for (const width of [320, 375, 768, 1440]) {
      // oxlint-disable-next-line no-await-in-loop -- a width at a time
      await page.setViewportSize({ width, height: 800 });
      // oxlint-disable-next-line no-await-in-loop -- a width at a time
      expect(await separatorsApart(page), `${shown} at ${String(width)}`).toEqual([]);
    }
    expect(await separatorsApart(page, trailWidths), `${shown}, its trail 120 to 1200px wide`).toEqual([]);
  };
  await keptIn(path);
  await keptIn(`${path}/versions/1`);
});

/** A line of Go longer than the page's column (720px): its block scrolls sideways. */
const wideGo = `fmt.Println("${"a line of Go wider than the page's column ".repeat(4)}")`;

/**
 * A done task with blocks under its text: code blocks, a callout with a tag, a quote, an item nested in it not done and
 * a folded callout (a code block or the nested list between them, or they would be one quote).
 */
const shipped = [
  "- [x] Shipped",
  "  ```sh",
  "  make ship",
  "  ```",
  "  > [!note] Shipped with it",
  "  > Its body, #shipped.",
  "  ```sh",
  "  make tag",
  "  ```",
  "  > A quote under it.",
  "  - [ ] Its follow-up",
  "  > [!tip]- Folded with it",
  "  > Inside the fold.",
].join("\n");

/** Properties of a list of maps and a list of words: their chips one height (review B's F1-M1). */
const listed = [
  "---",
  "people:",
  "  - name: Ada",
  "    role: lead",
  "  - name: Grace",
  "    role: review",
  "stages: [draft, review]",
  "---",
].join("\n");

/**
 * Callouts' titles with markup in them, one with an address longer than a phone's line: lines of text (decision 901);
 * its body of three blocks, as a loose list's item is of two and a tight one's of a line and a code block (902, 903).
 */
const titled = [
  "> [!note] A title with **bold**, a [[Doc]] link, `code` and https://example.com/a/very/long/path/that/keeps/going/and/going/without/a/space",
  "> Its body.",
  ">",
  "> Its second paragraph.",
  ">",
  "> ```sh",
  "> make build",
  "> ```",
  "",
  "> [!tip]- A folded title with **bold**, a [[Doc]] link and `code`",
  "> Inside the fold.",
  "",
  "- A loose item's first paragraph.",
  "",
  "  Its second paragraph.",
  "- Another loose item.",
  "",
  "Between the lists.",
  "",
  "- Build it:",
  "  ```sh",
  "  make build",
  "  ```",
  "  - Its nested step.",
].join("\n");

test("PG17 (page): a code block's head names its language and copies its code alone, its button turning to a check a moment, held at the block's left as the code scrolls sideways; a done task's own text is struck and faint, not its nested items' nor its blocks'; a property's map is a chip as tall as a word's; a callout's title is a line of text, its markup in it, at 375 wide too, its blocks apart as a list item's are", async ({
  api,
  browserName,
  context,
  signedInPage,
}, testInfo) => {
  await grantClipboardPermissions(context, browserName);
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const markdown = `${listed}\n\`\`\`go\nfunc main() {\n\t${wideGo}\n}\n\`\`\`\n\n${shipped}\n\n${titled}\n`;
  const doc = await createPage(api, pat, notebook.id, "Doc", null, markdown);
  const page = await signedInPage(tokens);
  await page.setViewportSize({ width: 1440, height: 900 });
  await goto(page, wikiPagePath(workspace.slug, notebook.id, doc.id));
  const article = page.getByRole("article", { name: "Doc" });
  const pre = article.locator(":scope > pre");
  const head = pre.locator(".nw-code-head");
  const copy = pre.getByRole("button", { name: "Copy", exact: true });
  const said = head.locator(".nw-code-said");

  // Its head names its language: the code's class, its language- taken off.
  await expect(head).toHaveText(/^go/);
  await expect(copy).toBeVisible();

  // Copy copies the code alone; its button turns to a check, Copied said to a screen reader alone (decision 890).
  await copy.click();
  await expect
    .poll(() =>
      pre.evaluate((block) => {
        const button = block.querySelector<HTMLElement>(".nw-code-copy");
        const word = block.querySelector<HTMLElement>(".nw-code-said");
        return {
          button: button?.dataset.state,
          said: word?.textContent,
          saidState: word?.dataset.state,
          shown: Math.round((word?.getBoundingClientRect().width ?? 0) * 100) / 100 <= 1,
        };
      })
    )
    .toEqual({ button: "copied", said: "Copied", saidState: "copied", shown: true });
  const code = await pre.locator(":scope > code").evaluate((element) => element.textContent);
  expect(code).toBe(`func main() {\n\t${wideGo}\n}\n`);
  expect(await readClipboardText(page, browserName)).toBe(code);
  // A moment later, as it was; its name Copy all along.
  await expect(copy).not.toHaveAttribute("data-state");
  await expect(said).toHaveText("");
  await expect(copy).toHaveAccessibleName("Copy");

  // The block wider than the page is a region named Code; scrolled sideways, its head stays at its left.
  await expect(article.getByRole("region", { name: "Code", exact: true })).toHaveCount(1);
  await expect(pre).toHaveAttribute("role", "region");
  const scrolled = await pre.evaluate((block) => {
    block.scrollLeft = 200;
    return block.scrollLeft;
  });
  expect(scrolled).toBe(200);
  await afterFrames(page);
  const [blockLeft, headLeft] = await Promise.all([
    pre.evaluate((block) => block.getBoundingClientRect().left),
    head.evaluate((element) => element.getBoundingClientRect().left),
  ]);
  expect(Math.abs(headLeft - (blockLeft + 1)), "the head's left, the block's border in").toBeLessThanOrEqual(1);

  // A done item's own text struck and faint; its code blocks, its callouts, its quote and its nested list each an
  // inline block the item's width, which its line-through goes not into, the text's colour, the quote's its own, as far
  // apart as blocks are, the callout's tag its own colour (decision 872; S2's fix check: a block's contents were struck
  // and faint with the item's text; round 1's C-F1-M1: the quote and the folded callout).
  const done = await article
    .locator(":scope > ul > li")
    .first()
    .evaluate((item) => {
      const probe = document.createElement("span");
      document.body.append(probe);
      probe.style.color = "var(--faint)";
      const faint = getComputedStyle(probe).color;
      probe.style.color = "var(--muted-foreground)";
      const muted = getComputedStyle(probe).color;
      probe.remove();
      const tag = item.querySelector(":scope > .nw-callout .nw-tag");
      const text = getComputedStyle(item.closest("article") as Element).color;
      const named = (colour: string) =>
        colour === text ? "text" : colour === muted ? "muted" : colour === faint ? "faint" : colour;
      const style = getComputedStyle(item);
      const line = item.getBoundingClientRect().top + parseFloat(style.lineHeight);
      const width = item.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
      const blocks = [...item.querySelectorAll(":scope > :is(pre, .nw-callout, blockquote, ul)")];
      return {
        struck: style.textDecorationLine.includes("line-through"),
        faint: style.color === faint,
        blocks: blocks.map((block) => ({
          tag: block.tagName.toLowerCase(),
          display: getComputedStyle(block).display,
          colour: named(getComputedStyle(block).color),
          wide: Math.abs(block.getBoundingClientRect().width - width) <= 1,
        })),
        tag: tag !== null && getComputedStyle(tag).color === muted,
        // Under the item's line, then under each other: their margins apart, no line's room more.
        gaps: blocks.map((block, at) =>
          Math.round(
            block.getBoundingClientRect().top -
              (at === 0 ? line : (blocks[at - 1] as Element).getBoundingClientRect().bottom)
          )
        ),
      };
    });
  expect(done).toEqual({
    struck: true,
    faint: true,
    blocks: [
      { tag: "pre", display: "inline-block", colour: "text", wide: true },
      { tag: "div", display: "inline-block", colour: "text", wide: true },
      { tag: "pre", display: "inline-block", colour: "text", wide: true },
      { tag: "blockquote", display: "inline-block", colour: "muted", wide: true },
      { tag: "ul", display: "inline-block", colour: "text", wide: true },
      { tag: "details", display: "inline-block", colour: "text", wide: true },
    ],
    tag: true,
    gaps: [8, 8, 8, 8, 4, 8],
  });

  // The properties' list of maps: each map a chip as tall as a list of words' chip, its table not 0.5rem down as a
  // list item's blocks are (round 1's B-F1-M1: decision 903's rule took the nested table, its chip 8px taller).
  const chips = await article.evaluate((view) => {
    const chip = (key: string) =>
      [...view.querySelectorAll("table.nw-props > tbody > tr")]
        .find((row) => row.querySelector(":scope > th")?.textContent === key)
        ?.querySelector(":scope > td > ul > li") ?? null;
    const [people, stages] = [chip("people"), chip("stages")];
    const nested = people?.querySelector(":scope > table.nw-props") ?? null;
    return {
      chips: [people, stages].map((each) =>
        each === null ? null : Math.round(each.getBoundingClientRect().height * 4) / 4
      ),
      nested: nested === null ? null : getComputedStyle(nested).marginTop,
    };
  });
  expect(chips.chips[0], "a map's chip as tall as a word's").toBe(chips.chips[1]);
  expect([chips.chips[0] !== null, chips.nested]).toEqual([true, "0px"]);

  // A callout's title is a line of text, its bold, link and code in it, folded or not; it wraps as text does, the
  // address too: at 1440 and at 375 wide in no more lines than its text on one line fills of the room its callout
  // gives it, and two for what lines' ends no longer hold (the address, too long for any line, starting one of its
  // own; a word the line before had no room for, once the words before the address wrap too); no callout nor the view
  // scrolling sideways at 375 (decision 901; review A's I1, B's I1: a flex made each of them an item of its own, in
  // columns side by side, an address past the edge). Bounded by what the browser's own fonts measure, not by a count:
  // Linux's are wider, and at 375 the first title took more than the four lines it takes on macOS in Chromium and
  // WebKit on CI; with Verdana or Courier New for the title it takes one more than its measure and the address's line.
  const titles = article.locator(":scope > .nw-callout > :is(.nw-callout-title, summary)");
  await expect(titles).toHaveCount(2);
  const read = () =>
    titles.evaluateAll((each) =>
      each.map((title) => {
        const style = getComputedStyle(title);
        const callout = getComputedStyle(title.parentElement as Element);
        const own = parseFloat(style.paddingLeft) + parseFloat(style.paddingRight);
        // The room its text has: its callout's, less the title's own padding (a folded one's, the chevron's).
        const room =
          (title.parentElement as Element).clientWidth -
          parseFloat(callout.paddingLeft) -
          parseFloat(callout.paddingRight) -
          own;
        const lines = Math.round(title.getBoundingClientRect().height / parseFloat(style.lineHeight));
        // Its text on one line, its icon with it, as the page's fonts draw it: the title unwrapped a moment.
        const was = (title as HTMLElement).style.cssText;
        (title as HTMLElement).style.cssText = `${was}; white-space: nowrap; width: max-content`;
        const line = title.getBoundingClientRect().width - own;
        (title as HTMLElement).style.cssText = was;
        return { text: (title as HTMLElement).innerText, lines, most: Math.ceil(line / room) + 2 };
      })
    );
  const fit = (shown: Awaited<ReturnType<typeof read>>) =>
    expect(
      shown.map((title) => title.lines <= title.most),
      `lines ${JSON.stringify(shown.map((title) => title.lines))} at most ${JSON.stringify(shown.map((title) => title.most))}`
    ).toEqual([true, true]);
  const texts = [
    "A title with bold, a Doc link, code and https://example.com/a/very/long/path/that/keeps/going/and/going/without/a/space",
    "A folded title with bold, a Doc link and code",
  ];
  const wide = await read();
  expect(wide.map((title) => title.text)).toEqual(texts);
  fit(wide);
  await page.setViewportSize({ width: phoneWidth, height: 800 });
  await afterFrames(page);
  const narrow = await read();
  expect(narrow.map((title) => title.text)).toEqual(texts);
  fit(narrow);
  await expectFits(page, article, article.locator(":scope > .nw-callout").first(), titles.first(), titles.last());

  // In a callout its title 4px above its body, its blocks 0.5rem apart (decision 902); in a list item too, a loose
  // list's paragraphs and a code block under its line, a nested list as it was (decision 903; review B's Q1, Q2).
  const apart = await article.evaluate((view) => {
    const items = [...view.querySelectorAll("li")];
    const loose = items.find((item) => item.textContent.trim().startsWith("A loose item"));
    const tight = items.find((item) => item.textContent.trim().startsWith("Build it"));
    return {
      callout: [...(view.querySelector(":scope > .nw-callout")?.children ?? [])].map(
        (child) => getComputedStyle(child).marginTop
      ),
      loose: [...(loose?.children ?? [])].map((child) => getComputedStyle(child).marginTop),
      tight: [...(tight?.children ?? [])].map((child) => [
        child.tagName.toLowerCase(),
        getComputedStyle(child).marginTop,
      ]),
    };
  });
  expect(apart).toEqual({
    callout: ["0px", "4px", "8px", "8px"],
    loose: ["0px", "8px"],
    tight: [
      ["pre", "8px"],
      ["ul", "4px"],
    ],
  });
});
