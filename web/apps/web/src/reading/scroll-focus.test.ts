import { expect, test, vi } from "vitest";

import { scrollFocus } from "./scroll-focus";

// jsdom lays nothing out: the widths are set by hand, and the observer is
// the test's, which says when a width changed.

/** Sets how wide element's content is and how wide it shows. */
function widths(element: HTMLElement, content: number, shown: number) {
  Object.defineProperty(element, "scrollWidth", { configurable: true, value: content });
  Object.defineProperty(element, "clientWidth", { configurable: true, value: shown });
}

function observed() {
  const observers: { follow: () => void; targets: Element[]; disconnected: boolean }[] = [];
  vi.stubGlobal(
    "ResizeObserver",
    class {
      entry: (typeof observers)[number];
      constructor(follow: () => void) {
        this.entry = { follow, targets: [], disconnected: false };
        observers.push(this.entry);
      }
      observe(target: Element) {
        this.entry.targets.push(target);
      }
      disconnect() {
        this.entry.disconnected = true;
      }
    }
  );
  return observers;
}

const context = { workspace: "lab", notebook: "n", page: "p", revision: 1, role: "reader" as const, reload: () => {} };

test("the view and each code block take the focus while wider than they show, and only then", () => {
  const observers = observed();
  const article = document.createElement("article");
  article.innerHTML = "<table><tr><td>wide</td></tr></table><pre><code>short</code></pre><pre><code>long</code></pre>";
  const [short, long] = article.querySelectorAll("pre");
  widths(article, 900, 600);
  widths(short as HTMLElement, 300, 600);
  widths(long as HTMLElement, 1200, 600);

  const undo = scrollFocus(article, context);

  expect([article.tabIndex, short?.hasAttribute("tabindex"), long?.tabIndex]).toEqual([0, false, 0]);
  expect(observers[0]?.targets).toEqual([article, short, long]);

  // The window grows: nothing is wider than it shows.
  widths(article, 900, 1000);
  widths(long as HTMLElement, 1200, 1300);
  observers[0]?.follow();
  expect([article.hasAttribute("tabindex"), long?.hasAttribute("tabindex")]).toEqual([false, false]);

  widths(article, 900, 600);
  observers[0]?.follow();
  undo?.();
  expect(article.hasAttribute("tabindex")).toBe(false);
  expect(observers[0]?.disconnected).toBe(true);
});
