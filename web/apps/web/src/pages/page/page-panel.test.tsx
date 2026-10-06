import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { json, problem } from "../../test/fakes";
import { pageEditor } from "../../test/page-editor";
import { guide, install, linux, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The page's right column (M6/P7 design 7–10).

afterEach(() => {
  vi.useRealTimers();
  Reflect.deleteProperty(Element.prototype, "scrollIntoView");
});

/** scrolls gives elements a way to scroll, which jsdom has not, and is what was scrolled into view. */
function scrolls(): Element[] {
  const scrolled: Element[] = [];
  Element.prototype.scrollIntoView = function (this: Element) {
    scrolled.push(this);
  };
  return scrolled;
}

/** panel is the page's right column. */
function panel(): HTMLElement {
  return screen.getByRole("complementary", { name: "About this page" });
}

/** shownPanel waits for the page's right column. */
function shownPanel(): Promise<HTMLElement> {
  return screen.findByRole("complementary", { name: "About this page" });
}

/** section is the right column's section titled title. */
function section(title: string): HTMLElement {
  const summary = within(panel()).getByText(title, { selector: "summary" });
  const details = summary.closest("details");
  expect(details).not.toBeNull();
  return details as HTMLElement;
}

/** backlinked is what the backlinks list: for each page, its link's text, how many links, and its lines. */
function backlinked(): (string | null)[][] {
  return [...section("Backlinks").querySelectorAll(":scope > ul > li")].map((item) =>
    [...item.querySelectorAll("a, span, li")].map((each) => each.textContent)
  );
}

/** readAgain has SWR read what is shown again, as a refocus does once its interval is over. */
async function readAgain() {
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));
}

/** outlined is what the outline lists: each heading's text, and how far it is indented. */
function outlined(): string[][] {
  const outline = within(panel()).getByRole("navigation", { name: "Outline" });
  return within(outline)
    .getAllByRole("listitem")
    .map((item) => [item.textContent ?? "", item.style.paddingLeft]);
}

test("the outline lists the page's headings with an id, by their text, indented by their level from the page's highest", async () => {
  const server = pageServer();
  server.views.set(install.id, {
    html: [
      '<h2 id="nw-intro">Intro</h2><p>Text</p>',
      '<h3 id="nw-set-up">Set <em>up</em></h3>',
      '<h4 id="nw-energy">Energy <span class="nw-math">E=mc^2</span></h4>',
      '<h2 id="nw-section"> </h2><h2>No id</h2>',
      '<h3 id="nw-intro-1">Intro</h3>',
    ].join(""),
    revision: 1,
  });
  renderApp(pagePath(install.id), server.app);

  await screen.findByRole("navigation", { name: "Outline" });
  expect(outlined()).toEqual([
    ["Intro", "0rem"],
    ["Set up", "0.75rem"],
    ["Energy E=mc^2", "1.5rem"],
    ["Intro", "0.75rem"],
  ]);
});

test("a heading of the outline goes to its heading through the router, which shows and takes the focus, each time", async () => {
  const scrolled = scrolls();
  const user = userEvent.setup();
  const server = pageServer();
  server.views.set(install.id, { html: '<p>intro</p><h2 id="nw-安装">安装</h2><h2 id="nw-b">B</h2>', revision: 1 });
  const { router } = renderApp(pagePath(install.id), server.app);
  const outline = await screen.findByRole("navigation", { name: "Outline" });

  await user.click(within(outline).getByRole("link", { name: "安装" }));
  const heading = within(screen.getByRole("article")).getByRole("heading", { level: 2, name: "安装" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(router.state.location.pathname).toBe(pagePath(install.id));
  expect(router.state.location.hash).toBe(`#${encodeURIComponent("nw-安装")}`);
  expect(scrolled).toEqual([heading]);

  await user.click(within(outline).getByRole("link", { name: "B" }));
  await user.click(within(outline).getByRole("link", { name: "安装" }));
  await waitFor(() => expect(scrolled).toHaveLength(3));
  expect(document.activeElement).toBe(heading);
});

test("the outline follows the view read again, with no read of its own; a page without headings has none", async () => {
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1 });
  const reloads: (() => void)[] = [];
  renderApp(pagePath(install.id), server.app, {
    enhancements: [
      (_container, context) => {
        reloads.push(context.reload);
        return undefined;
      },
    ],
  });
  await screen.findByRole("navigation", { name: "Outline" });
  expect(outlined()).toEqual([["A", "0rem"]]);

  server.views.set(install.id, { html: '<h1 id="nw-a">A</h1><h3 id="nw-c">C</h3>', revision: 2 });
  act(() => reloads.at(-1)?.());
  await waitFor(() =>
    expect(outlined()).toEqual([
      ["A", "0rem"],
      ["C", "1.5rem"],
    ])
  );

  server.views.set(install.id, { html: "<p>No headings</p>", revision: 3 });
  act(() => reloads.at(-1)?.());
  await waitFor(() => expect(screen.getByRole("article").textContent).toBe("No headings"));
  expect(within(panel()).queryByRole("navigation")).toBeNull();
  expect(server.sent.filter((sent) => sent.startsWith("GET view"))).toEqual([
    "GET view Install",
    "GET view Install",
    "GET view Install",
  ]);
});

