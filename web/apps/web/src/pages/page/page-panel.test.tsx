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
  const heading = within(panel()).getByRole("heading", { level: 2, name: title });
  const details = heading.closest("details");
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

test("the outline lists the page's headings with an id, but the footnotes', by their text without a footnote's number or an image's address, indented by their level from the page's highest", async () => {
  const server = pageServer();
  server.views.set(install.id, {
    html: [
      '<h2 id="nw-intro">Intro</h2><p>Text</p>',
      '<h3 id="nw-set-up">Set <em>up</em></h3>',
      '<h4 id="nw-energy">Energy <span class="nw-math">E=mc^2</span></h4>',
      '<h2 id="nw-section"> </h2><h2>No id</h2>',
      '<h3 id="nw-intro-1">Intro</h3>',
      '<h2 id="nw-notes">Notes<sup id="nw-fnref:1"><a href="#nw-fn:1" class="footnote-ref">1</a></sup></h2>',
      '<h3 id="nw-has-image">Has <span class="nw-image">alt <a href="https://x.test/i.png">https://x.test/i.png</a></span> image</h3>',
      '<div class="footnotes"><ol><li id="nw-fn:1"><h4 id="nw-in-a-note">In a note</h4></li></ol></div>',
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
    ["Notes", "0rem"],
    ["Has alt image", "0.75rem"],
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

test("the outline follows the view read again, its reads the view's; a page without headings has none", async () => {
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

/** backlinkPage is a page of backlinks: a page that links here count times, then the cursor of the next. */
function backlinkPage(id: string, count: number, next: string | null) {
  return { data: [{ id, count, contexts: [] }], next_cursor: next };
}

test("more reads the next page and adds it, the focus kept on it until the last, which takes it to the first page it adds; read again, or come back to, the list is as many pages, from the first", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const draft = pageNode(9, "Draft");
  const server = pageServer({ nodes: [guide, install, linux, notes, draft] });
  const last = {
    data: [
      { id: linux.id, count: 1, contexts: [] },
      { id: draft.id, count: 1, contexts: [] },
    ],
    next_cursor: null,
  };
  server.backlinks.set(install.id, [backlinkPage(guide.id, 1, "1"), backlinkPage(notes.id, 1, "2"), last]);
  const { router } = renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  const more = within(section("Backlinks")).getByRole("button", { name: "More backlinks" });

  more.focus();
  let before = server.sent.length;
  await user.keyboard("{Enter}");
  await within(section("Backlinks")).findByRole("link", { name: "Notes" });
  await act(() => vi.advanceTimersByTimeAsync(0));
  expect(document.activeElement).toBe(more);
  expect(server.sent.slice(before).filter((each) => each.startsWith("GET backlinks"))).toEqual([
    `GET backlinks ${install.id} 1`,
  ]);

  await user.keyboard("{Enter}");
  const added = await within(section("Backlinks")).findByRole("link", { name: "Linux" });
  expect(backlinked()).toEqual([["Guide"], ["Notes"], ["Linux"], ["Draft"]]);
  expect(within(section("Backlinks")).queryByRole("button", { name: "More backlinks" })).toBeNull();
  await waitFor(() => expect(document.activeElement).toBe(added));

  // Come back to, the page shows the pages read, and reads as many.
  await act(() => router.navigate(pagePath(notes.id)));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  // Past SWR's deduping of the reads it starts.
  await act(() => vi.advanceTimersByTimeAsync(3_000));
  before = server.sent.length;
  await act(() => router.navigate(pagePath(install.id)));
  await within(await shownPanel()).findByRole("link", { name: "Draft" });
  await waitFor(() =>
    expect(server.sent.slice(before).filter((each) => each.startsWith(`GET backlinks ${install.id}`))).toEqual([
      `GET backlinks ${install.id}`,
      `GET backlinks ${install.id} 1`,
      `GET backlinks ${install.id} 2`,
    ])
  );
  expect(backlinked()).toEqual([["Guide"], ["Notes"], ["Linux"], ["Draft"]]);

  server.backlinks.set(install.id, [backlinkPage(guide.id, 2, "1"), backlinkPage(notes.id, 3, "2"), last]);
  before = server.sent.length;
  await readAgain();
  await waitFor(() =>
    expect(backlinked()).toEqual([["Guide", " · 2 links"], ["Notes", " · 3 links"], ["Linux"], ["Draft"]])
  );
  expect(server.sent.slice(before).filter((each) => each.startsWith("GET backlinks"))).toEqual([
    `GET backlinks ${install.id}`,
    `GET backlinks ${install.id} 1`,
    `GET backlinks ${install.id} 2`,
  ]);
});

/**
 * heldMore is a server whose backlinks are Guide's, then Notes' on a last
 * page, which the first time is held until answered; reads are the
 * cursors of the backlinks read, null the first page's.
 */
function heldMore() {
  const held = { reads: [] as (string | null)[], answer: undefined as (() => void) | undefined };
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/backlinks": (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        held.reads.push(cursor);
        if (cursor === null) {
          return json(backlinkPage(guide.id, 1, "1"));
        }
        const last = backlinkPage(notes.id, 1, null);
        return held.answer === undefined
          ? new Promise((resolve) => (held.answer = () => resolve(json(last))))
          : json(last);
      },
    },
  });
  return { server, held };
}

test("the last more leaves the focus where the reader put it meanwhile: on another link, on the content clicked", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  for (const elsewhere of [
    (guideLink: HTMLElement) => act(() => guideLink.focus()),
    () => user.click(screen.getByRole("article")),
  ]) {
    const { server, held } = heldMore();
    const { unmount } = renderApp(pagePath(install.id), server.app);
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    const guideLink = await within(await shownPanel()).findByRole("link", { name: "Guide" });

    // oxlint-disable-next-line no-await-in-loop -- one app after another
    await user.click(within(section("Backlinks")).getByRole("button", { name: "More backlinks" }));
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    await elsewhere(guideLink);
    const put = document.activeElement;
    held.answer?.();
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    await within(section("Backlinks")).findByRole("link", { name: "Notes" });
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(document.activeElement).toBe(put);
    unmount();
  }
});

test("a more answered after the page was left and come back to stays in the list, which reads it again with it", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const { server, held } = heldMore();
  const { router } = renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  await user.click(within(section("Backlinks")).getByRole("button", { name: "More backlinks" }));

  await act(() => router.navigate(pagePath(notes.id)));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await act(() => router.navigate(pagePath(install.id)));
  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  held.answer?.();
  await within(section("Backlinks")).findByRole("link", { name: "Notes" });

  held.reads.length = 0;
  await readAgain();
  await waitFor(() => expect(held.reads).toEqual([null, "1"]));
  expect(backlinked()).toEqual([["Guide"], ["Notes"]]);
});

