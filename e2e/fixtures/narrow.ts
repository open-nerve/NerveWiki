import { expect, type Locator, type Page } from "@playwright/test";

/** The width the stories check what is long against: a phone's (WCAG 1.4.10; the stories have taken 375 since M8). */
export const phoneWidth = 375;

/**
 * expectFits checks, at the window's width (phoneWidth, or narrower: 320, M13/P5/S1), that page scrolls not sideways,
 * and that each of boxes keeps to the screen and, one not inline (a paragraph, a heading, a dialog, a filter's
 * wrapper), holds its content: a long name widens one of them first. A dialog is fixed: what it lets out does not
 * widen the page, so it is checked as a box (M12/P2/S3).
 */
export async function expectFits(page: Page, ...boxes: Locator[]): Promise<void> {
  const screen = page.viewportSize()?.width ?? phoneWidth;
  const width = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    content: document.documentElement.scrollWidth,
  }));
  expect(width, "the page's width").toEqual({ viewport: screen, content: screen });
  for (const box of boxes) {
    // oxlint-disable-next-line no-await-in-loop -- a box at a time, named in what fails
    const placed = await box.evaluate((element) => {
      const rect = element.getBoundingClientRect();
      const inline = getComputedStyle(element).display === "inline";
      // An inline box's clientWidth is 0: only its edges say whether it keeps to the screen.
      return { left: rect.left, right: rect.right, overflow: inline ? 0 : element.scrollWidth - element.clientWidth };
    });
    expect(placed.left, `${String(box)}'s left`).toBeGreaterThanOrEqual(0);
    expect(placed.right, `${String(box)}'s right`).toBeLessThanOrEqual(screen);
    expect(placed.overflow, `what overflows ${String(box)}`).toBeLessThanOrEqual(0);
  }
}