test("the right column comes after the page's content and its subpages", async () => {
  renderApp(pagePath(guide.id), pageServer().app);

  const subpages = await screen.findByRole("list", { name: "Subpages" });
  const article = screen.getByRole("article");
  expect(article.compareDocumentPosition(panel()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(subpages.compareDocumentPosition(panel()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});

test("while the page is edited the outline is not shown; back to reading, it is", async () => {
  const server = pageServer({ role: "editor" });
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1 });
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("navigation", { name: "Outline" });

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await pageEditor();
  expect(within(panel()).queryByRole("navigation", { name: "Outline" })).toBeNull();

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await screen.findByRole("button", { name: "Edit" });
  expect(await within(await shownPanel()).findByRole("navigation", { name: "Outline" })).toBeTruthy();
});

test("the backlinks are the pages that link here, by their title, with how many links when more than one and their lines; each goes to its page", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  server.backlinks.set(install.id, [
    {
      data: [
        { id: guide.id, count: 1, contexts: ["See [[Install]]"] },
        { id: notes.id, count: 3, contexts: ["a  [[Install]]", "b [[Install|it]]…"] },
        { id: linux.id, count: 1000, contexts: [] },
      ],
      next_cursor: null,
    },
  ]);
  const { router } = renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  expect(backlinked()).toEqual([
    ["Guide", "See [[Install]]"],
    ["Notes", " · 3 links", "a  [[Install]]", "b [[Install|it]]…"],
    ["Linux", " · 1000+ links"],
  ]);

  await user.click(within(section("Backlinks")).getByRole("link", { name: "Notes" }));
  const heading = await screen.findByRole("heading", { level: 1, name: "Notes" });
  expect(router.state.location.pathname).toBe(pagePath(notes.id));
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

test("a page that links here the tree does not have yet shows once the tree is read again; with none, the backlinks say so", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const draft = pageNode(9, "Draft");
  const server = pageServer();
  server.backlinks.set(guide.id, [{ data: [{ id: draft.id, count: 1, contexts: ["[[Guide]]"] }], next_cursor: null }]);
  renderApp(pagePath(guide.id), server.app);
  await waitFor(() => expect(server.sent).toContain(`GET backlinks ${guide.id}`));
  expect(await within(section("Backlinks")).findByText("No page links here.")).toBeTruthy();

  server.nodes = [...server.nodes, draft];
  await readAgain();
  expect(await within(section("Backlinks")).findByRole("link", { name: "Draft" })).toBeTruthy();
});

test("more reads the next page of the backlinks and adds it, until the last; read again, the list starts from its first", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = pageServer();
  server.backlinks.set(install.id, [
    { data: [{ id: guide.id, count: 1, contexts: [] }], next_cursor: "1" },
    { data: [{ id: notes.id, count: 1, contexts: [] }], next_cursor: null },
  ]);
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("link", { name: "Guide" });

  await user.click(within(section("Backlinks")).getByRole("button", { name: "More" }));
  await within(section("Backlinks")).findByRole("link", { name: "Notes" });
  expect(backlinked()).toEqual([["Guide"], ["Notes"]]);
  expect(within(section("Backlinks")).queryByRole("button", { name: "More" })).toBeNull();
  expect(server.sent).toContain(`GET backlinks ${install.id} 1`);

  server.backlinks.set(install.id, [
    { data: [{ id: guide.id, count: 2, contexts: [] }], next_cursor: "1" },
    { data: [{ id: notes.id, count: 1, contexts: [] }], next_cursor: null },
  ]);
  await readAgain();
  await waitFor(() => expect(backlinked()).toEqual([["Guide", " · 2 links"]]));
  expect(within(section("Backlinks")).getByRole("button", { name: "More" })).toBeTruthy();
});

test("a next page read as the list is read again, which changed, is not added; one that fails says so, and more reads it again", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  let first = { data: [{ id: guide.id, count: 1, contexts: [] as string[] }], next_cursor: "1" };
  let next: () => Promise<Response> = failing;
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/backlinks": (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        server.sent.push(`GET backlinks ${cursor ?? ""}`.trim());
        return cursor === null ? json(first) : next();
      },
    },
  });
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  const more = within(section("Backlinks")).getByRole("button", { name: "More" });

  await user.click(more);
  expect(await within(section("Backlinks")).findByRole("alert")).toBeTruthy();

  let answer!: (response: Response) => void;
  next = () => new Promise((resolve) => (answer = resolve));
  await user.click(more);
  await waitFor(() => expect(server.sent.filter((each) => each === "GET backlinks 1")).toHaveLength(2));
  expect(within(section("Backlinks")).queryByRole("alert")).toBeNull();
  expect(more.getAttribute("aria-busy")).toBe("true");
  first = { data: [{ id: guide.id, count: 2, contexts: [] }], next_cursor: "1" };
  await readAgain();
  await waitFor(() => expect(backlinked()).toEqual([["Guide", " · 2 links"]]));
  answer(json({ data: [{ id: notes.id, count: 1, contexts: [] }], next_cursor: null }));
  await waitFor(() => expect(more.getAttribute("aria-busy")).toBeNull());
  expect(backlinked()).toEqual([["Guide", " · 2 links"]]);
});

