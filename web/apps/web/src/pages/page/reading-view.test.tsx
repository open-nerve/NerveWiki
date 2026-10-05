import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import { readingEnhancements, type Enhancement } from "../../reading/enhancement";
import { notebookJSON } from "../../test/fakes";
import { guide, install, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The reading view's enhancements (M4/P5 design 3.8).

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  // jsdom scrolls nothing: scrolls gives elements a way.
  Reflect.deleteProperty(Element.prototype, "scrollIntoView");
});

/** recording is an enhancement that logs, as name, the HTML it runs on and what it knows, and the HTML it is undone on. */
function recording(log: string[], name: string): Enhancement {
  return (container, context) => {
    const { workspace, notebook, page, revision, role } = context;
    const known = notebook === notebookJSON.id && page === install.id ? "Install" : "?";
    log.push(`${name} on ${container.innerHTML}: ${workspace} ${known} ${revision} ${role}`);
    return () => log.push(`undo ${name} on ${container.innerHTML}`);
  };
}

test("the enhancements run on the HTML in their order, undone in the reverse before it is replaced and as the page goes", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer({ role: "editor" });
  const log: string[] = [];
  const { router } = renderApp(pagePath(install.id), server.app, {
    enhancements: [recording(log, "a"), recording(log, "b")],
  });

  await waitFor(() => expect(log).toHaveLength(2));
  expect(log).toEqual(["a on <p>Install</p>: lab Install 1 editor", "b on <p>Install</p>: lab Install 1 editor"]);

  server.views.set(install.id, { html: "<p>Install, changed</p>", revision: 2 });
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));
  await waitFor(() => expect(log).toHaveLength(6));
  expect(log.slice(2)).toEqual([
    "undo b on <p>Install</p>",
    "undo a on <p>Install</p>",
    "a on <p>Install, changed</p>: lab Install 2 editor",
    "b on <p>Install, changed</p>: lab Install 2 editor",
  ]);

  await act(() => router.navigate(`/lab/notebooks/${notebookJSON.id}`));
  expect(log.slice(6)).toEqual(["undo b on <p>Install, changed</p>", "undo a on <p>Install, changed</p>"]);
});

const throws: Enhancement = () => {
  throw new Error("enhancement");
};

test("an enhancement that throws leaves the page and the others be", async () => {
  const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const log: string[] = [];
  renderApp(pagePath(install.id), pageServer().app, { enhancements: [throws, recording(log, "b")] });

  expect((await screen.findByRole("article")).innerHTML).toBe("<p>Install</p>");
  await waitFor(() => expect(log).toEqual(["b on <p>Install</p>: lab Install 1 admin"]));
  expect(error).toHaveBeenCalledTimes(1);
});

test("an enhancement reads the view again through its context", async () => {
  const server = pageServer();
  const reloads: (() => void)[] = [];
  renderApp(pagePath(install.id), server.app, {
    enhancements: [
      (_container, context) => {
        reloads.push(context.reload);
        return undefined;
      },
    ],
  });
  await waitFor(() => expect(reloads).toHaveLength(1));
  server.views.set(install.id, { html: "<p>Install, again</p>", revision: 2 });

  act(() => reloads[0]?.());

  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Install, again</p>"));
  expect(server.sent.filter((line) => line === "GET view Install")).toHaveLength(2);
});

/** section is the view's heading named name. */
function section(name: string): HTMLElement {
  return screen.getByRole("heading", { level: 2, name });
}

/** scrolls records the elements scrolled into view, as jsdom scrolls none. */
function scrolls(): Element[] {
  const scrolled: Element[] = [];
  Element.prototype.scrollIntoView = function (this: Element) {
    scrolled.push(this);
  };
  return scrolled;
}

