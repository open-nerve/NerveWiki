import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { PreferencesStore } from "../../stores/preferences.store";
import { AppStores } from "../../stores/root.store";
import { darkScheme, json, memoryStorage } from "../../test/fakes";
import { pageEditor } from "../../test/page-editor";
import { panel, scrolls, shownPanel } from "../../test/page-panel";
import { guide, install, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The page's right column (M6/P7 design 7–10): where it is, and its outline; its toggle in the page's head, the
// section read marked in its outline, followed in its own scroll (M13/P4/S1, decisions 859–861, 886, 893).

afterEach(() => {
  Reflect.deleteProperty(Element.prototype, "scrollIntoView");
  Reflect.deleteProperty(document.documentElement, "scrollHeight");
});

/** outlined is what the outline lists: each heading's text, and how far it is indented. */
function outlined(): string[][] {
  const outline = within(panel()).getByRole("navigation", { name: "Outline" });
  return within(outline)
    .getAllByRole("listitem")
    .map((item) => [item.textContent ?? "", item.style.paddingLeft]);
}

test("the outline lists the page's headings with an id, but the footnotes', by their text without a footnote's number or an image's address, an attachment's image by its text, indented by their level from the page's highest", async () => {
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
      '<h3 id="nw-diagram"><img class="nw-asset" src="/api/v0/assets/a/content" alt="架构图" loading="lazy"></h3>',
      '<h3 id="nw-talk"><audio class="nw-asset" src="/api/v0/assets/b/content" controls="" preload="none" aria-label="讲解"></audio></h3>',
      '<h3 id="nw-demo"><video class="nw-asset" src="/api/v0/assets/c/content" controls="" preload="none" aria-label="演示"></video></h3>',
      '<div class="footnotes"><ol><li id="nw-fn:1"><h4 id="nw-in-a-note">In a note</h4></li></ol></div>',
    ].join(""),
    revision: 1,
    assets_expire_at: null,
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
    ["架构图", "0.75rem"],
    ["讲解", "0.75rem"],
    ["演示", "0.75rem"],
  ]);
});

/** h3s is the HTML of count h3 headings, H0 on. */
function h3s(count: number): string {
  return Array.from({ length: count }, (_, at) => `<h3 id="nw-h${at.toString()}">H${at.toString()}</h3>`).join("");
}

test("the outline lists the first 1,000 headings, indented from the highest of them, and says how many more the page has, after its list", async () => {
  const server = pageServer();
  // 1,000 h3, then an h1, an h2, one whose text is a footnote's number alone and those of an attachment's image or
  // video alone not listed, counted; one without text and one of the footnotes, not.
  server.views.set(install.id, {
    html: [
      h3s(1_000),
      '<h1 id="nw-top">Top</h1><h2 id="nw-blank"> </h2><h2 id="nw-next">Next</h2>',
      '<h2 id="nw-ref"><sup id="nw-fnref:1"><a href="#nw-fn:1">1</a></sup></h2>',
      '<h2 id="nw-pic"><img class="nw-asset" src="/api/v0/assets/a/content" alt="p" loading="lazy"></h2>',
      '<h2 id="nw-clip"><video class="nw-asset" src="/api/v0/assets/c/content" controls="" preload="none" aria-label="c"></video></h2>',
      '<div class="footnotes"><ol><li id="nw-fn:1"><h4 id="nw-in-a-note">In a note</h4></li></ol></div>',
    ].join(""),
    revision: 1,
    assets_expire_at: null,
  });
  const { unmount } = renderApp(pagePath(install.id), server.app);

  const outline = await screen.findByRole("navigation", { name: "Outline" });
  const items = within(outline).getAllByRole("listitem");
  expect([items.length, items.at(-1)?.textContent, items[0]?.style.paddingLeft]).toEqual([1_000, "H999", "0rem"]);
  expect(within(outline).getByText("…and 5 more").closest("li")).toBeNull();
  unmount();

  // As many as listed: none more.
  server.views.set(install.id, { html: h3s(1_000), revision: 2, assets_expire_at: null });
  renderApp(pagePath(install.id), server.app);
  const all = await screen.findByRole("navigation", { name: "Outline" });
  expect(within(all).getAllByRole("listitem")).toHaveLength(1_000);
  expect(within(all).queryByText(/more/)).toBeNull();
});

