import type { ApiClient, AuthTokens } from "@nervewiki/api-client";
import type { Page, TestInfo } from "@playwright/test";

import { expectFits, phoneWidth } from "../../fixtures/narrow";
import { goto } from "../../fixtures/navigation";
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
