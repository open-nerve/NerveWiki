import { act, configure, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test, vi } from "vitest";

import { notebookJSON } from "../../test/fakes";
import { pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { readersInput } from "../page/readers-input";

// The page shown's row brought into the left column's view (M13/P2/S2, decisions 779, 791): by the column's area
// alone, never the window; at most once a navigation, a hash, a query or a replace of the address shown the same
// one; not once the reader has done something in the area since the navigation.

const home = `/lab/notebooks/${notebookJSON.id}`;
/** The page n, Page 01 to Page 60, each at the root in its order. */
const page = (n: number) => pageNode(10 + n, `Page ${String(n).padStart(2, "0")}`);
const page01 = page(1);
const page60 = page(60);
const sixty = Array.from({ length: 60 }, (_, at) => page(at + 1));

/** How far below the area's top the tree's first link is, a row's height, and how much of the area shows. */
const above = 120;
const rowHeight = 30;
const shows = 300;

const tree = () => screen.findByRole("navigation", { name: "Pages of Plans" });

/**
 * layOut lays out what jsdom does not: the column's area shows 300px at the top of the viewport; the tree's links are
 * rows of 30px one under the other, the first 120px down the area, each at its offset less the area's scrollTop.
 * Nothing else has a box. scrollIntoView and the window's scrollTo are spies: the reveal calls neither.
 */
function layOut() {
  Element.prototype.scrollIntoView = () => undefined;
  const intoView = vi.spyOn(Element.prototype, "scrollIntoView");
  const windowScroll = vi.spyOn(window, "scrollTo").mockImplementation(() => undefined);
  const laidOut = vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    const scroller = this.closest<HTMLElement>("[data-sidebar-scroll]");
    if (scroller === null) {
      return new DOMRect();
    }
    if (this === scroller) {
      return new DOMRect(0, 0, 240, shows);
    }
    const nav = this.closest('nav[aria-label="Pages of Plans"]');
    if (nav === null || !(this instanceof HTMLAnchorElement)) {
      return new DOMRect();
    }
    const offset = above + [...nav.querySelectorAll("a")].indexOf(this) * rowHeight;
    return new DOMRect(0, offset - scroller.scrollTop, 200, rowHeight);
  });
  onTestFinished(() => {
    laidOut.mockRestore();
    windowScroll.mockRestore();
    intoView.mockRestore();
    Reflect.deleteProperty(Element.prototype, "scrollIntoView");
  });
  return { intoView, windowScroll };
}

/** The column's area that scrolls. */
function area(): HTMLElement {
  const found = document.querySelector<HTMLElement>("[data-sidebar-scroll]");
  if (found === null) throw new Error("the column has no area that scrolls");
  return found;
}

/** Whether the link is all in the area's view, as layOut lays it out. */
function inView(link: HTMLElement): boolean {
  const { top, bottom } = link.getBoundingClientRect();
  return top >= 0 && bottom <= shows;
}

/** The tree's link of the page titled title, once it shows. */
async function row(title: string): Promise<HTMLElement> {
  return within(await tree()).findByRole("link", { name: title });
}

/** A key the reader presses in the main, a scrolling one. */
const keyInMain = () => fireEvent.keyDown(screen.getByRole("main"), { key: "ArrowDown" });

test("the page shown's row below the column's view is brought into it by the column's area alone: neither scrollIntoView nor the window's scroll", async () => {
  const { intoView, windowScroll } = layOut();
  renderApp(pagePath(page60.id), pageServer({ nodes: sixty }).app);

  const link = await row("Page 60");
  await waitFor(() => expect(inView(link)).toBe(true));
  // 120 + 59 × 30 + 30 = 1,920 down, the area showing 300: 1,620 to scroll, 8 to spare.
  expect(area().scrollTop).toBe(1_628);
  expect(intoView).not.toHaveBeenCalled();
  expect(windowScroll).not.toHaveBeenCalled();
});