test("the outline indents from the page's highest heading, whichever it is", async () => {
  const server = pageServer();
  server.views.set(install.id, {
    html: '<h4 id="nw-a">A</h4><h3 id="nw-b">B</h3><h4 id="nw-c">C</h4>',
    revision: 1,
    assets_expire_at: null,
  });
  renderApp(pagePath(install.id), server.app);

  await screen.findByRole("navigation", { name: "Outline" });
  expect(outlined()).toEqual([
    ["A", "0.75rem"],
    ["B", "0rem"],
    ["C", "0.75rem"],
  ]);
});

test("a heading of the outline goes to its heading through the router, which shows and takes the focus, each time", async () => {
  const scrolled = scrolls();
  const user = userEvent.setup();
  const server = pageServer();
  server.views.set(install.id, {
    html: '<p>intro</p><h2 id="nw-安装">安装</h2><h2 id="nw-b">B</h2>',
    revision: 1,
    assets_expire_at: null,
  });
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
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1, assets_expire_at: null });
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

  server.views.set(install.id, {
    html: '<h1 id="nw-a">A</h1><h3 id="nw-c">C</h3>',
    revision: 2,
    assets_expire_at: null,
  });
  act(() => reloads.at(-1)?.());
  await waitFor(() =>
    expect(outlined()).toEqual([
      ["A", "0rem"],
      ["C", "1.5rem"],
    ])
  );

  server.views.set(install.id, { html: "<p>No headings</p>", revision: 3, assets_expire_at: null });
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
  // Scrolling itself from xl up, it is no stop of Tab, which Firefox makes it (M13 closeout, review A's M13CA-M3).
  expect(panel().getAttribute("tabindex")).toBe("-1");
});

/** editOutline is the right column's outline while the page is edited: its headings' buttons. */
function editOutline(): HTMLElement[] {
  return within(within(panel()).getByRole("navigation", { name: "Outline" })).getAllByRole("button");
}

test("while the page is edited the outline is the editor's: a heading's button puts the cursor at its line, the editor focused; back to reading, the outline's links are the view's (decision 876)", async () => {
  const server = pageServer({ role: "editor" });
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1, assets_expire_at: null });
  server.contents.set(install.id, { content: "## A\n\nText\n\n## B\n\n### C\n", revision: 1 });
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("navigation", { name: "Outline" });

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  // There as the editor shows: read as it is made, not 300 ms later (review A's M3).
  await screen.findByRole("textbox", { name: "Page content" });
  expect(editOutline().map((button) => [button.textContent, button.closest("li")?.style.paddingLeft])).toEqual([
    ["A", "0rem"],
    ["B", "0rem"],
    ["C", "0.75rem"],
  ]);
  const { content, view } = await pageEditor();
  // In the column, first: before the backlinks.
  const outline = within(panel()).getByRole("navigation", { name: "Outline" });
  expect(
    outline.compareDocumentPosition(within(panel()).getByText("Backlinks")) & Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy();

  await userEvent.setup().click(within(outline).getByRole("button", { name: "B" }));
  expect(document.activeElement).toBe(content);
  expect(view.state.selection.main.head).toBe(view.state.doc.line(5).from);

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await screen.findByRole("button", { name: "Edit" });
  const reading = await within(await shownPanel()).findByRole("navigation", { name: "Outline" });
  expect(
    within(reading)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["A"]);
  expect(within(reading).queryByRole("button")).toBeNull();
});

