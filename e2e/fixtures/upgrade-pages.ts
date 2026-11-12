import type { Locator, Page } from "@playwright/test";

// A notebook's upgrade to an LLM Wiki on the page (M11/P1 design 4.4):
// its banner, its start in the settings, and its page, each found by its
// exact name (05-lessons, section 6).

/** What the banner says by the upgrade's state. */
export const bannerText = {
  upgrading: "This notebook is being upgraded to an LLM Wiki: it is read-only for now.",
  reviewing: "This notebook's upgrade is submitted and waits for an admin's review: it is read-only for now.",
  rolling_back: "This notebook's upgrade is being undone: it is read-only for now.",
} as const;

/** The address of the upgrade's page of the notebook id in the workspace of slug. */
export function upgradePath(slug: string, id: string): string {
  return `/${slug}/notebooks/${id}/upgrade`;
}

/** The banner of the notebook's upgrade, a status above its pages, saying text. */
export function upgradeBanner(page: Page, text: string): Locator {
  return page.getByRole("status").filter({ has: page.getByText(text, { exact: true }) });
}

/** The upgrade's page's title. */
export function upgradeTitle(page: Page): Locator {
  return page.getByRole("heading", { level: 1, name: "Upgrade to an LLM Wiki", exact: true });
}

/** The section of the upgrade's page headed name. */
export function upgradeSection(page: Page, name: string): Locator {
  return page.locator("section", { has: page.getByRole("heading", { level: 2, name, exact: true }) });
}

/** The mode section of the notebook's general settings, where its upgrade starts. */
export function modeSection(page: Page): Locator {
  return page.locator("section", { has: page.getByRole("heading", { name: "Mode", exact: true }) });
}

/** The dialog of the start of the upgrade of the notebook named notebook. */
export function startDialog(page: Page, notebook: string): Locator {
  return page.getByRole("alertdialog", { name: `Upgrade “${notebook}” to an LLM Wiki?`, exact: true });
}

/** The counts a list says, each term with its count: what a stopped rollback left. */
export function countsIn(holder: Locator): Promise<string[][]> {
  return holder
    .locator("dt")
    .evaluateAll((terms) => terms.map((term) => [term.textContent ?? "", term.nextElementSibling?.textContent ?? ""]));
}

/**
 * The names of the buttons of the upgrade's page that open a dialog, in their order: its changes, each ending with
 * an ellipsis (M11/P3 design 4.3); not a card's More.
 */
export async function changeButtons(page: Page): Promise<string[]> {
  return (await page.getByRole("main").getByRole("button").allTextContents()).filter((name) => name.endsWith("…"));
}

/** The dialog of the change of the upgrade of the notebook named notebook: its approval, send-back or rejection. */
export function reviewDialog(page: Page, change: "Approve" | "Send back" | "Reject", notebook: string): Locator {
  return page.getByRole("alertdialog", { name: `${change} the upgrade of “${notebook}”?`, exact: true });
}

/**
 * letInAtHome lets in the held stream (letIn) of page, at the notebook notebookId's home, and waits until its
 * connection has read again what the home shows, level by level, its attachments last (the web's refreshedOnConnect):
 * it answers the API requests page sent since, which watch's apiRequests holds.
 */
export async function letInAtHome(
  page: Page,
  watch: { apiRequests: string[] },
  notebookId: string,
  letIn: () => void
): Promise<string[]> {
  const from = watch.apiRequests.length;
  // The connection's own: after its first read, of the workspaces, which nothing else on the home reads again.
  const last = page.waitForRequest(
    (request) =>
      request.method() === "GET" &&
      new URL(request.url()).pathname === `/api/v0/notebooks/${notebookId}/assets` &&
      watch.apiRequests.slice(from).includes("GET /api/v0/workspaces")
  );
  letIn();
  await (await last).response();
  return watch.apiRequests.slice(from);
}