test("the page shown's row in the column's view stays where it is", async () => {
  layOut();
  renderApp(pagePath(page01.id), pageServer({ nodes: sixty }).app);

  await row("Page 01");
  await screen.findByRole("heading", { level: 1, name: "Page 01" });
  expect(area().scrollTop).toBe(0);
});

test("the row shown again in the same navigation is not brought into view again", async () => {
  layOut();
  const folder = page(61);
  const inside = pageNode(80, "Inside", folder);
  renderApp(pagePath(inside.id), pageServer({ nodes: [...sixty.slice(0, 59), folder, inside] }).app);
  const link = await row("Inside");
  await waitFor(() => expect(inView(link)).toBe(true));
  // The reader's scroll, by the scrollbar's: the area alone.
  area().scrollTop = 0;
  const subpages = within(await tree()).getByRole("button", { name: "Subpages of Page 61" });

  fireEvent.click(subpages);
  expect(within(await tree()).queryByRole("link", { name: "Inside" })).toBeNull();
  fireEvent.click(subpages);

  expect(await row("Inside")).not.toBe(link);
  expect(area().scrollTop).toBe(0);
});

test("each navigation brings the row in, three and more, a key pressed in the main and a row of the tree clicked between each", async () => {
  const user = userEvent.setup();
  layOut();
  renderApp(pagePath(page01.id), pageServer({ nodes: sixty }).app);
  await row("Page 01");
  await screen.findByRole("heading", { level: 1, name: "Page 01" });
  expect(area().scrollTop).toBe(0);

  // Each one far from the other: the row to go to is out of view each time.
  const to = async (title: string) => {
    const before = area().scrollTop;
    keyInMain();
    await user.click(await row(title));
    await screen.findByRole("heading", { level: 1, name: title });
    const link = await row(title);
    await waitFor(() => expect(inView(link)).toBe(true));
    expect(area().scrollTop).not.toBe(before);
  };
  await to("Page 60");
  await to("Page 01");
  await to("Page 60");
  await to("Page 01");
});

test("from a page to the notebook's home, the column stays as the reader left it: no row is the page shown", async () => {
  layOut();
  const { router } = renderApp(pagePath(page60.id), pageServer({ nodes: sixty }).app);
  const link = await row("Page 60");
  await waitFor(() => expect(inView(link)).toBe(true));
  area().scrollTop = 0;

  await act(() => router.navigate(home));

  await screen.findByRole("heading", { level: 1, name: "Plans" });
  expect(link.getAttribute("aria-current")).toBeNull();
  expect(area().scrollTop).toBe(0);
});

test("from the notebook's home, a key pressed in the column's area, then a page: its row is brought in", async () => {
  layOut();
  const { router } = renderApp(home, pageServer({ nodes: sixty }).app);
  await row("Page 60");
  // In the area, which the home's navigation counts: the page's is another, its watch made anew.
  fireEvent.keyDown(area(), { key: "ArrowDown" });

  await act(() => router.navigate(pagePath(page60.id)));

  const link = await row("Page 60");
  await waitFor(() => expect(inView(link)).toBe(true));
});

test("a wheel in the column's area before the tree is read: the row of the page, read since, is not brought in; the next page's is", async () => {
  layOut();
  const server = pageServer({ nodes: sixty });
  server.nodesHeld = true;
  const { router } = renderApp(pagePath(page60.id), server.app);
  await tree();
  await waitFor(() => expect(server.held.length).toBeGreaterThan(0));

  fireEvent.wheel(area());
  server.nodesHeld = false;
  act(() => server.release());

  const link = await row("Page 60");
  await screen.findByRole("heading", { level: 1, name: "Page 60" });
  expect(inView(link)).toBe(false);
  expect(area().scrollTop).toBe(0);

  // Another navigation, its row drawn and out of view: the wheel was the last one's (at most once, not never).
  await act(() => router.navigate(pagePath(page(59).id)));
  await screen.findByRole("heading", { level: 1, name: "Page 59" });
  const next = await row("Page 59");
  await waitFor(() => expect(inView(next)).toBe(true));
});