test("a change of the content keeps the outline's button that has the focus, as its heading moves (review A's M2)", async () => {
  const server = pageServer({ role: "editor" });
  server.contents.set(install.id, { content: "## A\n\n## B\n\n## C\n", revision: 1 });
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("button", { name: "Edit" });
  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  const { view } = await pageEditor();
  const focused = editOutline()[2];
  focused?.focus();
  expect(focused?.textContent).toBe("C");

  // Not typed, the focus where it is: as an upload's link goes in.
  act(() => view.dispatch({ changes: { from: 0, insert: "## New\n\n" } }));
  await waitFor(() => expect(editOutline().map((button) => button.textContent)).toEqual(["New", "A", "B", "C"]));
  expect(document.activeElement).toBe(focused);
  expect(editOutline()[2]).toBe(focused);
});

/**
 * heldContents answers the reads of the page's content, content each time: the first count at once, those after when
 * release is called. The edit opens (its session) before its content is read.
 */
function heldContents(content: string, count: number) {
  let reads = 0;
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const answer = async () => {
    reads++;
    if (reads > count) {
      await held;
    }
    return json({ content, revision: 1, content_hash: "0".repeat(64) });
  };
  return { answer, release: () => release?.() };
}

/** outlineLinks is the texts of the links of the right column's outline: the view's. */
function outlineLinks(): (string | null)[] {
  const outline = within(panel()).getByRole("navigation", { name: "Outline" });
  return within(outline)
    .getAllByRole("link")
    .map((link) => link.textContent);
}

/** editOpened opens the edit with its keys, and resolves once it is open, its content maybe not read yet. */
async function editOpened(): Promise<void> {
  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await waitFor(() => expect(screen.queryByRole("button", { name: "Edit" })).toBeNull());
}

test("as the edit opens, its content read, the right column keeps the view's outline, measuring nothing, until the editor tells its own (review A's P4S3A-M1)", async () => {
  const contents = heldContents("## A\n\n## B\n", 0);
  const server = pageServer({ role: "editor", answers: { "GET /api/v0/pages/*/content": contents.answer } });
  server.views.set(install.id, { html: '<h2 id="nw-v">V</h2>', revision: 1, assets_expire_at: null });
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("navigation", { name: "Outline" });

  await editOpened();
  // Its content held: the column's sections as they were, the outline first.
  expect(outlineLinks()).toEqual(["V"]);
  expect([...panel().querySelectorAll("nav")].map((nav) => nav.getAttribute("aria-label"))[0]).toBe("Outline");

  contents.release();
  await pageEditor();
  expect(editOutline().map((button) => button.textContent)).toEqual(["A", "B"]);
});

/** columnOutline is the right column's outline, the view's or the editor's. */
const columnOutline = () => within(panel()).getByRole("navigation", { name: "Outline" });

/** outlineInert tells whether the right column's outline section, its summary with it, is inert. */
const outlineInert = () => columnOutline().closest("details")?.hasAttribute("inert");

// The view's outline kept as the edit opens is inert, its section's summary too: not to be clicked nor tabbed to, a
// link followed there no jump to an old heading once the edit is left, the focus on its summary not lost as the
// editor's section comes (review A's P4S3A-F1-N1, F2-N1). Read, and the editor's, are not.
test("as the edit opens, its content read, the view's outline section it keeps is inert, its summary with it; read and once the editor tells, the section is not", async () => {
  const contents = heldContents("## A\n\n## B\n", 0);
  const server = pageServer({ role: "editor", answers: { "GET /api/v0/pages/*/content": contents.answer } });
  server.views.set(install.id, { html: '<h2 id="nw-v">V</h2>', revision: 1, assets_expire_at: null });
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("navigation", { name: "Outline" });
  expect(outlineInert()).toBe(false);

  await editOpened();
  expect(outlineLinks()).toEqual(["V"]);
  expect(outlineInert()).toBe(true);
  contents.release();
  await pageEditor();
  expect(editOutline().map((button) => button.textContent)).toEqual(["A", "B"]);
  expect(outlineInert()).toBe(false);
});

