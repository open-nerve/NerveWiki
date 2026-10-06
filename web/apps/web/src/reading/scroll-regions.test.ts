import { afterEach, expect, test, vi } from "vitest";

import { translator } from "../i18n/i18n";
import type { ReadingContext } from "./enhancement";
import { scrollRegions } from "./scroll-regions";

// What scrolls sideways (M6/P6 design 6). jsdom lays nothing out: the
// widths are set by hand, and the resize observer is the test's, which
// says when a box changed.

afterEach(() => document.body.replaceChildren());

/** Sets how wide element's content is and how wide it shows. */
function widths(element: Element | null | undefined, content: number, shown: number) {
  Object.defineProperty(element, "scrollWidth", { configurable: true, value: content });
  Object.defineProperty(element, "clientWidth", { configurable: true, value: shown });
}

function observed() {
  const observers: { follow: (entries: { target: Element }[]) => void; targets: Element[]; disconnected: boolean }[] =
    [];
  vi.stubGlobal(
    "ResizeObserver",
    class {
      entry: (typeof observers)[number];
      constructor(follow: (entries: { target: Element }[]) => void) {
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

const context: ReadingContext = {
  workspace: "lab",
  notebook: "n",
  page: "p",
  revision: 1,
  role: "reader",
  t: translator("en"),
  theme: "light",
  reload: () => {},
  navigate: () => {},
  report: () => {},
  unresolved: () => {},
};

/** mutated lets the mutation observers' records come. */
const mutated = () => new Promise((resolve) => setTimeout(resolve, 0));

/** state is how element takes the focus: its tabindex, role and name, or none. */
function state(element: Element | null | undefined): string {
  if (!element?.hasAttribute("tabindex")) {
    return [element?.getAttribute("role"), element?.getAttribute("aria-label")].some((each) => each !== null)
      ? "half"
      : "none";
  }
  return `${element.getAttribute("tabindex")} ${element.getAttribute("role")} ${element.getAttribute("aria-label")}`;
}

/** view is an article of each wide thing the server writes, and the article's own text. */
function view() {
  const article = document.createElement("article");
  article.innerHTML =
    '<div class="nw-scroll"><table class="nw-props"><tbody><tr><th>a</th><td>1</td></tr></tbody></table></div>' +
    '<div class="nw-scroll"><table><tbody><tr><td>wide</td></tr></tbody></table></div>' +
    '<div class="nw-scroll"><div class="nw-math nw-math-block">x</div></div>' +
    '<p>a <span class="nw-math nw-math-block">y</span></p>' +
    '<pre><code>short</code></pre><div class="nw-scroll"><p>other</p></div>';
  document.body.append(article);
  const [props, table, math, other] = article.querySelectorAll<HTMLElement>(".nw-scroll");
  return {
    article,
    props,
    table,
    math,
    inline: article.querySelector<HTMLElement>("span.nw-math-block"),
    code: article.querySelector("pre"),
    other,
  };
}

test("what is wider than it shows takes the focus as a region named for what it holds, and only it", () => {
  const observers = observed();
  const { article, props, table, math, inline, code, other } = view();
  for (const each of [props, table, math, inline, code, other]) {
    widths(each, 900, 600);
  }
  widths(article, 600, 600);

  scrollRegions(article, context);

  expect([props, table, math, inline, code, other].map(state)).toEqual([
    "0 region Properties",
    "0 region Table",
    "0 region Formula",
    "0 region Formula",
    "0 region Code",
    "0 region Wide content",
  ]);
  expect(state(article)).toBe("none");
  expect(observers[0]?.targets).toEqual([props, table, math, inline, code, other]);
});

test("one no wider than it shows is no region, and becomes one as the window narrows, and none as it widens", () => {
  const observers = observed();
  const { article, table, code } = view();
  widths(table, 300, 600);
  // As wide as it shows: nothing to scroll.
  widths(code, 600, 600);

  scrollRegions(article, context);
  expect([state(table), state(code)]).toEqual(["none", "none"]);

  widths(table, 900, 600);
  observers[0]?.follow([{ target: table as Element }]);
  expect([state(table), state(code)]).toEqual(["0 region Table", "none"]);

  widths(table, 900, 1000);
  observers[0]?.follow([{ target: table as Element }]);
  expect(state(table)).toBe("none");
});

test("a content that an enhancement renders wider, and a wrapper it adds, are followed as they come", async () => {
  observed();
  const { article, math } = view();
  scrollRegions(article, context);
  expect(state(math)).toBe("none");

  // The formula typeset: its box stays, its content grows.
  widths(math, 900, 600);
  math?.firstElementChild?.replaceChildren(document.createElement("span"));
  const diagram = Object.assign(document.createElement("div"), { className: "nw-scroll nw-diagram" });
  widths(diagram, 1200, 600);
  article.append(diagram);
  await mutated();

  expect([state(math), state(diagram)]).toEqual(["0 region Formula", "0 region Diagram"]);
});

test("undone, nothing takes the focus, and nothing is followed", async () => {
  const observers = observed();
  const { article, table, code } = view();
  widths(table, 900, 600);
  widths(code, 900, 600);
  const undo = scrollRegions(article, context);

  undo?.();
  expect([state(table), state(code)]).toEqual(["none", "none"]);
  expect(observers[0]?.disconnected).toBe(true);
  const late = Object.assign(document.createElement("pre"), { textContent: "late" });
  widths(late, 900, 600);
  article.append(late);
  await mutated();
  expect(state(late)).toBe("none");
});