test("in StrictMode, its unmount and mount again ending the first watch, a wheel in the column's area before the tree is read still keeps the row out", async () => {
  configure({ reactStrictMode: true });
  onTestFinished(() => configure({ reactStrictMode: false }));
  layOut();
  const server = pageServer({ nodes: sixty });
  server.nodesHeld = true;
  renderApp(pagePath(page60.id), server.app);
  await tree();
  await waitFor(() => expect(server.held.length).toBeGreaterThan(0));

  fireEvent.wheel(area());
  server.nodesHeld = false;
  act(() => server.release());

  const link = await row("Page 60");
  await screen.findByRole("heading", { level: 1, name: "Page 60" });
  expect(inView(link)).toBe(false);
  expect(area().scrollTop).toBe(0);
});

test("a wheel in the main before the tree is read: the row of the page, read since, is brought in", async () => {
  layOut();
  const server = pageServer({ nodes: sixty });
  server.nodesHeld = true;
  renderApp(pagePath(page60.id), server.app);
  await tree();
  await waitFor(() => expect(server.held.length).toBeGreaterThan(0));

  fireEvent.wheel(screen.getByRole("main"));
  server.nodesHeld = false;
  act(() => server.release());

  const link = await row("Page 60");
  await waitFor(() => expect(inView(link)).toBe(true));
});

test("on the page shown, the outline's heading and a replace of the address shown are the same navigation: the area the reader scrolled stays", async () => {
  const user = userEvent.setup();
  layOut();
  const server = pageServer({ nodes: sixty });
  server.views.set(page60.id, { html: '<h2 id="nw-part">Part</h2><p>Text</p>', revision: 1, assets_expire_at: null });
  const { router } = renderApp(pagePath(page60.id), server.app);
  const link = await row("Page 60");
  await waitFor(() => expect(inView(link)).toBe(true));

  fireEvent.wheel(area());
  area().scrollTop = 0;
  const outline = await screen.findByRole("navigation", { name: "Outline" });
  await user.click(within(outline).getByRole("link", { name: "Part" }));
  await waitFor(() => expect(router.state.location.hash).toBe("#nw-part"));
  expect(area().scrollTop).toBe(0);

  await act(() => router.navigate(`${pagePath(page60.id)}#nw-part`, { replace: true }));
  expect(router.state.historyAction).toBe("REPLACE");
  expect(area().scrollTop).toBe(0);
});

test("a hash of the page changed before the tree is read: the row, read since, is brought in", async () => {
  layOut();
  const server = pageServer({ nodes: sixty });
  server.views.set(page60.id, { html: '<h2 id="nw-part">Part</h2><p>Text</p>', revision: 1, assets_expire_at: null });
  server.nodesHeld = true;
  const { router } = renderApp(pagePath(page60.id), server.app);
  await tree();
  await waitFor(() => expect(server.held.length).toBeGreaterThan(0));

  await act(() => router.navigate(`${pagePath(page60.id)}#nw-part`));
  server.nodesHeld = false;
  act(() => server.release());

  const link = await row("Page 60");
  await waitFor(() => expect(inView(link)).toBe(true));
});

test("leaving the notebook before its tree is read ends the watch of the reader: each of its events unheard once", async () => {
  layOut();
  const server = pageServer({ nodes: sixty });
  server.nodesHeld = true;
  const { router } = renderApp(home, server.app);
  await tree();
  await waitFor(() => expect(server.held.length).toBeGreaterThan(0));
  const removed = vi.spyOn(window, "removeEventListener");
  onTestFinished(() => removed.mockRestore());

  await act(() => router.navigate("/lab"));
  await screen.findByRole("heading", { level: 1, name: "Lab" });

  const unheard = removed.mock.calls.filter(([, , options]) => options === true).map(([type]) => type);
  expect(readersInput.map((type) => unheard.filter((each) => each === type).length)).toEqual([1, 1, 1, 1]);
});
