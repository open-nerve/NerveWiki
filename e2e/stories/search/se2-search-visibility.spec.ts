import type { ApiClient } from "@nervewiki/api-client";

import { bearer, emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { search } from "../../fixtures/search";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { resultNames, searchBox, searchFor, searchStatus } from "../../fixtures/wiki-search";
import { newTeam } from "../../fixtures/workspaces";

// SE2, what a search sees (M8 design 4.12, section 9; M8/P5 design 4.6):
// the notebooks a search looks through are those the account sees. A
// member finds the pages of the notebooks open to the workspace, not those
// of a private one; a guest those of the notebook it was added to alone.
// notebook_id narrows a search to one notebook; one the account does not
// see is 404, which the results page says, with Search the whole
// workspace. No assertion of the database: a search writes nothing.

/** The word every notebook's page holds. */
const word = "zephyr";

/** The id of the account of credential. */
async function idOf(api: ApiClient, credential: string): Promise<string> {
  const { data, response } = await api.GET("/api/v0/me", { headers: bearer(credential) });
  expect(response.status).toBe(200);
  return data?.id ?? "";
}

/**
 * The notebooks of SE2, which adminPat creates in the workspace of slug, each with a page holding the word: Open,
 * which the workspace's members edit; Shared, which they read; Private, its admin's alone; and Guests, private but
 * for the guest guestId, added as its reader.
 */
async function notebooksOf(api: ApiClient, adminPat: string, slug: string, guestId: string) {
  const open = await createNotebook(api, adminPat, slug, "Open", "editor");
  await createPage(api, adminPat, open.id, "Open plan", null, `The ${word} project.\n`);
  const shared = await createNotebook(api, adminPat, slug, "Shared", "viewer");
  await createPage(api, adminPat, shared.id, "Shared plan", null, `The ${word} project.\n`);
  const closed = await createNotebook(api, adminPat, slug, "Private", "none");
  await createPage(api, adminPat, closed.id, "Private plan", null, `The ${word} project.\n`);
  const guests = await createNotebook(api, adminPat, slug, "Guests", "none");
  const guestPage = await createPage(api, adminPat, guests.id, "Guest plan", null, `The ${word} project.\n`);
  await addedNotebookMember(api, adminPat, guests.id, guestId, "reader");
  return { open, shared, closed, guests, guestPage };
}

/** The names of what credential's search of the word finds in the workspace of slug, narrowed to notebookId. */
async function namesFound(api: ApiClient, credential: string, slug: string, notebookId?: string): Promise<string[]> {
  const { data, error, response } = await search(api, credential, slug, word, { notebookId });
  expect(response.status, JSON.stringify(error)).toBe(200);
  return data?.data.map((hit) => hit.node.name) ?? [];
}

test("SE2 (API): a member finds the pages of the notebooks open to the workspace, not a private one's; a guest the one's it was added to alone; notebook_id narrows to one, and one not seen is 404", async ({
  api,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const memberPat = await joinAs(api, pat, workspace.slug, emailFor(testInfo, "member"), "member");
  const guestPat = await joinAs(api, pat, workspace.slug, emailFor(testInfo, "guest"), "guest");
  const { open, closed, guests } = await notebooksOf(api, pat, workspace.slug, await idOf(api, guestPat));

  // The admin made them all, each its admin: newest first.
  expect(await namesFound(api, pat, workspace.slug)).toEqual([
    "Guest plan",
    "Private plan",
    "Shared plan",
    "Open plan",
  ]);
  expect(await namesFound(api, memberPat, workspace.slug)).toEqual(["Shared plan", "Open plan"]);
  expect(await namesFound(api, guestPat, workspace.slug)).toEqual(["Guest plan"]);

  expect(await namesFound(api, memberPat, workspace.slug, open.id)).toEqual(["Open plan"]);
  expect(await namesFound(api, guestPat, workspace.slug, guests.id)).toEqual(["Guest plan"]);
  for (const [credential, notebookId] of [
    [memberPat, closed.id],
    [memberPat, guests.id],
    [guestPat, open.id],
  ] as const) {
    // oxlint-disable-next-line no-await-in-loop -- one refusal at a time
    const { error, response } = await search(api, credential, workspace.slug, word, { notebookId });
    expect(response.status).toBe(404);
    expect(error?.code).toBe("notebook.not_found");
  }
});

test("SE2 (page): a member's search from a notebook looks in it, as the box's hint says; All notebooks finds the others it sees, not a private one's; a notebook it does not see is not found, with Search the whole workspace; a guest's finds the one it was added to alone", async ({
  anotherPage,
  api,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const tokens = await joinOnboarded(api, pat, workspace.slug, emailFor(testInfo, "member"), "member");
  const guestTokens = await joinOnboarded(api, pat, workspace.slug, emailFor(testInfo, "guest"), "guest");
  const { open, closed, guests, guestPage } = await notebooksOf(
    api,
    pat,
    workspace.slug,
    await idOf(api, guestTokens.access_token)
  );
  const openPage = (await search(api, pat, workspace.slug, word, { notebookId: open.id })).data?.data[0]?.node.id ?? "";
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");

  await page.goto(wikiPagePath(workspace.slug, open.id, openPage));
  await expect(pageHeading(page, "Open plan")).toBeVisible();
  await expect(searchBox(page)).toHaveAccessibleDescription("Searches in “Open”.");
  expect((await searchFor(page, word)).status()).toBe(200);
  await expect(page).toHaveURL(`/${workspace.slug}/search?q=${word}&notebook_id=${open.id}`);
  await expect(searchStatus(page)).toHaveText("Results: 1.");
  expect(await resultNames(page)).toEqual(["Open plan"]);
  const scope = main.getByRole("combobox", { name: "Where to look", exact: true });
  await expect(scope).toHaveValue(open.id);
  await expect(scope.locator("option")).toHaveText(["All notebooks", "Open", "Shared"]);

  // Chosen as a reader of the page would: the select has the focus (selectOption alone does not give it).
  await scope.focus();
  await scope.selectOption({ label: "All notebooks" });
  await expect(page).toHaveURL(`/${workspace.slug}/search?q=${word}`);
  await expect(searchStatus(page)).toHaveText("Results: 2.");
  expect(await resultNames(page)).toEqual(["Shared plan", "Open plan"]);
  await expect(scope).toBeFocused();
  await expect(searchBox(page)).toHaveAccessibleDescription("Searches the whole workspace.");

  // The private notebook, by its id in the address.
  pageWatch.expectConsole({ errors: [failedToLoad(404)] });
  await page.goto(`/${workspace.slug}/search?q=${word}&notebook_id=${closed.id}`);
  await expect(main.getByRole("alert")).toHaveText(
    "This notebook is not found: it does not exist, or you have no access to it."
  );
  await main.getByRole("link", { name: "Search the whole workspace", exact: true }).click();
  await expect(page).toHaveURL(`/${workspace.slug}/search?q=${word}`);
  await expect(searchStatus(page)).toHaveText("Results: 2.");
  expect(await resultNames(page)).toEqual(["Shared plan", "Open plan"]);
  await expect(main.getByRole("heading", { level: 1, name: `Search “${word}”`, exact: true })).toBeFocused();

  // The guest, from the page of the notebook it was added to: it finds that one's alone, which is all Where to look
  // lists, in it and in all it sees (M8 closeout B-M2, C-M5).
  const guestTab = await anotherPage(guestTokens);
  await guestTab.goto(wikiPagePath(workspace.slug, guests.id, guestPage.id));
  await expect(pageHeading(guestTab, "Guest plan")).toBeVisible();
  await expect(searchBox(guestTab)).toHaveAccessibleDescription("Searches in “Guests”.");
  expect((await searchFor(guestTab, word)).status()).toBe(200);
  await expect(guestTab).toHaveURL(`/${workspace.slug}/search?q=${word}&notebook_id=${guests.id}`);
  await expect(searchStatus(guestTab)).toHaveText("Results: 1.");
  expect(await resultNames(guestTab)).toEqual(["Guest plan"]);
  const guestScope = guestTab.getByRole("main").getByRole("combobox", { name: "Where to look", exact: true });
  await expect(guestScope.locator("option")).toHaveText(["All notebooks", "Guests"]);
  await guestScope.focus();
  await guestScope.selectOption({ label: "All notebooks" });
  await expect(guestTab).toHaveURL(`/${workspace.slug}/search?q=${word}`);
  await expect(searchStatus(guestTab)).toHaveText("Results: 1.");
  expect(await resultNames(guestTab)).toEqual(["Guest plan"]);
});
