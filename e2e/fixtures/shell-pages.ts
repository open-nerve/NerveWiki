import { expect, type Locator, type Page } from "@playwright/test";

// The left column at a phone's width, a drawer (M13/P2/S3): the stories that touch the column there open it first.

/** The left column's root, [data-shell]: the drawer below md. */
export function shell(page: Page): Locator {
  return page.locator("[data-shell]");
}

/** The menu of the page's strip below md, which opens the drawer. */
export function drawerButton(page: Page): Locator {
  return page.getByRole("button", { name: "Navigation", exact: true });
}

/**
 * Opens the drawer of page by its menu; resolves the drawer once it is the dialog named Navigation, slid in: where it
 * is, and what is in view in it, is measured once it is in its place.
 */
export async function openDrawer(page: Page): Promise<Locator> {
  await drawerButton(page).click();
  const drawer = page.getByRole("dialog", { name: "Navigation", exact: true });
  await expect(drawer).toBeVisible();
  await drawer.evaluate((element) => Promise.all(element.getAnimations().map((animation) => animation.finished)));
  return drawer;
}
