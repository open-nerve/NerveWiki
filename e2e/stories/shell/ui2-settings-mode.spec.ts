import type { ApiClient, AuthTokens } from "@nervewiki/api-client";
import type { Page, TestInfo } from "@playwright/test";

import { expectNotebook } from "../../fixtures/assert/notebook";
import { expectRenamed } from "../../fixtures/assert/workspace";
import { displayNameOf } from "../../fixtures/auth";
import { accountMenu } from "../../fixtures/auth-pages";
import { countAnswers } from "../../fixtures/browser";
import { holdStream } from "../../fixtures/events";
import { expectFits, phoneWidth } from "../../fixtures/narrow";
import { goto } from "../../fixtures/navigation";
import { notebookHeading, notebookPath, renameNotebookWith, workspaceNav } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { openNotebookSettings, settingsBack, settingsGroup, settingsTitle } from "../../fixtures/settings-pages";
import { drawerButton, openDrawer, shell } from "../../fixtures/shell-pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, pageTree, wikiPagePath } from "../../fixtures/wiki-pages";
import { renameWorkspaceWith, switcher, workspaceHeading } from "../../fixtures/workspace-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// UI2, the settings' mode (M13/P3/S1; 00-M13 4.4.1): their routes turn the left column into their directory, Back
// above it going where they were entered from; each way in and out arrives at the page's title, a link of the same
// shell keeping the focus on itself (decisions 810–818). In the three browsers, the page's version only: the cells
// check the interface's way in and out, names and focus; the two renames make the same database checks as W4's and
// N6's, whose API's and MCP's versions are theirs (v0.1 design 13.4, item 3).

/** A team's workspace, Acme, its notebook Plans, with a page Notes: the admin's page, signed in, and their ids. */
async function plansOfAcme(api: ApiClient, signedInPage: (tokens: AuthTokens) => Promise<Page>, testInfo: TestInfo) {
  const { adminEmail, adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const notes = await createPage(api, pat, notebook.id, "Notes");
  const page = await signedInPage(tokens);
  return { page, adminId, account: displayNameOf(adminEmail), workspace, notebook, notes };
}

test("UI2 (page): the settings are entered from the column's Workspace settings, a notebook's cog and the account's menu, each arrived at; Back goes back where they were entered from", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { page, account, workspace, notebook, notes } = await plansOfAcme(api, signedInPage, testInfo);
  const { slug } = workspace;
  await goto(page, wikiPagePath(slug, notebook.id, notes.id));
  await expect(pageHeading(page, "Notes")).toBeVisible();

  // The cog: the notebook's settings, the column their directory, no search, no switcher, the tree out of sight.
  await openNotebookSettings(page, "Plans");
  await expect(page).toHaveURL(notebookPath(slug, notebook.id, "general"));
  await expect(settingsTitle(page, "General")).toBeFocused();
  await expect(page.getByRole("searchbox")).toHaveCount(0);
  await expect(switcher(page, "Acme")).toHaveCount(0);
  await expect(page.getByRole("navigation", { name: "Pages of Plans", exact: true })).toHaveCount(0);
  await expect(settingsGroup(page, "Settings")).toBeVisible();
  await expect(settingsGroup(page, "Workspace settings")).toContainText("Acme");
  await expect(settingsGroup(page, "Notebook settings")).toContainText("Plans");

  // A page of the same shell: opened, the focus on the link. Followed by its key: WebKit gives a link no focus as the
  // pointer clicks it, as Safari does, and the focus kept would be the body's.
  const members = settingsGroup(page, "Notebook settings").getByRole("link", { name: "Members", exact: true });
  await members.focus();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(notebookPath(slug, notebook.id, "members"));
  await expect(members).toBeFocused();

  // Across to the account's: arrived at, the origin kept.
  await settingsGroup(page, "Settings").getByRole("link", { name: "Profile", exact: true }).click();
  await expect(settingsTitle(page, "Profile")).toBeFocused();
  await settingsBack(page, "Plans").click();
  await expect(page).toHaveURL(wikiPagePath(slug, notebook.id, notes.id));
  await expect(pageHeading(page, "Notes")).toBeFocused();
  await expect(pageTree(page, "Plans")).toBeVisible();

  // The column's Workspace settings, from the workspace's home (M13 closeout, decision 964).
  await workspaceNav(page, "Acme").getByRole("link", { name: "Home", exact: true }).click();
  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await workspaceNav(page, "Acme").getByRole("link", { name: "Workspace settings", exact: true }).click();
  await expect(page).toHaveURL(`/${slug}/settings/general`);
  await expect(settingsTitle(page, "General")).toBeFocused();
  await settingsBack(page, "Acme").click();
  await expect(page).toHaveURL(`/${slug}`);
  await expect(workspaceHeading(page, "Acme")).toBeFocused();

  // The account's menu, from the notebook's home.
  await workspaceNav(page, "Acme").getByRole("link", { name: "Plans", exact: true }).click();
  await expect(notebookHeading(page, "Plans")).toBeVisible();
  await accountMenu(page, account).click();
  await page.getByRole("menuitem", { name: "Settings", exact: true }).click();
  await expect(page).toHaveURL("/settings/profile");
  await expect(settingsTitle(page, "Profile")).toBeFocused();
  await settingsBack(page, "Plans").click();
  await expect(page).toHaveURL(notebookPath(slug, notebook.id));
  await expect(notebookHeading(page, "Plans")).toBeFocused();
});