test("left and opened again, its content read, the edit shows none of the last edit's headings: the view's outline until the editor tells (review C's P4S3C-N1)", async () => {
  const contents = heldContents("## A\n\n## B\n", 1);
  const server = pageServer({ role: "editor", answers: { "GET /api/v0/pages/*/content": contents.answer } });
  server.views.set(install.id, { html: '<h2 id="nw-v">V</h2>', revision: 1, assets_expire_at: null });
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("navigation", { name: "Outline" });
  await editOpened();
  await pageEditor();
  expect(editOutline().map((button) => button.textContent)).toEqual(["A", "B"]);
  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await screen.findByRole("button", { name: "Edit" });
  await waitFor(() => expect(outlineLinks()).toEqual(["V"]));

  await editOpened();
  expect(outlineLinks()).toEqual(["V"]);
  expect(within(panel()).queryByRole("button", { name: "A" })).toBeNull();
  contents.release();
  await pageEditor();
  expect(editOutline().map((button) => button.textContent)).toEqual(["A", "B"]);
});

test("while the page is edited the outline lists the first 1,000 headings and says how many more, as read (review C's P4S3C-M3)", async () => {
  const server = pageServer({ role: "editor" });
  const content = Array.from({ length: 1_003 }, (_, at) => `## H${at.toString()}`).join("\n\n");
  server.contents.set(install.id, { content, revision: 1 });
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("button", { name: "Edit" });
  await editOpened();
  await pageEditor();
  const outline = within(panel()).getByRole("navigation", { name: "Outline" });
  await within(outline).findByText("…and 3 more");
  const buttons = within(outline).getAllByRole("button");
  expect([buttons.length, buttons.at(-1)?.textContent]).toEqual([1_000, "H999"]);
});

/** storedIn is app with its preferences over storage: what this browser had stored as the page opened. */
function storedIn(app: AppStores, storage: ReturnType<typeof memoryStorage>): AppStores {
  return new AppStores(
    new PreferencesStore({ storage, darkScheme: darkScheme(false), languages: ["en-US"] }),
    app.session,
    app.events,
    app.transfer
  );
}

/** toggle is the right column's toggle in the page's head. */
function toggle(): HTMLElement {
  return within(screen.getByRole("main")).getByRole("button", { name: "About this page" });
}

/** outlineLink is the outline's link to the heading named name. */
function outlineLink(name: string): HTMLElement {
  return within(within(panel()).getByRole("navigation", { name: "Outline" })).getByRole("link", { name });
}

/** current is the text of the outline's links marked as the section read. */
function current(): string[] {
  return within(within(panel()).getByRole("navigation", { name: "Outline" }))
    .getAllByRole("link")
    .filter((link) => link.hasAttribute("aria-current"))
    .map((link) => `${link.textContent ?? ""} ${link.getAttribute("aria-current") ?? ""}`);
}

/** frame settles once the window has drawn its next frame. */
function frame(): Promise<void> {
  return new Promise((resolve) => {
    requestAnimationFrame(() => resolve());
  });
}

/** windowScrolled has the window scroll and waits for a frame, the section read measured in it. */
async function windowScrolled(): Promise<void> {
  await act(async () => {
    fireEvent.scroll(window);
    await frame();
  });
}

/** jsdom's own getBoundingClientRect, which answers an empty box: what laidOut answers of the rest. */
const box = Element.prototype.getBoundingClientRect;

/**
 * laidOut stubs what jsdom does not lay out (design 7.1): the window 900 high, the page far taller, at its top; the
 * article's headings at the tops given by id (0 unless given), each measure of one counted; the right column 300 high,
 * its outline's items 28 each from its top, less its own scroll. columned gives the column its heights once it shows.
 */
