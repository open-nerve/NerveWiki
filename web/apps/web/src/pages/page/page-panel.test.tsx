import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test } from "vitest";

import { pageEditor } from "../../test/page-editor";
import { panel, scrolls, shownPanel } from "../../test/page-panel";
import { guide, install, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The page's right column (M6/P7 design 7–10): where it is, and its outline.

afterEach(() => {
  Reflect.deleteProperty(Element.prototype, "scrollIntoView");
});

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

/** h3s is the HTML of count h3 headings, H0 on. */
function h3s(count: number): string {
  return Array.from({ length: count }, (_, at) => `<h3 id="nw-h${at.toString()}">H${at.toString()}</h3>`).join("");
}

test("the outline lists the first 1,000 headings, indented from the highest of them, and says how many more the page has, after its list", async () => {
  const server = pageServer();
  // 1,000 h3, then an h1, an h2 and one whose text is a footnote's number alone not listed, counted; one without
  // text and one of the footnotes, not.
  server.views.set(install.id, {
    html: [
      h3s(1_000),
      '<h1 id="nw-top">Top</h1><h2 id="nw-blank"> </h2><h2 id="nw-next">Next</h2>',
      '<h2 id="nw-ref"><sup id="nw-fnref:1"><a href="#nw-fn:1">1</a></sup></h2>',
      '<div class="footnotes"><ol><li id="nw-fn:1"><h4 id="nw-in-a-note">In a note</h4></li></ol></div>',
    ].join(""),
    revision: 1,
  });
  const { unmount } = renderApp(pagePath(install.id), server.app);

  const outline = await screen.findByRole("navigation", { name: "Outline" });
  const items = within(outline).getAllByRole("listitem");
  expect([items.length, items.at(-1)?.textContent, items[0]?.style.paddingLeft]).toEqual([1_000, "H999", "0rem"]);
  expect(within(outline).getByText("…and 3 more").closest("li")).toBeNull();
  unmount();

  // As many as listed: none more.
  server.views.set(install.id, { html: h3s(1_000), revision: 2 });
  renderApp(pagePath(install.id), server.app);
  const all = await screen.findByRole("navigation", { name: "Outline" });
  expect(within(all).getAllByRole("listitem")).toHaveLength(1_000);
  expect(within(all).queryByText(/more/)).toBeNull();
});

test("the outline indents from the page's highest heading, whichever it is", async () => {
  const server = pageServer();
  server.views.set(install.id, { html: '<h4 id="nw-a">A</h4><h3 id="nw-b">B</h3><h4 id="nw-c">C</h4>', revision: 1 });
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