/** failing answers a read that failed. */
function failing(): Promise<Response> {
  return Promise.resolve(problem(500, "internal"));
}

/** properties is what the properties show: each key, and its value's text. */
function properties(): string[][] {
  return [...section("Properties").querySelectorAll("dl > div")].map((row) => [
    row.querySelector("dt")?.textContent ?? "",
    row.querySelector("dd")?.textContent ?? "",
  ]);
}

test("the properties show each key and its value; a property link its text, leading to its page, or to none, styled so", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    properties: [
      { key: "status", value: "draft" },
      { key: "count", value: 3 },
      { key: "done", value: false },
      { key: "empty", value: null },
      { key: "meta", value: { a: 1 } },
      { key: "up", value: "[[Guide| the guide ]]" },
      {
        key: "related",
        value: ["[[ Notes ]]", "[Linux *x*](Linux)", "[[Gone#Part]]", "[[Guide # Intro]]", "[[#Top]]", ["x"]],
      },
    ],
    links: [
      { key: "up", node_id: guide.id },
      { key: "related.0", node_id: notes.id },
      { key: "related.1", node_id: linux.id },
      { key: "related.2", node_id: null },
      { key: "related.3", node_id: guide.id },
    ],
  });
  const { router } = renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "the guide" });
  expect(properties()).toEqual([
    ["status", "draft"],
    ["count", "3"],
    ["done", "false"],
    ["empty", ""],
    ["meta", '{"a":1}'],
    ["up", "the guide"],
    ["related", 'NotesLinux *x*Gone > PartGuide > Intro[[#Top]]["x"]'],
  ]);
  const links = within(section("Properties")).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href")])).toEqual([
    ["the guide", pagePath(guide.id)],
    ["Notes", pagePath(notes.id)],
    ["Linux *x*", pagePath(linux.id)],
    ["Guide > Intro", pagePath(guide.id)],
  ]);
  expect(within(section("Properties")).getByText("Gone > Part").className).toContain("decoration-dashed");

  await user.click(links[1] as HTMLElement);
  const heading = await screen.findByRole("heading", { level: 1, name: "Notes" });
  expect(router.state.location.pathname).toBe(pagePath(notes.id));
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

test("a frontmatter that is not valid says so, as one without properties does", async () => {
  const server = pageServer();
  server.properties.set(install.id, { valid: false, properties: [], links: [] });
  renderApp(pagePath(install.id), server.app);
  expect(
    await within(await shownPanel()).findByText("The page's frontmatter is not valid: it has no properties.")
  ).toBeTruthy();

  renderApp(pagePath(guide.id), pageServer().app);
  expect(await screen.findByText("No properties.")).toBeTruthy();
});

test("while the page is edited its backlinks and properties are shown", async () => {
  const server = pageServer({ role: "editor" });
  server.backlinks.set(install.id, [{ data: [{ id: guide.id, count: 1, contexts: [] }], next_cursor: null }]);
  server.properties.set(install.id, { valid: true, properties: [{ key: "status", value: "draft" }], links: [] });
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("link", { name: "Guide" });

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await pageEditor();
  expect(within(section("Backlinks")).getByRole("link", { name: "Guide" })).toBeTruthy();
  expect(properties()).toEqual([["status", "draft"]]);
});