function laidOut() {
  const tops = new Map<string, number>();
  const measured: string[] = [];
  vi.stubGlobal("innerHeight", 900);
  Object.defineProperty(document.documentElement, "scrollHeight", { configurable: true, get: () => 100_000 });
  vi.spyOn(window, "scrollY", "get").mockReturnValue(0);
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    if (/^H[1-6]$/.test(this.tagName) && this.closest("article") !== null) {
      measured.push(this.id);
      return new DOMRect(0, tops.get(this.id) ?? 0, 600, 28);
    }
    if (this.matches("aside[data-page-panel]")) {
      return new DOMRect(1000, 0, 252, 300);
    }
    const column = this.closest<HTMLElement>("aside[data-page-panel]");
    if (column !== null && this.matches("nav li")) {
      const index = [...(this.parentElement?.children ?? [])].indexOf(this);
      return new DOMRect(1000, index * 28 - column.scrollTop, 252, 28);
    }
    return box.call(this);
  });
  return {
    tops,
    measured,
    /** at places the 40 headings H0–H39, 100 apart, so that the line (300) reads H<index>. */
    at(index: number) {
      for (let i = 0; i < 40; i++) {
        tops.set(`nw-h${i.toString()}`, 100 * i - (100 * index - 250));
      }
    },
    columned(column: HTMLElement) {
      Object.defineProperty(column, "clientHeight", { configurable: true, value: 300 });
      Object.defineProperty(column, "scrollHeight", { configurable: true, value: 2_000 });
    },
  };
}

/**
 * unfollowed keeps jsdom from following a link the router lets be (a modifier's press) to another document, which it
 * has not (review C's R2-M1): our own onClick has seen the press by then.
 */
function unfollowed(event: MouseEvent) {
  if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) {
    event.preventDefault();
  }
}

/** forty is the view of a page of 40 headings, H0–H39. */
const forty = { html: h3s(40), revision: 1, assets_expire_at: null };

test("the toggle in the page's head folds the right column away and shows it again, expanded or not, controlling it, the focus kept on it; folded is stored for the next visit", async () => {
  const user = userEvent.setup();
  const storage = memoryStorage();
  renderApp(pagePath(install.id), storedIn(pageServer().app, storage));
  const column = await shownPanel();
  const button = toggle();
  expect([button.getAttribute("aria-expanded"), button.getAttribute("aria-controls")]).toEqual(["true", column.id]);
  expect(column.getAttribute("data-open")).toBe("true");

  await user.click(button);
  expect([button.getAttribute("aria-expanded"), column.getAttribute("data-open")]).toEqual(["false", "false"]);
  expect(storage.values["nwiki.panel"]).toBe("closed");
  expect(document.activeElement).toBe(button);

  await user.click(button);
  expect([button.getAttribute("aria-expanded"), column.getAttribute("data-open")]).toEqual(["true", "true"]);
  expect(storage.values["nwiki.panel"]).toBe("open");
  expect(document.activeElement).toBe(button);
});

test("stored folded, the page opens with its right column folded away; in Chinese the toggle is named as the column is", async () => {
  const server = pageServer();
  const app = storedIn(server.app, memoryStorage({ "nwiki.panel": "closed" }));
  app.preferences.setLocale("zh-CN");
  renderApp(pagePath(install.id), app);

  const column = await screen.findByRole("complementary", { name: "页面信息" });
  const button = within(screen.getByRole("main")).getByRole("button", { name: "页面信息" });
  expect([button.getAttribute("aria-expanded"), column.getAttribute("data-open")]).toEqual(["false", "false"]);
});

test("folded away with the focus in it, the right column gives the focus to its toggle, the window not scrolled", async () => {
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1, assets_expire_at: null });
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("navigation", { name: "Outline" });
  const link = outlineLink("A");
  link.focus();
  const focus = vi.spyOn(HTMLElement.prototype, "focus");

  // A click that gives the button no focus, as WebKit's on macOS.
  fireEvent.click(toggle());
  expect(document.activeElement).toBe(toggle());
  const given = focus.mock.contexts.indexOf(toggle());
  expect(given).not.toBe(-1);
  expect(focus.mock.calls[given]).toEqual([{ preventScroll: true }]);
});

