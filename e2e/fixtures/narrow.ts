import { expect, type Locator, type Page } from "@playwright/test";

/** The width the stories check what is long against: a phone's (WCAG 1.4.10; the stories have taken 375 since M8). */
export const phoneWidth = 375;

/**
 * expectFits checks, at the window's width (phoneWidth, or narrower: 320, M13/P5/S1; a wider one is a caller that
 * forgot to set it, and fails, P5S1A-N4), that page scrolls not sideways, and that each of boxes keeps to the screen
 * and, one not inline (a paragraph, a heading, a dialog, a filter's wrapper), holds its content: a long name widens
 * one of them first. A dialog is fixed: what it lets out does not widen the page, so it is checked as a box
 * (M12/P2/S3).
 */
export async function expectFits(page: Page, ...boxes: Locator[]): Promise<void> {
  const screen = page.viewportSize()?.width ?? phoneWidth;
  expect(screen, "a phone's width or narrower").toBeLessThanOrEqual(phoneWidth);
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

/**
 * expectButtonsInside checks, by their boxes and not by a font (05-lessons 6), that each button of box keeps inside
 * its border and padding: a row of them to the right that cannot wrap sticks out of its left at a narrow width
 * (M13/P5/S3, decision 958; review B's P5S3B-I1). A dialog pops in scaled (styles.css's pop-in, 0.97 to 1 in 150 ms)
 * while its border and padding read unscaled, which put a button up to 0.75 px past the padding: what runs on box or
 * above it, and ends, is let end first (as openDrawer does), then the edges are compared as they are (review C's
 * P5S3C-F1-I1; not an engine's rounding, as said before).
 */
export async function expectButtonsInside(box: Locator): Promise<void> {
  await box.evaluate((element) => {
    const running: Animation[] = [];
    for (let at: Element | null = element; at !== null; at = at.parentElement) {
      running.push(...at.getAnimations().filter((animation) => animation.effect?.getTiming().iterations !== Infinity));
    }
    // A dialog closed meanwhile cancels its animation: finished then rejects, and there is nothing to wait for.
    return Promise.all(running.map((animation) => animation.finished.catch(() => undefined)));
  });
  const placed = await box.evaluate((element) => {
    const style = getComputedStyle(element);
    const rect = element.getBoundingClientRect();
    return {
      left: rect.left + parseFloat(style.borderLeftWidth) + parseFloat(style.paddingLeft),
      right: rect.right - parseFloat(style.borderRightWidth) - parseFloat(style.paddingRight),
      buttons: [...element.querySelectorAll("button")].map((button) => {
        const { left, right } = button.getBoundingClientRect();
        return { name: button.textContent, left, right };
      }),
    };
  });
  expect(placed.buttons.length, `the buttons of ${String(box)}`).toBeGreaterThan(0);
  for (const button of placed.buttons) {
    expect(button.left, `${button.name}'s left, inside ${String(box)}`).toBeGreaterThanOrEqual(placed.left);
    expect(button.right, `${button.name}'s right, inside ${String(box)}`).toBeLessThanOrEqual(placed.right);
  }
}
