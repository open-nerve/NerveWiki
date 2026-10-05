import userEvent from "@testing-library/user-event";
import { afterEach, expect, test } from "vitest";

import { appLinks, inApp } from "./app-links";
import type { ReadingContext } from "./enhancement";

// The links into the app (M6/P3 design 6.7, M6/P6 design 8, 9).

afterEach(() => document.body.replaceChildren());

/** browser does what the browser would after the view, which jsdom does not: it handles a click. */
const browser = (event: Event) => event.preventDefault();

const site = window.location.origin;

/**
 * setUp puts a view with links to pages, one to none, tags, a property's link, full addresses of this site and of
 * another, and a footnote's, and a context that records where it goes.
 */
function setUp() {
  const container = document.createElement("article");
  container.innerHTML =
    '<div class="nw-scroll"><table class="nw-props"><tbody><tr><th>up</th>' +
    '<td><a class="nw-wikilink" data-nw-node="c3">Up</a></td></tr></tbody></table></div>' +
    '<p><a class="nw-wikilink" data-nw-node="a1">A</a> <a data-nw-node="b2" data-nw-anchor="nw-part-two">B</a> ' +
    '<a class="nw-wikilink nw-unresolved" data-nw-target="C">C</a> <a href="https://x.example/">x</a> ' +
    '<a href="#nw-fn:1">1</a> <a class="nw-wikilink" data-nw-node="a1"><em>emphasis</em></a> ' +
    '<a class="nw-tag" data-nw-tag="Proj">#Proj</a> <a class="nw-tag" data-nw-tag="a/b">#a/b</a> ' +
    '<a class="nw-tag" data-nw-tag="a/">#a//</a> <a class="nw-tag" data-nw-tag="中文">#中文</a> ' +
    `<a href="${site}/lab/notebooks/n/pages/d4?x=1#nw-h">here</a> <a href="${site}/api/v0/instance">api</a></p>`;
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
    unresolved: () => undefined,
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

test("a link to a page, a property's too, has the page's address in the app, with its anchor's", () => {
  const { container, context, link } = setUp();
  appLinks(container, context);
  expect(link("A").getAttribute("href")).toBe("/lab/notebooks/n/pages/a1");
  expect(link("B").getAttribute("href")).toBe("/lab/notebooks/n/pages/b2#nw-part-two");
  expect(link("Up").getAttribute("href")).toBe("/lab/notebooks/n/pages/c3");
  expect(link("C").hasAttribute("href")).toBe(false);
  expect(link("x").getAttribute("href")).toBe("https://x.example/");
});

test("a tag has the address of its pages, its name one segment, a nested tag's and a last '/' too", () => {
  const { container, context, link } = setUp();
  appLinks(container, context);
  expect(link("#Proj").getAttribute("href")).toBe("/lab/notebooks/n/tags/Proj");
  expect(link("#a/b").getAttribute("href")).toBe("/lab/notebooks/n/tags/a%2Fb");
  expect(link("#a//").getAttribute("href")).toBe("/lab/notebooks/n/tags/a%2F");
  expect(link("#中文").getAttribute("href")).toBe("/lab/notebooks/n/tags/%E4%B8%AD%E6%96%87");
});

test("a plain click goes through the router, to a page, a tag, or the app's page a full address of this site names", async () => {
  const { container, context, went, link } = setUp();
  appLinks(container, context);

  await userEvent.click(link("A"));
  await userEvent.click(link("B"));
  await userEvent.click(link("emphasis").querySelector("em") ?? link("emphasis"));
  await userEvent.click(link("Up"));
  await userEvent.click(link("#a/b"));
  await userEvent.click(link("here"));
  const click = new MouseEvent("click", { bubbles: true, cancelable: true });
  link("A").dispatchEvent(click);
  expect(click.defaultPrevented).toBe(true);

  expect(went).toEqual([
    "/lab/notebooks/n/pages/a1",
    "/lab/notebooks/n/pages/b2#nw-part-two",
    "/lab/notebooks/n/pages/a1",
    "/lab/notebooks/n/pages/c3",
    "/lab/notebooks/n/tags/a%2Fb",
    "/lab/notebooks/n/pages/d4?x=1#nw-h",
    "/lab/notebooks/n/pages/a1",
  ]);
});

test("a click with a modifier, of another button, or that another handled, and one on another link, is the browser's", () => {
  const { container, context, went, link } = setUp();
  appLinks(container, context);
  document.addEventListener("click", browser);
  const clicks: [HTMLElement, MouseEventInit][] = [
    [link("A"), { ctrlKey: true }],
    [link("A"), { metaKey: true }],
    [link("A"), { shiftKey: true }],
    [link("A"), { altKey: true }],
    [link("A"), { button: 1 }],
    [link("#Proj"), { metaKey: true }],
    [link("here"), { ctrlKey: true }],
    [link("C"), {}],
    [link("x"), {}],
    [link("1"), {}],
    [link("api"), {}],
  ];
  for (const [target, init] of clicks) {
    target.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, ...init }));
  }
  link("A").addEventListener("click", (event) => event.preventDefault(), { once: true });
  link("A").dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));
  document.removeEventListener("click", browser);

  expect(went).toEqual([]);
});