test("UI2 (page): opened at their address, the directory names its workspace and notebook as renamed, and Back goes to the workspace", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { page, adminId, workspace, notebook } = await plansOfAcme(api, signedInPage, testInfo);
  const { slug } = workspace;
  await goto(page, notebookPath(slug, notebook.id, "general"));
  await expect(settingsTitle(page, "General")).toBeVisible();
  // No origin: Back to the directory's workspace.
  await expect(settingsBack(page, "Acme")).toBeVisible();

  const notebookRenamed = await renameNotebookWith(page, notebook.id, "Plans 2027");
  expect(notebookRenamed.status).toBe(200);
  await expect(settingsGroup(page, "Notebook settings")).toContainText("Plans 2027");
  await expectNotebook(db, notebookRenamed.renamed, adminId);

  const general = settingsGroup(page, "Workspace settings").getByRole("link", { name: "General", exact: true });
  await general.focus();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(`/${slug}/settings/general`);
  await expect(general).toBeFocused();
  const { status, renamed } = await renameWorkspaceWith(page, slug, "Acme Labs");
  expect(status).toBe(200);
  await expect(settingsGroup(page, "Workspace settings")).toContainText("Acme Labs");
  await expectRenamed(db, renamed, adminId);
  await settingsBack(page, "Acme Labs").click();
  await expect(page).toHaveURL(`/${slug}`);
  await expect(workspaceHeading(page, "Acme Labs")).toBeFocused();

  // A new document, no origin: the workspace this device showed last, the list read for it once. The event stream
  // held: connected while that read is out, its refresh would read the list again (holdStream).
  const letStreamIn = await holdStream(page);
  const reads = countAnswers(page, "GET", "/api/v0/workspaces");
  await goto(page, "/settings/profile");
  await expect(settingsTitle(page, "Profile")).toBeVisible();
  await expect(settingsGroup(page, "Workspace settings")).toContainText("Acme Labs");
  await expect(settingsBack(page, "Acme Labs")).toBeVisible();
  expect(reads()).toBe(1);
  letStreamIn();
});

test("UI2 (page): at a phone's width the directory is the drawer, each link in it closing it with the page's title focused", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { page, account, workspace, notebook, notes } = await plansOfAcme(api, signedInPage, testInfo);
  const { slug } = workspace;
  await page.setViewportSize({ width: phoneWidth, height: 800 });
  await goto(page, wikiPagePath(slug, notebook.id, notes.id));
  await expect(pageHeading(page, "Notes")).toBeVisible();

  await openDrawer(page);
  await openNotebookSettings(page, "Plans");
  await expect(shell(page)).toHaveJSProperty("inert", true);
  await expect(settingsTitle(page, "General")).toBeFocused();
  await expect(drawerButton(page).locator("xpath=..")).toHaveText("Settings");

  // Opened, the drawer's first control is Back.
  await openDrawer(page);
  await expect(settingsBack(page, "Plans")).toBeFocused();
  await settingsGroup(page, "Notebook settings").getByRole("link", { name: "Members", exact: true }).click();
  await expect(page).toHaveURL(notebookPath(slug, notebook.id, "members"));
  await expect(shell(page)).toHaveJSProperty("inert", true);
  await expect(settingsTitle(page, "Members")).toBeFocused();

  // Arrived at after an arrival: the page's h1 mounts anew and takes it again (review C's I1).
  await openDrawer(page);
  await settingsGroup(page, "Notebook settings")
    .getByRole("link", { name: "Import, export and restore", exact: true })
    .click();
  await expect(page).toHaveURL(notebookPath(slug, notebook.id, "transfer"));
  await expect(shell(page)).toHaveJSProperty("inert", true);
  await expect(settingsTitle(page, "Import, export and restore")).toBeFocused();
  await expectFits(page);

  await openDrawer(page);
  await settingsBack(page, "Plans").click();
  await expect(page).toHaveURL(wikiPagePath(slug, notebook.id, notes.id));
  await expect(pageHeading(page, "Notes")).toBeFocused();

  // The account's menu in the drawer: Settings arrived at; chosen again on the profile, no arrival, the address the
  // same: the drawer closes, the focus on its menu (decision 816; review A's M2).
  await openDrawer(page);
  await accountMenu(page, account).click();
  await page.getByRole("menuitem", { name: "Settings", exact: true }).click();
  await expect(page).toHaveURL("/settings/profile");
  await expect(shell(page)).toHaveJSProperty("inert", true);
  await expect(settingsTitle(page, "Profile")).toBeFocused();
  await openDrawer(page);
  await accountMenu(page, account).click();
  await page.getByRole("menuitem", { name: "Settings", exact: true }).click();
  await expect(shell(page)).toHaveJSProperty("inert", true);
  await expect(drawerButton(page)).toBeFocused();
  await page.waitForTimeout(100);
  await expect(drawerButton(page)).toBeFocused();
  await expect(page).toHaveURL("/settings/profile");
});
