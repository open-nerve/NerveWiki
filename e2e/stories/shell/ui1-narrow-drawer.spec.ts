import type { ApiClient, AuthTokens } from "@nervewiki/api-client";
import type { Locator, Page, TestInfo } from "@playwright/test";

import { expectFits, phoneWidth } from "../../fixtures/narrow";
import { goto } from "../../fixtures/navigation";
import { notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { drawerButton, openDrawer, shell } from "../../fixtures/shell-pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, pageTree, wikiPagePath } from "../../fixtures/wiki-pages";
import { searchBox, searchFor, searchResults } from "../../fixtures/wiki-search";
import { switcher, switchWorkspace } from "../../fixtures/workspace-pages";
import { createWorkspace, newOnboardedTeam, slugFor } from "../../fixtures/workspaces";

// UI1, the left column at a phone's width (M13/P2/S3; 00-M13 4.2.5): a drawer, inert and off the screen while
// closed; open, a dialog that holds the focus; closed by Escape, its overlay or a navigation, the focus going to the
// page gone to when it takes it, else to the menu (decisions 781, 782, 790). In the three browsers, the page's version
// only: the cells check the interface's behaviour and write nothing, the API's and MCP's versions exempt (v0.1 design
// 13.4, item 3).

/**
 * Plans, its Roadmap with Q1 under it, and Notes, in a team's workspace, Acme, and the admin's other workspace if
 * other names one; Roadmap shown at a phone's width.
 */
async function roadmapAtPhoneWidth(
  api: ApiClient,
  signedInPage: (tokens: AuthTokens) => Promise<Page>,
  testInfo: TestInfo,
  other?: string
) {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  if (other !== undefined) {
    await createWorkspace(api, pat, other, slugFor(testInfo, other.toLowerCase()));
  }
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const roadmap = await createPage(api, pat, notebook.id, "Roadmap");
  await createPage(api, pat, notebook.id, "Q1", roadmap.id);
  await createPage(api, pat, notebook.id, "Notes");
  const page = await signedInPage(tokens);
  await page.setViewportSize({ width: phoneWidth, height: 800 });
  await goto(page, wikiPagePath(workspace.slug, notebook.id, roadmap.id));
  await expect(pageHeading(page, "Roadmap")).toBeVisible();
  return { page };
}

test("UI1 (page): at 375 wide the left column is a drawer: closed it is inert and off the screen; the menu opens it with the focus in it, which Tab keeps there; Escape, the overlay and a switch of workspace close it, the focus back on the menu", async ({
  api,
  signedInPage,
  browserName,
}, testInfo) => {
  const { page } = await roadmapAtPhoneWidth(api, signedInPage, testInfo, "Beta");
  await expect(shell(page)).toHaveJSProperty("inert", true);
  await expect(shell(page)).toBeHidden();
  await expectFits(page, page.getByRole("main"));

  await openDrawer(page);
  await expect(drawerButton(page)).toHaveAttribute("aria-expanded", "true");
  await expect(switcher(page, "Acme")).toBeFocused();
  // Tab goes round in it: back from its first to its last, the account's button, and on to its first again.
  await page.keyboard.press("Shift+Tab");
  await expect(shell(page).getByRole("button").last()).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(switcher(page, "Acme")).toBeFocused();
  // A focus put outside it, by a program (the overlay keeps the pointer from the page), comes back (review A-M5).
  await page.evaluate(() => document.querySelector<HTMLElement>("main h1")?.focus());
  await expect(switcher(page, "Acme")).toBeFocused();
  await expectFits(page, shell(page));

  await page.keyboard.press("Escape");
  await expect(shell(page)).toHaveJSProperty("inert", true);
  await expect(drawerButton(page)).toBeFocused();

  // Escape in the box holding text: Chromium and Firefox empty it first, the drawer staying; Playwright's WebKit does
  // not, and it closes at once (decision 797).
  await openDrawer(page);
  await searchBox(page).fill("x");
  await page.keyboard.press("Escape");
  if (browserName === "webkit") {
    await expect(drawerButton(page)).toBeFocused();
    await expect(shell(page)).toHaveJSProperty("inert", true);
  } else {
    await expect(searchBox(page)).toHaveValue("");
    await expect(searchBox(page)).toBeFocused();
    await expect(shell(page)).toHaveJSProperty("inert", false);
    await page.keyboard.press("Escape");
    await expect(drawerButton(page)).toBeFocused();
    await expect(shell(page)).toHaveJSProperty("inert", true);
  }

  // The overlay, beside the drawer.
  await openDrawer(page);
  await page.mouse.click(360, 400);
  await expect(drawerButton(page)).toBeFocused();
  await expect(shell(page)).toHaveJSProperty("inert", true);

  // It slides in, unless less motion is asked for (review A-M2, decision 781).
  await openDrawer(page);
  expect(await shell(page).evaluate((element) => getComputedStyle(element).transitionDuration)).toBe("0.15s");
  await page.keyboard.press("Escape");
  await expect(drawerButton(page)).toBeFocused();
  await page.emulateMedia({ reducedMotion: "reduce" });
  await openDrawer(page);
  expect(await shell(page).evaluate((element) => getComputedStyle(element).transitionDuration)).toBe("0s");
  await page.keyboard.press("Escape");
  await expect(drawerButton(page)).toBeFocused();
  await page.emulateMedia({ reducedMotion: null });

  // A switch of workspace in it: closed, the focus on the menu, not given back to the switcher, inert by then.
  await openDrawer(page);
  await switchWorkspace(page, "Acme", "Beta");
  await expect(page.getByRole("heading", { level: 1, name: "Beta", exact: true })).toBeVisible();
  await expect(drawerButton(page)).toBeFocused();
  await expect(shell(page)).toHaveJSProperty("inert", true);
  await page.waitForTimeout(100);
  await expect(drawerButton(page)).toBeFocused();
});

// Control+K is CI's, on Linux; on macOS the story presses Cmd+K (decision 795(d)).
test("UI1 (page): Ctrl+K or Cmd+K opens the drawer at the search box; a search from it closes it, the results' title takes the focus", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { page } = await roadmapAtPhoneWidth(api, signedInPage, testInfo);

  await page.keyboard.press("ControlOrMeta+k");
  await expect(searchBox(page)).toBeFocused();
  await expect(page.getByRole("dialog", { name: "Navigation", exact: true })).toBeVisible();
  await expect(shell(page)).toHaveJSProperty("inert", false);

  await searchFor(page, "Roadmap");
  await expect(page.getByRole("heading", { level: 1, name: "Search “Roadmap”", exact: true })).toBeFocused();
  await expect(shell(page)).toHaveJSProperty("inert", true);
});