test("a link a diagram adds later goes through the router too, by its href or its xlink:href", async () => {
  const { container, context, went } = setUp();
  appLinks(container, context);
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  const plain = document.createElementNS("http://www.w3.org/2000/svg", "a");
  plain.setAttribute("href", `${site}/lab/notebooks/n/pages/e5`);
  plain.append(document.createElementNS("http://www.w3.org/2000/svg", "text"));
  const old = document.createElementNS("http://www.w3.org/2000/svg", "a");
  old.setAttributeNS("http://www.w3.org/1999/xlink", "xlink:href", `${site}/lab/notebooks/n/tags/t`);
  const away = document.createElementNS("http://www.w3.org/2000/svg", "a");
  away.setAttribute("href", "https://x.example/");
  svg.append(plain, old, away);
  container.append(svg);
  document.addEventListener("click", browser);

  for (const target of [plain.firstElementChild ?? plain, old, away]) {
    target.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));
  }
  document.removeEventListener("click", browser);

  expect(went).toEqual(["/lab/notebooks/n/pages/e5", "/lab/notebooks/n/tags/t"]);
});

test("undone, the links have no address and a click goes nowhere", async () => {
  const { container, context, went, link } = setUp();
  const undo = appLinks(container, context);
  undo?.();

  expect(link("A").hasAttribute("href")).toBe(false);
  expect(link("B").hasAttribute("href")).toBe(false);
  expect(link("#Proj").hasAttribute("href")).toBe(false);
  expect(link("x").getAttribute("href")).toBe("https://x.example/");
  expect(link("here").getAttribute("href")).toBe(`${site}/lab/notebooks/n/pages/d4?x=1#nw-h`);
  document.addEventListener("click", browser);
  await userEvent.click(link("A"));
  await userEvent.click(link("here"));
  // An address the link has again, another's: its click is not the view's.
  link("A").setAttribute("href", "/elsewhere");
  await userEvent.click(link("A"));
  document.removeEventListener("click", browser);
  expect(went).toEqual([]);
});

test("a full address is the app's when it is of this site and names a page of the app", () => {
  const app = "https://wiki.example";
  expect(inApp(`${app}/acme/notebooks/n/pages/p`, app)).toBe("/acme/notebooks/n/pages/p");
  expect(inApp(`${app}/acme/notebooks/n/pages/p?q=1#nw-h`, app)).toBe("/acme/notebooks/n/pages/p?q=1#nw-h");
  expect(inApp(`HTTPS://WIKI.example/acme`, app)).toBe("/acme");
  expect(inApp(app, app)).toBe("/");
  expect(inApp(`${app}/apis/x`, app)).toBe("/apis/x");
  expect(inApp(`${app}/healthzz`, app)).toBe("/healthzz");
  expect(inApp(`${app}/acme/../settings`, app)).toBe("/settings");
  for (const other of [
    `${app}/api`,
    `${app}/api/v0/instance`,
    `${app}/healthz`,
    `${app}/readyz`,
    `${app}/assets`,
    `${app}/assets/index-abc.js`,
    `${app}/favicon.svg`,
    `${app}/acme/notes.md`,
    "http://wiki.example/acme",
    "https://wiki.example:8443/acme",
    "https://x.example/acme",
    "mailto:a@wiki.example",
    "#nw-h",
    "/acme",
    "https://",
    null,
  ]) {
    expect(inApp(other, app), String(other)).toBeUndefined();
  }
});