test("the outline marks the section read, as the window scrolls, as the location", async () => {
  const layout = laidOut();
  const server = pageServer();
  server.views.set(install.id, forty);
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("navigation", { name: "Outline" });

  layout.at(7);
  await windowScrolled();
  expect(current()).toEqual(["H7 location"]);
  layout.at(12);
  await windowScrolled();
  expect(current()).toEqual(["H12 location"]);
});

test("of a page of 1,001 headings, all above the line, the section read is the last listed", async () => {
  laidOut();
  const server = pageServer();
  server.views.set(install.id, { html: h3s(1_001), revision: 1, assets_expire_at: null });
  renderApp(pagePath(install.id), server.app);
  const outline = await screen.findByRole("navigation", { name: "Outline" });

  await windowScrolled();
  expect(within(outline).getAllByRole("listitem")).toHaveLength(1_000);
  expect(within(outline).getByText("…and 1 more")).toBeTruthy();
  expect(current()).toEqual(["H999 location"]);
});

test("folded away, or not beside the page, the right column measures none of the page's headings", async () => {
  const folded = laidOut();
  const server = pageServer();
  server.views.set(install.id, forty);
  const { unmount } = renderApp(pagePath(install.id), storedIn(server.app, memoryStorage({ "nwiki.panel": "closed" })));
  await screen.findByRole("navigation", { name: "Outline" });
  await windowScrolled();
  expect(folded.measured).toEqual([]);
  expect(current()).toEqual([]);
  unmount();

  // Below xl the column is after the page's content (decision 886).
  vi.stubGlobal("matchMedia", (query: string) => ({
    matches: false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
  const narrow = laidOut();
  const beside = pageServer();
  beside.views.set(install.id, forty);
  renderApp(pagePath(install.id), beside.app);
  await screen.findByRole("navigation", { name: "Outline" });
  await windowScrolled();
  expect(narrow.measured).toEqual([]);
  expect(current()).toEqual([]);
});

test("shown again, the outline measures and marks the section read, the column following it; folded away, it measures no more", async () => {
  const user = userEvent.setup();
  const layout = laidOut();
  const server = pageServer();
  server.views.set(install.id, forty);
  const { unmount } = renderApp(pagePath(install.id), storedIn(server.app, memoryStorage({ "nwiki.panel": "closed" })));
  await screen.findByRole("navigation", { name: "Outline" });
  layout.columned(panel());
  layout.at(30);

  await user.click(toggle());
  expect(current()).toEqual(["H30 location"]);
  expect(panel().scrollTop).toBe(568);
  unmount();

  const shown = laidOut();
  const again = pageServer();
  again.views.set(install.id, forty);
  renderApp(pagePath(install.id), again.app);
  await screen.findByRole("navigation", { name: "Outline" });
  await user.click(toggle());
  shown.measured.length = 0;
  await windowScrolled();
  expect(shown.measured).toEqual([]);
});

test("the right column follows the section read only as far as shows it, scrolling itself alone, until the reader moves in it", async () => {
  const intoView = scrolls();
  const scrollTo = vi.spyOn(window, "scrollTo").mockImplementation(() => {});
  const layout = laidOut();
  const server = pageServer();
  server.views.set(install.id, forty);
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("navigation", { name: "Outline" });
  const column = panel();
  layout.columned(column);

  // H30's item below the column's 300: its bottom brought to the column's, not to its middle.
  layout.at(30);
  await windowScrolled();
  expect(current()).toEqual(["H30 location"]);
  expect(column.scrollTop).toBe(30 * 28 - 300 + 28);
  // H25's in view: the column stays.
  layout.at(25);
  await windowScrolled();
  expect(current()).toEqual(["H25 location"]);
  expect(column.scrollTop).toBe(568);
  expect([intoView, scrollTo.mock.calls]).toEqual([[], []]);

  // The reader scrolls the column: it follows no more.
  fireEvent.wheel(column);
  column.scrollTop = 0;
  layout.at(31);
  await windowScrolled();
  expect(current()).toEqual(["H31 location"]);
  expect(column.scrollTop).toBe(0);
});

test("the reader's scroll of the column stops its following until the outline's list mounts again: after an edit and back, it follows (decision 897)", async () => {
  const layout = laidOut();
  const server = pageServer({ role: "editor" });
  server.views.set(install.id, forty);
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("navigation", { name: "Outline" });
  const column = panel();
  layout.columned(column);
  layout.at(30);
  await windowScrolled();
  expect(column.scrollTop).toBe(568);

  fireEvent.wheel(column);
  column.scrollTop = 0;
  layout.at(31);
  await windowScrolled();
  expect(current()).toEqual(["H31 location"]);
  expect(column.scrollTop).toBe(0);

  // Into the edit, where the column's outline is the editor's, and back: the outline's list mounts anew.
  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await pageEditor();
  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await screen.findByRole("button", { name: "Edit" });
  await within(await shownPanel()).findByRole("navigation", { name: "Outline" });
  expect(panel()).toBe(column);
  layout.at(35);
  await windowScrolled();
  expect(current()).toEqual(["H35 location"]);
  expect(column.scrollTop).toBe(35 * 28 + 28 - 300);
});

test("an outline come after its page, its heading clicked, is marked until gone from view; the column follows still", async () => {
  scrolls();
  const user = userEvent.setup();
  const layout = laidOut();
  const server = pageServer();
  server.views.set(install.id, forty);
  server.viewsHeld = true;
  renderApp(pagePath(install.id), server.app);
  const column = await shownPanel();
  await screen.findByRole("heading", { level: 1, name: "Install" });
  expect(within(column).queryByRole("navigation", { name: "Outline" })).toBeNull();

  act(() => server.release());
  await screen.findByRole("navigation", { name: "Outline" });
  layout.columned(column);
  // H1 at 16, in view: the line reads H3.
  for (let i = 0; i < 40; i++) {
    layout.tops.set(`nw-h${i.toString()}`, 100 * i - 84);
  }
  await user.click(outlineLink("H1"));
  expect(current()).toEqual(["H1 location"]);
  await act(() => frame());
  expect(current()).toEqual(["H1 location"]);

  // Seen, then gone from view: the line reads H32, whose item the column brings into its view.
  layout.at(32);
  await windowScrolled();
  expect(current()).toEqual(["H32 location"]);
  expect(column.scrollTop).toBe(32 * 28 + 28 - 300);
});

test("a press on an outline's heading with a modifier marks nothing, the tab or window it opens leaving the view where it is; a plain one marks it at once", async () => {
  scrolls();
  const user = userEvent.setup();
  const layout = laidOut();
  const server = pageServer();
  server.views.set(install.id, forty);
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("navigation", { name: "Outline" });
  layout.at(5);
  await windowScrolled();
  expect(current()).toEqual(["H5 location"]);
  document.addEventListener("click", unfollowed, true);
  try {
    for (const key of ["Control", "Meta", "Shift", "Alt"]) {
      // oxlint-disable-next-line no-await-in-loop -- one press after the other
      await user.keyboard(`{${key}>}`);
      // oxlint-disable-next-line no-await-in-loop -- one press after the other
      await user.click(outlineLink("H9"));
      // oxlint-disable-next-line no-await-in-loop -- one press after the other
      await user.keyboard(`{/${key}}`);
      expect(current(), key).toEqual(["H5 location"]);
    }
  } finally {
    document.removeEventListener("click", unfollowed, true);
  }

  await user.click(outlineLink("H9"));
  expect(current()).toEqual(["H9 location"]);
});