test("an address's anchor has the view go to its element once the HTML is in, which takes the focus; read again, the view stays", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(install.id, { html: '<p>intro</p><h2 id="nw-part-two">Part Two</h2>', revision: 1 });
  const reloads: (() => void)[] = [];
  const { router } = renderApp(`${pagePath(install.id)}#nw-part-two`, server.app, {
    enhancements: [
      (_container, context) => {
        reloads.push(context.reload);
        return undefined;
      },
    ],
  });

  const heading = await screen.findByRole("heading", { level: 2, name: "Part Two" });
  expect(scrolled).toEqual([heading]);
  expect(document.activeElement).toBe(heading);
  expect(heading.getAttribute("tabindex")).toBe("-1");

  server.views.set(install.id, { html: '<p>intro, again</p><h2 id="nw-part-two">Part Two</h2>', revision: 2 });
  act(() => reloads.at(-1)?.());
  await waitFor(() => expect(screen.getByRole("article").textContent).toContain("intro, again"));
  expect(scrolled).toHaveLength(1);

  await act(() => router.navigate(`${pagePath(install.id)}#nw-part-two`));
  expect(scrolled).toEqual([heading, screen.getByRole("heading", { level: 2, name: "Part Two" })]);
});

test("an anchor written escaped names the element of its id", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-über">Über</h2>', revision: 1 });
  renderApp(`${pagePath(install.id)}#nw-%C3%BCber`, server.app);

  const heading = await screen.findByRole("heading", { level: 2, name: "Über" });
  expect(scrolled).toEqual([heading]);
});

/** viewReads is how many times server's view of Install was read. */
function viewReads(server: ReturnType<typeof pageServer>): number {
  return server.sent.filter((line) => line === "GET view Install").length;
}

test.each(["#nw-none", "#nw-%E0%A4"])(
  "an anchor the view has no element of, a malformed escape among them, has it read again, once, then gives the page's heading the focus: %s",
  async (anchor) => {
    const scrolled = scrolls();
    const server = pageServer();
    // Each read another HTML, none with the element.
    const read = server.views.get.bind(server.views);
    let reads = 0;
    server.views.get = (id) => (id === install.id ? { html: `<p>read ${++reads}</p>`, revision: reads } : read(id));
    renderApp(`${pagePath(install.id)}${anchor}`, server.app);

    const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
    await waitFor(() => expect(document.activeElement).toBe(heading));
    expect(screen.getByRole("article").innerHTML).toBe("<p>read 2</p>");
    expect(viewReads(server)).toBe(2);
    expect(scrolled).toEqual([]);
  }
);

test("an anchor the address changes to has the view go there, though the history's key is the same (an address typed in)", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2><h2 id="nw-b">B</h2>', revision: 1 });
  const { router } = renderApp(
    [
      { pathname: pagePath(install.id), hash: "#nw-b", key: "typed" },
      { pathname: pagePath(install.id), hash: "#nw-a", key: "typed" },
    ],
    server.app
  );

  const a = await screen.findByRole("heading", { level: 2, name: "A" });
  expect(scrolled).toEqual([a]);
  await act(() => router.navigate(-1));
  expect(scrolled).toEqual([a, screen.getByRole("heading", { level: 2, name: "B" })]);
});

test("an element with an id that had the focus has it back in the view read again, shown again if it showed and no longer does", async () => {
  const scroll = vi.fn();
  Element.prototype.scrollIntoView = scroll;
  // Where an element is in the window: at its data-at, 20 high, as jsdom lays out nothing.
  const laidOut = vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    const top = Number(this.getAttribute("data-at") ?? 0);
    return { top, bottom: top + 20 } as DOMRect;
  });
  onTestFinished(() => laidOut.mockRestore());
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-a" data-at="10">A</h2>', revision: 1 });
  const reloads: (() => void)[] = [];
  renderApp(`${pagePath(install.id)}#nw-a`, server.app, {
    enhancements: [
      (_container, context) => {
        reloads.push(context.reload);
        return undefined;
      },
    ],
  });
  /** readAgain has the view read again, its HTML html, what scrolled before forgotten. */
  const readAgain = async (html: string, revision: number) => {
    scroll.mockClear();
    server.views.set(install.id, { html, revision });
    act(() => reloads.at(-1)?.());
    await waitFor(() => expect(screen.getByRole("article").textContent).toBe(html.replace(/<[^>]*>/g, "")));
  };
  await screen.findByRole("heading", { level: 2, name: "A" });

  // It showed and shows still: the focus, no scroll; focusable as the anchor's target is.
  await readAgain('<h2 id="nw-a" data-at="20">A, still</h2>', 2);
  expect(document.activeElement).toBe(section("A, still"));
  expect(section("A, still").getAttribute("tabindex")).toBe("-1");
  expect(scroll).not.toHaveBeenCalled();

  // It showed and no longer does: the focus, and shown again.
  await readAgain('<h2 id="nw-a" data-at="2000">A, moved</h2>', 3);
  expect(document.activeElement).toBe(section("A, moved"));
  expect([scroll.mock.contexts, scroll.mock.calls]).toEqual([[section("A, moved")], [[{ block: "nearest" }]]]);

  // It did not show: the focus, no scroll.
  await readAgain('<h2 id="nw-a" data-at="3000">A, out of view</h2>', 4);
  expect(document.activeElement).toBe(section("A, out of view"));
  expect(scroll).not.toHaveBeenCalled();

  // Not in the new HTML: no focus.
  await readAgain('<p id="nw-p">gone</p>', 5);
  expect(document.activeElement).toBe(document.body);
  expect(scroll).not.toHaveBeenCalled();
});

