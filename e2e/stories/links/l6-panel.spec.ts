import { countAnswers } from "../../fixtures/browser";
import { holdStream } from "../../fixtures/events";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// L6, the page's right column (M6 design 9; M6/P7 design 7–11): beside the
// page, its outline goes to a heading, one in a folded callout too, which
// opens for it, a property link to its page; the backlinks list the pages
// that link here, with their lines, and one another writes shows as it is
// written (the links event).

test("L6 (page): the right column's outline goes to a heading, one in a folded callout too, a property link to its page, whose backlinks show a link another writes as it is written", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const hub = await createPage(api, pat, notebook.id, "Hub");
  const other = await createPage(api, pat, notebook.id, "Other");
  const content = [
    "---",
    'up: "[[Hub]]"',
    'see: "[[Nowhere]]"',
    "---",
    "# Intro",
    "",
    "## Setup",
    "",
    "filler\n\n".repeat(80),
    "## Usage",
    "",
    "> [!note]- Later",
    "> ## Hidden",
    "",
  ].join("\n");
  const doc = await createPage(api, pat, notebook.id, "Doc", null, content);
  const hubPath = wikiPagePath(workspace.slug, notebook.id, hub.id);
  const page = await signedInPage(tokens);
  const letStreamIn = await holdStream(page);
  const backlinkReads = countAnswers(page, "GET", `/api/v0/pages/${hub.id}/backlinks`);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, doc.id));
  const panel = page.getByRole("complementary", { name: "About this page", exact: true });

  // Beside the page's content.
  const article = page.getByRole("article", { name: "Doc" });
  await expect(article).toBeVisible();
  const [view, column] = [await article.boundingBox(), await panel.boundingBox()];
  expect(column?.x).toBeGreaterThan((view?.x ?? 0) + (view?.width ?? 0));

  const outline = panel.getByRole("navigation", { name: "Outline", exact: true });
  await expect(outline.getByRole("link")).toHaveText(["Intro", "Setup", "Usage", "Hidden"]);
  await outline.getByRole("link", { name: "Usage", exact: true }).click();
  const usage = article.getByRole("heading", { level: 2, name: "Usage", exact: true });
  await expect(usage).toBeFocused();
  await expect(usage).toBeInViewport();
  await expect(page).toHaveURL(/#nw-usage$/);
  await outline.getByRole("link", { name: "Hidden", exact: true }).click();
  const hidden = article.getByRole("heading", { level: 2, name: "Hidden", exact: true });
  await expect(hidden).toBeFocused();
  await expect(hidden).toBeInViewport();
  // Held at the top as the window scrolls.
  await expect(panel).toBeInViewport();

  await expect(panel.getByText("Nowhere", { exact: true })).toBeVisible();
  await expect(panel.getByRole("link", { name: "Nowhere" })).toHaveCount(0);
  await panel.getByRole("link", { name: "Hub", exact: true }).click();
  await expect(pageHeading(page, "Hub")).toBeFocused();
  await expect(page).toHaveURL(hubPath);

  // Hub's backlinks: Doc, by its property's line.
  const backlinks = panel.locator("details", { has: page.getByText("Backlinks", { exact: true }) });
  await expect(backlinks.getByRole("link")).toHaveText(["Doc"]);
  await expect(backlinks.getByText('up: "[[Hub]]"', { exact: true })).toBeVisible();
  // The stream connects, and its refresh reads them again: from then on, what is written comes as events.
  letStreamIn();
  await expect.poll(backlinkReads).toBeGreaterThanOrEqual(2);
  await page.evaluate(() => Object.assign(window, { notReloaded: true }));

  await writeContent(api, pat, other.id, { content: "Meet at [[Hub]] today\n", base_revision: 1 });
  // By id: Other was made first.
  await expect(backlinks.getByRole("link")).toHaveText(["Other", "Doc"]);
  await expect(backlinks.getByText("Meet at [[Hub]] today", { exact: true })).toBeVisible();
  expect(await page.evaluate(() => "notReloaded" in window)).toBe(true);
});

test("L6 (page, keyboard): the last More adds the next page of backlinks and takes the focus to the first it adds, in view, without a scroll", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const hub = await createPage(api, pat, notebook.id, "Hub");
  // One more than a page of backlinks (50), made in order: the list is by id.
  for (let n = 1; n <= 51; n++) {
    // oxlint-disable-next-line no-await-in-loop -- one after another, in order
    await createPage(api, pat, notebook.id, `P${String(n).padStart(2, "0")}`, null, "See [[Hub]]\n");
  }
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, hub.id));
  const panel = page.getByRole("complementary", { name: "About this page", exact: true });
  const backlinks = panel.locator("details", { has: page.getByText("Backlinks", { exact: true }) });
  await expect(backlinks.getByRole("link")).toHaveCount(50);

  const more = backlinks.getByRole("button", { name: "More backlinks", exact: true });
  await more.focus();
  // The window's scroll, and the column's own (at xl).
  const scrolls = () => Promise.all([page.evaluate(() => window.scrollY), panel.evaluate((aside) => aside.scrollTop)]);
  const scrolled = await scrolls();
  await page.keyboard.press("Enter");
  const added = backlinks.getByRole("link", { name: "P51", exact: true });
  await expect(added).toBeFocused();
  await expect(more).toHaveCount(0);
  await expect(added).toBeInViewport();
  expect(await scrolls()).toEqual(scrolled);
});

test("L6 (page, large): properties of many strings at long paths show at once; a page of 140,000 headings opens with its outline", async ({
  api,
  signedInPage,
}, testInfo) => {
  test.setTimeout(180_000);
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  // A key past 16,383 characters, a browser's map's square for many strings at paths that long, each over two lines;
  // as many as the server takes for a valid frontmatter, about.
  const strings = await createPage(
    api,
    pat,
    notebook.id,
    "Strings",
    null,
    `---\n? ${"k".repeat(16_400)}\n: [${Array.from({ length: 9_800 }, () => '"a\n b"').join(", ")}]\n---\nbody\n`
  );
  const headings = await createPage(api, pat, notebook.id, "Headings", null, "# a\n".repeat(140_000));
  const page = await signedInPage(tokens);
  const panel = page.getByRole("complementary", { name: "About this page", exact: true });

  const started = Date.now();
  await page.goto(wikiPagePath(workspace.slug, notebook.id, strings.id));
  await expect(panel.getByRole("heading", { level: 2, name: "Properties", exact: true })).toBeVisible();
  await expect(panel.locator("dl dt")).toHaveCount(1);
  expect(Date.now() - started).toBeLessThan(4_000);

  // More headings than a call takes arguments.
  await page.goto(wikiPagePath(workspace.slug, notebook.id, headings.id));
  const outline = panel.getByRole("navigation", { name: "Outline", exact: true });
  const failed = page.getByText("Something went wrong");
  await expect(outline.or(failed)).toBeVisible({ timeout: 120_000 });
  await expect(failed).toHaveCount(0);
  await expect(outline).toBeVisible();
});
