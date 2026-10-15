import type { Locator, Page, Response } from "@playwright/test";

// A workspace's search as a user works it (M8/P5 design 4.2, 4.3, 4.6):
// the box at the top of the left column, and the results page.
// fixtures/search.ts is the API's.

/** The search box's field, at the top of the workspace's left column. */
export function searchBox(page: Page): Locator {
  return page.getByRole("search").getByRole("searchbox", { name: "Search terms", exact: true });
}

/** Searches q with the search box, Enter submitting it; resolves the search's answer. */
export async function searchFor(page: Page, q: string): Promise<Response> {
  const answer = page.waitForResponse(
    (response) =>
      response.request().method() === "GET" &&
      /^\/api\/v0\/workspaces\/[^/]+\/search$/.test(new URL(response.url()).pathname)
  );
  await searchBox(page).fill(q);
  await searchBox(page).press("Enter");
  return answer;
}

/** The hits the results page on page lists, in order. */
export function searchResults(page: Page): Locator {
  return page.getByRole("main").getByRole("list", { name: "Search results", exact: true }).getByRole("listitem");
}

/** The names of the hits the results page lists, in order: their links' text, a link a hit. */
export function resultNames(page: Page): Promise<string[]> {
  return searchResults(page).getByRole("link").allTextContents();
}

/** The results page's status line: the output its heading is described by, once it says what was found. */
export function searchStatus(page: Page): Locator {
  return page.getByRole("main").locator("xpath=.//output[@id = //h1/@aria-describedby]");
}