test("an anchor of no element that a link of the page goes to leaves the focus on the link", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(install.id, { html: '<p><a href="#nw-gone">gone</a></p>', revision: 1 });
  const { router } = renderApp(pagePath(install.id), server.app);
  const link = within(await screen.findByRole("article")).getByRole("link", { name: "gone" });
  link.focus();

  await act(() => router.navigate(`${pagePath(install.id)}#nw-gone`));
  // The view read again for the anchor, and that read in.
  await waitFor(() => expect(viewReads(server)).toBe(2));
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  expect(router.state.location.hash).toBe("#nw-gone");
  expect(document.activeElement).toBe(link);
  expect(scrolled).toEqual([]);
});

test("an anchor that only the view on its way has is gone to once it is in", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const scrolled = scrolls();
  const server = pageServer();
  const { router } = renderApp(pagePath(install.id), server.app);
  expect((await screen.findByRole("article")).innerHTML).toBe("<p>Install</p>");
  await act(() => router.navigate(pagePath(guide.id)));
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  server.views.set(install.id, { html: '<h2 id="nw-x">X</h2>', revision: 2 });
  // Past SWR's deduplication: the view shown again from its cache is read again.
  await act(() => vi.advanceTimersByTimeAsync(3_000));

  await act(() => router.navigate(`${pagePath(install.id)}#nw-x`));
  const x = await screen.findByRole("heading", { level: 2, name: "X" });
  expect(document.activeElement).toBe(x);
  expect(scrolled).toEqual([x]);
});

test("going back to an address with an anchor goes to it again, after an address of the page without one", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1 });
  const { router } = renderApp(`${pagePath(install.id)}#nw-a`, server.app);
  const a = await screen.findByRole("heading", { level: 2, name: "A" });

  await act(() => router.navigate(pagePath(install.id)));
  await act(() => router.navigate(-1));
  expect(router.state.location.hash).toBe("#nw-a");
  expect(scrolled).toEqual([a, a]);
});

test("a link to a page goes there through the router, with the app's enhancements, arriving at the page unless at an anchor", async () => {
  scrolls();
  // The view's scrolling enhancement watches widths, which jsdom does not have.
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    }
  );
  const user = userEvent.setup();
  const server = pageServer();
  server.views.set(install.id, {
    html: `<p><a data-nw-node="${guide.id}" data-nw-anchor="nw-x">at x</a> <a data-nw-node="${guide.id}">Guide</a></p>`,
    revision: 1,
  });
  const { router } = renderApp(pagePath(install.id), server.app, { enhancements: readingEnhancements });

  await user.click(within(await screen.findByRole("article")).getByRole("link", { name: "Guide" }));
  expect(await screen.findByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(pagePath(guide.id));
  expect(router.state.location.state).toEqual({ arrived: true });

  await act(() => router.navigate(-1));
  await user.click(within(await screen.findByRole("article", { name: "Install" })).getByRole("link", { name: "at x" }));
  await waitFor(() => expect(router.state.location.hash).toBe("#nw-x"));
  expect([router.state.location.pathname, router.state.location.state]).toEqual([pagePath(guide.id), null]);
});