test("UI1 (page): a page opened from the drawer's tree closes it, the page's title takes the focus; one in a folder, opened from the search, has its title in view", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { page } = await roadmapAtPhoneWidth(api, signedInPage, testInfo);

  await openDrawer(page);
  await pageTree(page, "Plans").getByRole("link", { name: "Notes", exact: true }).click();
  await expect(pageHeading(page, "Notes")).toBeFocused();
  await expect(shell(page)).toHaveJSProperty("inert", true);

  // The tree opens to Q1 in the drawer, not above the page, pushing its title down (M5-polish 35.7; review C-M11).
  await openDrawer(page);
  await searchFor(page, "Q1");
  await expect(shell(page)).toHaveJSProperty("inert", true);
  await searchResults(page).getByRole("link", { name: "Q1", exact: true }).click();
  await expect(pageHeading(page, "Q1")).toBeFocused();
  await expect(pageHeading(page, "Q1")).toBeInViewport();
});

/**
 * longName is 60 characters without a space, hyphens before digits, as a name taken from a date may have them, where
 * Firefox breaks no line (UAX #14's HY × NU, as PG17's unbroken; review B's P4S1B-F1).
 */
const longName = `Notes-${"2026-11-25-".repeat(5).slice(0, 54)}`;

/** How many lines of its own line height element is high: 1 for a line, 2 or more for a name broken. */
function lines(box: Locator): Promise<number> {
  return box.evaluate(
    (element) => element.getBoundingClientRect().height / parseFloat(getComputedStyle(element).lineHeight)
  );
}