test("a last more that adds no page shown takes the focus to the section's title", async () => {
  const user = userEvent.setup();
  const draft = pageNode(9, "Draft");
  // A list whose pages the tree has none of yet.
  const lone = pageServer({
    answers: {
      "GET /api/v0/pages/*/backlinks": (request) =>
        json(
          new URL(request.url).searchParams.get("cursor") === null
            ? backlinkPage(draft.id, 1, "1")
            : backlinkPage(draft.id, 1, null)
        ),
    },
  });
  renderApp(pagePath(guide.id), lone.app);
  const button = await within(await shownPanel()).findByRole("button", { name: "More backlinks" });
  button.focus();
  await user.keyboard("{Enter}");
  const title = within(panel()).getByRole("heading", { level: 2, name: "Backlinks" }).closest("summary");
  await waitFor(() => expect(document.activeElement).toBe(title));
});

test("a next page that fails says so, and more reads it again; one added while the list is read again has the list read again whole", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  let first = backlinkPage(guide.id, 1, "1");
  let held = Promise.resolve();
  let next: () => Promise<Response> = failing;
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/backlinks": async (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        server.sent.push(`GET backlinks ${cursor ?? ""}`.trim());
        if (cursor !== null) {
          return next();
        }
        await held;
        return json(first);
      },
    },
  });
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  const more = within(section("Backlinks")).getByRole("button", { name: "More backlinks" });

  await user.click(more);
  expect(await within(section("Backlinks")).findByRole("alert")).toBeTruthy();

  let answer!: (response: Response) => void;
  next = () => new Promise((resolve) => (answer = resolve));
  await user.click(more);
  await waitFor(() => expect(server.sent.filter((each) => each === "GET backlinks 1")).toHaveLength(2));
  expect(within(section("Backlinks")).queryByRole("alert")).toBeNull();
  expect(more.getAttribute("aria-busy")).toBe("true");

  // The list is read again, its first page's answer held, as the next page comes: SWR drops that read.
  let release!: () => void;
  held = new Promise((resolve) => (release = resolve));
  first = backlinkPage(guide.id, 3, "1");
  await readAgain();
  await waitFor(() => expect(server.sent.filter((each) => each === "GET backlinks")).toHaveLength(2));
  next = () => Promise.resolve(json(backlinkPage(notes.id, 2, null)));
  answer(json(backlinkPage(notes.id, 1, null)));
  await waitFor(() => expect(backlinked()).toEqual([["Guide"], ["Notes"]]));
  release();
  await waitFor(() =>
    expect(backlinked()).toEqual([
      ["Guide", " · 3 links"],
      ["Notes", " · 2 links"],
    ])
  );
});

