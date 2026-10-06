import { act, screen, within } from "@testing-library/react";
import { expect, vi } from "vitest";

// The page's right column, as its tests find it (M6/P7 design 7–10).

/** scrolls gives elements a way to scroll, which jsdom has not, and is what was scrolled into view. */
export function scrolls(): Element[] {
  const scrolled: Element[] = [];
  Element.prototype.scrollIntoView = function (this: Element) {
    scrolled.push(this);
  };
  return scrolled;
}

/** panel is the page's right column. */
export function panel(): HTMLElement {
  return screen.getByRole("complementary", { name: "About this page" });
}

/** shownPanel waits for the page's right column. */
export function shownPanel(): Promise<HTMLElement> {
  return screen.findByRole("complementary", { name: "About this page" });
}

/** section is the right column's section titled title. */
export function section(title: string): HTMLElement {
  const heading = within(panel()).getByRole("heading", { level: 2, name: title });
  const details = heading.closest("details");
  expect(details).not.toBeNull();
  return details as HTMLElement;
}

/** readAgain has SWR read what is shown again, as a refocus does once its interval is over. */
export async function readAgain() {
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));
}