/** Whether box's text is cut short: wider than the box it shows in. */
function cut(box: Locator): Promise<boolean> {
  return box.evaluate((element) => element.scrollWidth > element.clientWidth);
}

// M13/P5/S1 (decisions 916, 917, 919, 931): the cards one abreast at a phone's width, their grid's track written (no
// auto track widened by a name cut short), the name on one line, cut; the notebook's title broken, New page below it on
// the right; the list of pages a row a name, cut, its subpages said by the link's description.
test("UI1 (page): at 375 wide the workspace's home and a notebook's home keep to the screen, a long name cut on its card and in the list of pages, broken in the title", async ({
  api,
  signedInPage,
}, testInfo) => {
  expect(longName).toHaveLength(60);
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const plans = await createNotebook(api, pat, workspace.slug, "Plans");
  await createNotebook(api, pat, workspace.slug, "Wiki", undefined, { mode: "llm_wiki" });
  const long = await createNotebook(api, pat, workspace.slug, longName);
  const longPage = await createPage(api, pat, plans.id, longName);
  await createPage(api, pat, plans.id, "Child", longPage.id);
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");
  await page.setViewportSize({ width: phoneWidth, height: 800 });

  await goto(page, `/${workspace.slug}`);
  const card = main.getByRole("link", { name: longName });
  await expect(card).toBeVisible();
  await expectFits(page, card);
  await expect(main.getByRole("link", { name: "Wiki" })).toContainText("LLM Wiki");
  await expect(card).toContainText("· 1 member");
  const cardName = card.getByText(longName, { exact: true });
  expect(await lines(cardName)).toBeLessThan(1.5);
  expect(await cut(cardName)).toBe(true);

  await page.setViewportSize({ width: 320, height: 800 });
  await expectFits(page, card);

  await page.setViewportSize({ width: phoneWidth, height: 800 });
  await goto(page, notebookPath(workspace.slug, long.id));
  const title = main.getByRole("heading", { level: 1, name: longName, exact: true });
  await expect(title).toBeVisible();
  const newPage = main.getByRole("button", { name: "New page", exact: true });
  expect(await lines(title)).toBeGreaterThanOrEqual(1.5);
  const [titleBox, newPageBox] = [await title.boundingBox(), await newPage.boundingBox()];
  // Below the title, on the right as the title is: the title takes the width the button leaves, the button ms-auto.
  expect(newPageBox?.y ?? 0).toBeGreaterThanOrEqual((titleBox?.y ?? 0) + (titleBox?.height ?? 0));
  expect(
    Math.abs((newPageBox?.x ?? 0) + (newPageBox?.width ?? 0) - (titleBox?.x ?? 0) - (titleBox?.width ?? 0))
  ).toBeLessThan(1);
  await expectFits(page, title, newPage);

  await goto(page, notebookPath(workspace.slug, plans.id));
  const row = main
    .getByRole("list", { name: "Pages", exact: true })
    .getByRole("listitem")
    .filter({ hasText: longName });
  await expect(row).toBeVisible();
  await expectFits(page, row);
  const rowName = row.getByText(longName, { exact: true });
  expect(await lines(rowName)).toBeLessThan(1.5);
  expect(await cut(rowName)).toBe(true);
  // The number of its subpages outside the link, read by no one: the link's description says it.
  await expect(row.getByRole("link")).toHaveAccessibleDescription("1 subpage");
  // A short title keeps New page beside it: under sm the title is as wide as its words (review B's P5B-R1-M1).
  const plansTitle = await main.getByRole("heading", { level: 1, name: "Plans", exact: true }).boundingBox();
  const beside = await newPage.boundingBox();
  expect(beside?.y ?? 0).toBeLessThan((plansTitle?.y ?? 0) + (plansTitle?.height ?? 0));
});