test("a next page read after a cursor the list read again no longer ends with is not added", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  let first = backlinkPage(guide.id, 1, "1");
  let answer!: (response: Response) => void;
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/backlinks": (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        return cursor === null ? json(first) : new Promise((resolve) => (answer = resolve));
      },
    },
  });
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  const more = within(section("Backlinks")).getByRole("button", { name: "More backlinks" });

  await user.click(more);
  await waitFor(() => expect(more.getAttribute("aria-busy")).toBe("true"));
  first = backlinkPage(guide.id, 2, "2");
  await readAgain();
  await waitFor(() => expect(backlinked()).toEqual([["Guide", " · 2 links"]]));
  answer(json(backlinkPage(notes.id, 1, null)));
  await waitFor(() => expect(more.getAttribute("aria-busy")).toBeNull());
  expect(backlinked()).toEqual([["Guide", " · 2 links"]]);
});

test("a first page of pages the tree does not have yet, with more to read, does not say none link here", async () => {
  const draft = pageNode(9, "Draft");
  const server = pageServer();
  server.backlinks.set(install.id, [backlinkPage(draft.id, 1, "1"), backlinkPage(notes.id, 1, null)]);
  renderApp(pagePath(install.id), server.app);

  expect(await within(await shownPanel()).findByRole("button", { name: "More backlinks" })).toBeTruthy();
  expect(within(section("Backlinks")).queryByText("No page links here.")).toBeNull();
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
      // A table's \\| ends the target; a title may hold ](.
      { key: "escaped", value: "[[Guide\\|]]" },
      { key: "titled", value: '[Linux](Linux "a](b")' },
    ],
    links: [
      { key: "up", node_id: guide.id },
      { key: "related.0", node_id: notes.id },
      { key: "related.1", node_id: linux.id },
      { key: "related.2", node_id: null },
      { key: "related.3", node_id: guide.id },
      { key: "escaped", node_id: guide.id },
      { key: "titled", node_id: linux.id },
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
    ["escaped", "Guide"],
    ["titled", "Linux"],
  ]);
  const links = within(section("Properties")).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href")])).toEqual([
    ["the guide", pagePath(guide.id)],
    ["Notes", pagePath(notes.id)],
    ["Linux *x*", pagePath(linux.id)],
    ["Guide > Intro", pagePath(guide.id)],
    ["Guide", pagePath(guide.id)],
    ["Linux", pagePath(linux.id)],
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

test("property links at a path two values share are theirs in turn, in the order written, those an object holds too", async () => {
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    properties: [
      { key: "rel.0", value: "[[Guide]]" },
      { key: "rel", value: ["[[Notes]]", "[draft]"] },
      { key: "a", value: { b: "[[Missing]]" } },
      { key: "a.b", value: "[[Linux]]" },
      // A value that is no link takes none: the next value at its path has it.
      { key: "x.0", value: "plain" },
      { key: "x", value: ["[[Guide]]"] },
    ],
    links: [
      { key: "rel.0", node_id: guide.id },
      { key: "rel.0", node_id: notes.id },
      { key: "a.b", node_id: null },
      { key: "a.b", node_id: linux.id },
      { key: "x.0", node_id: guide.id },
    ],
  });
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Linux" });
  expect(properties()).toEqual([
    ["rel.0", "Guide"],
    ["rel", "Notes[draft]"],
    ["a", '{"b":"[[Missing]]"}'],
    ["a.b", "Linux"],
    ["x.0", "plain"],
    ["x", "Guide"],
  ]);
  const links = within(section("Properties")).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href")])).toEqual([
    ["Guide", pagePath(guide.id)],
    ["Notes", pagePath(notes.id)],
    ["Linux", pagePath(linux.id)],
    ["Guide", pagePath(guide.id)],
  ]);
});

