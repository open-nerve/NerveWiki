import userEvent from "@testing-library/user-event";
import { afterEach, expect, test } from "vitest";

import type { ReadingContext } from "./enhancement";
import { pageLinks } from "./page-links";

// The links to pages (M6/P3 design 6.7).

afterEach(() => document.body.replaceChildren());

/** browser does what the browser would after the view, which jsdom does not: it handles a click. */
const browser = (event: Event) => event.preventDefault();

/** setUp puts a view with links to pages, one to none, and another link in the page, and a context that records where it goes. */
function setUp() {
  const container = document.createElement("article");
  container.innerHTML =
    '<p><a class="nw-wikilink" data-nw-node="a1">A</a> <a data-nw-node="b2" data-nw-anchor="nw-part-two">B</a> ' +
    '<a class="nw-wikilink nw-unresolved" data-nw-target="C">C</a> <a href="https://x.example/">x</a> ' +
    '<a href="#nw-fn:1">1</a> <a class="nw-wikilink" data-nw-node="a1"><em>emphasis</em></a></p>';
  document.body.append(container);
  const went: string[] = [];
  const context: ReadingContext = {
    workspace: "lab",
    notebook: "n",
    page: "p",
    revision: 1,
    role: "reader",
    reload: () => undefined,
    navigate: (to) => went.push(to),
    report: () => undefined,
  };
  const link = (name: string) => {
    const found = [...container.querySelectorAll("a")].find((a) => a.textContent === name);
    if (found === undefined) {
      throw new Error(`no link ${name}`);
    }
    return found;
  };
  return { container, context, went, link };
}

test("a link to a page has the page's address in the app, with its anchor's", () => {
  const { container, context, link } = setUp();
  pageLinks(container, context);
  expect(link("A").getAttribute("href")).toBe("/lab/notebooks/n/pages/a1");
  expect(link("B").getAttribute("href")).toBe("/lab/notebooks/n/pages/b2#nw-part-two");
  expect(link("C").hasAttribute("href")).toBe(false);
  expect(link("x").getAttribute("href")).toBe("https://x.example/");
});

test("a plain click goes through the router", async () => {
  const { container, context, went, link } = setUp();
  pageLinks(container, context);

  await userEvent.click(link("A"));
  await userEvent.click(link("B"));
  await userEvent.click(link("emphasis").querySelector("em") ?? link("emphasis"));
  const click = new MouseEvent("click", { bubbles: true, cancelable: true });
  link("A").dispatchEvent(click);
  expect(click.defaultPrevented).toBe(true);

  expect(went).toEqual([
    "/lab/notebooks/n/pages/a1",
    "/lab/notebooks/n/pages/b2#nw-part-two",
    "/lab/notebooks/n/pages/a1",
    "/lab/notebooks/n/pages/a1",
  ]);
});

test("a click with a modifier, of another button, or that another handled, and one on another link, is the browser's", () => {
  const { container, context, went, link } = setUp();
  pageLinks(container, context);
  document.addEventListener("click", browser);
  const clicks: [HTMLElement, MouseEventInit][] = [
    [link("A"), { ctrlKey: true }],
    [link("A"), { metaKey: true }],
    [link("A"), { shiftKey: true }],
    [link("A"), { altKey: true }],
    [link("A"), { button: 1 }],
    [link("C"), {}],
    [link("x"), {}],
    [link("1"), {}],
  ];
  for (const [target, init] of clicks) {
    target.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, ...init }));
  }
  link("A").addEventListener("click", (event) => event.preventDefault(), { once: true });
  link("A").dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));
  document.removeEventListener("click", browser);

  expect(went).toEqual([]);
});

test("undone, the links have no address and a click goes nowhere", async () => {
  const { container, context, went, link } = setUp();
  const undo = pageLinks(container, context);
  undo?.();

  expect(link("A").hasAttribute("href")).toBe(false);
  expect(link("B").hasAttribute("href")).toBe(false);
  expect(link("x").getAttribute("href")).toBe("https://x.example/");
  await userEvent.click(link("A"));
  expect(went).toEqual([]);
});

test("a view without links to pages has nothing to undo", () => {
  const container = document.createElement("article");
  container.innerHTML = '<p><a href="https://x.example/">x</a></p>';
  const { context } = setUp();
  expect(pageLinks(container, context)).toBeUndefined();
});
