import userEvent from "@testing-library/user-event";
import { afterEach, expect, test } from "vitest";

import type { ReadingContext, UnresolvedLink } from "./enhancement";
import { unresolvedLinks } from "./unresolved-links";
import { translator } from "../i18n/i18n";

// The links to pages that are not there (M6/P6 design 7).

afterEach(() => document.body.replaceChildren());

/**
 * setUp puts a view with links to pages not there, a property's, an embed's and an image's among them, and other
 * links, and a context that records what it is handed.
 */
function setUp() {
  const container = document.createElement("article");
  container.innerHTML =
    '<p><a class="nw-wikilink nw-unresolved" data-nw-target="Plans/x">x</a> ' +
    '<a class="nw-unresolved" data-nw-target="y.md"><em>y</em></a> ' +
    '<a class="nw-wikilink nw-embed nw-unresolved" data-nw-target="E">E</a> ' +
    '<span class="nw-image">pic <a class="nw-unresolved" data-nw-target="p.png">p.png</a></span> ' +
    '<a class="nw-wikilink" data-nw-node="a1">A</a> <a href="https://x.example/">away</a></p>' +
    '<table class="nw-props"><tbody><tr><th>up</th><td><a class="nw-wikilink nw-unresolved" data-nw-target="Up">Up</a>' +
    "</td></tr></tbody></table>";
  document.body.append(container);
  const handed: [string, UnresolvedLink["kind"], string][] = [];
  const context: ReadingContext = {
    workspace: "lab",
    notebook: "n",
    page: "p",
    revision: 1,
    role: "editor",
    t: translator("en"),
    theme: () => "light",
    onThemeChange: () => () => undefined,
    reload: () => undefined,
    navigate: () => undefined,
    report: () => undefined,
    unresolved: ({ target, kind, element }) => handed.push([target, kind, element.textContent ?? ""]),
  };
  const link = (name: string) => {
    const found = [...container.querySelectorAll("a")].find((a) => a.textContent === name);
    if (found === undefined) {
      throw new Error(`no link ${name}`);
    }
    return found;
  };
  return { container, context, handed, link };
}

test("a link to a page not there is a focusable button that opens a dialog; the others are as they were", () => {
  const { container, context, link } = setUp();
  unresolvedLinks(container, context);
  for (const name of ["x", "y", "E", "p.png", "Up"]) {
    expect(link(name).getAttribute("role"), name).toBe("button");
    expect(link(name).getAttribute("tabindex"), name).toBe("0");
    expect(link(name).getAttribute("aria-haspopup"), name).toBe("dialog");
  }
  for (const name of ["A", "away"]) {
    expect(link(name).hasAttribute("role"), name).toBe(false);
    expect(link(name).hasAttribute("tabindex"), name).toBe(false);
  }
});

test("a click, Enter or Space hands the link to the view with its target and what it is", async () => {
  const { container, context, handed, link } = setUp();
  unresolvedLinks(container, context);

  await userEvent.click(link("x"));
  await userEvent.click(link("y").querySelector("em") ?? link("y"));
  await userEvent.click(link("E"));
  await userEvent.click(link("p.png"));
  link("Up").focus();
  await userEvent.keyboard("{Enter}");
  // Space acts as it comes up, as on a button; down, it would scroll the page.
  const down = new KeyboardEvent("keydown", { key: " ", bubbles: true, cancelable: true });
  link("x").dispatchEvent(down);
  expect(handed).toHaveLength(5);
  link("x").dispatchEvent(new KeyboardEvent("keyup", { key: " ", bubbles: true, cancelable: true }));

  expect(handed).toEqual([
    ["Plans/x", "link", "x"],
    ["y.md", "link", "y"],
    ["E", "embed", "E"],
    ["p.png", "image", "p.png"],
    ["Up", "link", "Up"],
    ["Plans/x", "link", "x"],
  ]);
  expect(down.defaultPrevented).toBe(true);
});

test("another key, a key with a modifier or while composing, another button, and what another handled hand nothing", () => {
  const { container, context, handed, link } = setUp();
  unresolvedLinks(container, context);
  const keys: KeyboardEventInit[] = [
    { key: "a" },
    { key: "Tab" },
    { key: "Enter", ctrlKey: true },
    { key: "Enter", metaKey: true },
    { key: " ", shiftKey: true },
    { key: "Enter", altKey: true },
    { key: "Enter", isComposing: true },
  ];
  for (const init of keys) {
    for (const type of ["keydown", "keyup"]) {
      link("x").dispatchEvent(new KeyboardEvent(type, { bubbles: true, cancelable: true, ...init }));
    }
  }
  // Enter acts as it goes down only.
  link("x").dispatchEvent(new KeyboardEvent("keyup", { key: "Enter", bubbles: true, cancelable: true }));
  const elsewhere = new KeyboardEvent("keydown", { key: " ", bubbles: true, cancelable: true });
  link("A").dispatchEvent(elsewhere);
  expect(elsewhere.defaultPrevented).toBe(false);
  link("x").dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, button: 1 }));
  link("x").addEventListener("click", (event) => event.preventDefault(), { once: true });
  link("x").dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));
  link("A").dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));
  link("A").dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true, cancelable: true }));

  expect(handed).toEqual([]);
});

test("undone, the links are as the server wrote them and hand nothing", async () => {
  const { container, context, handed, link } = setUp();
  const before = container.innerHTML;
  unresolvedLinks(container, context)?.();

  expect(container.innerHTML).toBe(before);
  await userEvent.click(link("x"));
  link("x").dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true, cancelable: true }));
  expect(handed).toEqual([]);
});

test("a diagram's link is mermaid's, not the server's: it is left be", async () => {
  const { container, context, handed } = setUp();
  const diagram = Object.assign(document.createElement("div"), { className: "nw-scroll nw-diagram" });
  diagram.innerHTML = '<svg><a class="nw-unresolved" data-nw-target="Secret/x"><text>make</text></a></svg>';
  container.append(diagram);
  unresolvedLinks(container, context);

  const made = diagram.querySelector("a");
  expect(made?.hasAttribute("role")).toBe(false);
  made?.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));
  expect(handed).toEqual([]);
});

test("a view without links to pages not there has nothing to undo", () => {
  const container = document.createElement("article");
  container.innerHTML = '<p><a class="nw-wikilink" data-nw-node="a1">A</a></p>';
  const { context } = setUp();
  expect(unresolvedLinks(container, context)).toBeUndefined();
});