test("a value that is no link takes none of its path's: an anchor alone, a bracketed word, an address elsewhere, a space around; a list's lists pass theirs", async () => {
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    properties: [
      { key: "a.0", value: "[[#Top]]" },
      { key: "a", value: ["[[Guide]]"] },
      { key: "b.0", value: "[WIP]" },
      { key: "b", value: ["[[Notes]]"] },
      { key: "c.0", value: "[site](https://example.com)" },
      { key: "c", value: ["[[Linux]]"] },
      { key: "d.0", value: " [[Guide]]" },
      { key: "d", value: ["[[Notes]]"] },
      { key: "e", value: [["[[Guide]]"]] },
      { key: "e.0.0", value: "[[Linux]]" },
    ],
    links: [
      { key: "a.0", node_id: guide.id },
      { key: "b.0", node_id: notes.id },
      { key: "c.0", node_id: linux.id },
      { key: "d.0", node_id: notes.id },
      { key: "e.0.0", node_id: guide.id },
      { key: "e.0.0", node_id: linux.id },
    ],
  });
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  expect(properties()).toEqual([
    ["a.0", "[[#Top]]"],
    ["a", "Guide"],
    ["b.0", "[WIP]"],
    ["b", "Notes"],
    ["c.0", "[site](https://example.com)"],
    ["c", "Linux"],
    ["d.0", " [[Guide]]"],
    ["d", "Notes"],
    ["e", '["[[Guide]]"]'],
    ["e.0.0", "Linux"],
  ]);
  const links = within(section("Properties")).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href")])).toEqual([
    ["Guide", pagePath(guide.id)],
    ["Notes", pagePath(notes.id)],
    ["Linux", pagePath(linux.id)],
    ["Notes", pagePath(notes.id)],
    ["Linux", pagePath(linux.id)],
  ]);
});

test("which values take their path's link is the server's shape of one: a target, no address elsewhere, one link whole", async () => {
  const notLinks = [
    "[[ ]]",
    "[[\\|b]]",
    "[[a]]x",
    "[[a]b]]",
    "[a](#x)",
    "[a]( #x)",
    "[a](//x.test)",
    "[site](<https://x.test>)",
    "[a](Hub) [b](Other)",
    "[Install](Install Guide)",
  ];
  // An address in <> with a space, a title, parentheses a pair deep.
  const links = ["[Install](<Install Guide>)", '[a](Linux "Linux")', "[a](Notes(1))"];
  const server = pageServer();
  // Each value at a path a list's first item shares: a value that is no link leaves the item its link.
  server.properties.set(install.id, {
    valid: true,
    properties: [
      ...notLinks.flatMap((value, at) => [
        { key: `n${at.toString()}.0`, value },
        { key: `n${at.toString()}`, value: ["[[Guide]]"] },
      ]),
      ...links.flatMap((value, at) => [
        { key: `l${at.toString()}.0`, value },
        { key: `l${at.toString()}`, value: ["[[Notes]]"] },
      ]),
    ],
    links: [
      ...notLinks.map((_, at) => ({ key: `n${at.toString()}.0`, node_id: guide.id })),
      ...links.flatMap((_, at) => [
        { key: `l${at.toString()}.0`, node_id: linux.id },
        { key: `l${at.toString()}.0`, node_id: notes.id },
      ]),
    ],
  });
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Install" });
  expect(properties()).toEqual([
    ...notLinks.flatMap((value, at) => [
      [`n${at.toString()}.0`, value],
      [`n${at.toString()}`, "Guide"],
    ]),
    ["l0.0", "Install"],
    ["l0", "Notes"],
    ["l1.0", "a"],
    ["l1", "Notes"],
    ["l2.0", "a"],
    ["l2", "Notes"],
  ]);
  const led = within(section("Properties"))
    .getAllByRole("link")
    .map((link) => link.getAttribute("href"));
  expect(led).toEqual([
    ...notLinks.map(() => pagePath(guide.id)),
    ...links.flatMap(() => [pagePath(linux.id), pagePath(notes.id)]),
  ]);
});
