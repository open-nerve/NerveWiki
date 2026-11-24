import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import { eventHandlers } from "../../events/handlers";
import { diagrams, type Drawer } from "../../reading/diagrams";
import { readingEnhancements, type Enhancement, type ReadingContext } from "../../reading/enhancement";
import { scrollRegions } from "../../reading/scroll-regions";
import { taskToggle } from "../../reading/task-toggle";
import { unresolvedLinks } from "../../reading/unresolved-links";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, upgradeOf, workspaceJSON } from "../../test/fakes";
import { pageEditor } from "../../test/page-editor";
import { assetNode, guide, install, pagePath, pageServer } from "../../test/page-server";
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
    const { workspace, notebook, page, revision } = context;
    const known = notebook === notebookJSON.id && page === install.id ? "Install" : "?";
    log.push(`${name} on ${container.innerHTML}: ${workspace} ${known} ${revision}`);
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
  expect(log).toEqual(["a on <p>Install</p>: lab Install 1", "b on <p>Install</p>: lab Install 1"]);

  server.views.set(install.id, { html: "<p>Install, changed</p>", revision: 2, assets_expire_at: null });
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));
  await waitFor(() => expect(log).toHaveLength(6));
  expect(log.slice(2)).toEqual([
    "undo b on <p>Install</p>",
    "undo a on <p>Install</p>",
    "a on <p>Install, changed</p>: lab Install 2",
    "b on <p>Install, changed</p>: lab Install 2",
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
  await waitFor(() => expect(log).toEqual(["b on <p>Install</p>: lab Install 1"]));
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
  server.views.set(install.id, { html: "<p>Install, again</p>", revision: 2, assets_expire_at: null });

  act(() => reloads[0]?.());

  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Install, again</p>"));
  expect(server.sent.filter((line) => line === "GET view Install")).toHaveLength(2);
});

test("a change of the theme runs no enhancement again, those that follow it hearing it; one of the language runs them again", async () => {
  const server = pageServer();
  server.views.set(install.id, { html: '<p>Install <a href="/x">x</a></p>', revision: 1, assets_expire_at: null });
  const log: string[] = [];
  const heard: string[] = [];
  renderApp(pagePath(install.id), server.app, {
    enhancements: [
      recording(log, "a"),
      (_container, { theme, onThemeChange }) => onThemeChange(() => heard.push(theme())),
    ],
  });
  await waitFor(() => expect(log).toHaveLength(1));
  const article = screen.getByRole("article", { name: "Install" });
  const first = article.firstChild;
  const link = within(article).getByRole("link", { name: "x" });
  link.focus();

  act(() => server.app.preferences.setTheme("dark"));
  expect(heard).toEqual(["dark"]);
  expect(log).toHaveLength(1);

  act(() => server.app.preferences.setLocale("zh-CN"));
  await waitFor(() => expect(log).toHaveLength(3));
  expect(log[1]).toBe('undo a on <p>Install <a href="/x">x</a></p>');
  // Its HTML as it was: the same nodes, the focus where it was (M12/P2/S4, 11.6: U5).
  expect(article.firstChild).toBe(first);
  expect(document.activeElement).toBe(link);
  // The listener undone hears no more: one for each change.
  act(() => server.app.preferences.setTheme("light"));
  expect(heard).toEqual(["dark", "light"]);
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
  server.views.set(install.id, {
    html: '<p>intro</p><h2 id="nw-part-two">Part Two</h2>',
    revision: 1,
    assets_expire_at: null,
  });
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

  server.views.set(install.id, {
    html: '<p>intro, again</p><h2 id="nw-part-two">Part Two</h2>',
    revision: 2,
    assets_expire_at: null,
  });
  act(() => reloads.at(-1)?.());
  await waitFor(() => expect(screen.getByRole("article").textContent).toContain("intro, again"));
  expect(scrolled).toHaveLength(1);

  await act(() => router.navigate(`${pagePath(install.id)}#nw-part-two`));
  expect(scrolled).toEqual([heading, screen.getByRole("heading", { level: 2, name: "Part Two" })]);
});

test("an anchor's element in a folded callout opens it to show, then takes the focus", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(install.id, {
    html: '<details class="nw-callout"><summary>Folded</summary><div><details><summary>In</summary><h2 id="nw-deep">Deep</h2></details></div></details>',
    revision: 1,
    assets_expire_at: null,
  });
  renderApp(`${pagePath(install.id)}#nw-deep`, server.app);

  const heading = await screen.findByRole("heading", { level: 2, name: "Deep", hidden: true });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(scrolled).toEqual([heading]);
  expect([...screen.getByRole("article").querySelectorAll("details")].map((each) => each.open)).toEqual([true, true]);
});

test("an anchor written escaped names the element of its id", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-über">Über</h2>', revision: 1, assets_expire_at: null });
  renderApp(`${pagePath(install.id)}#nw-%C3%BCber`, server.app);

  const heading = await screen.findByRole("heading", { level: 2, name: "Über" });
  expect(scrolled).toEqual([heading]);
});

/** viewReads is how many times server's view of Install was read. */
function viewReads(server: ReturnType<typeof pageServer>): number {
  return server.sent.filter((line) => line === "GET view Install").length;
}

test.each(["#nw-none", "#nw-%E0%A4"])(
  "an anchor the view has no element of, a malformed escape among them, gives the page's heading the focus as the page opens: %s",
  async (anchor) => {
    const scrolled = scrolls();
    const server = pageServer();
    renderApp(`${pagePath(install.id)}${anchor}`, server.app);

    const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
    await waitFor(() => expect(document.activeElement).toBe(heading));
    // Read as the page opened, it is not read again.
    expect(viewReads(server)).toBe(1);
    expect(scrolled).toEqual([]);
  }
);

test("a view from the cache is read again, once, for an anchor of no element, the page's heading taking the focus at once", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  // Each read another HTML, none with the element.
  const read = server.views.get.bind(server.views);
  let reads = 0;
  server.views.get = (id) =>
    id === install.id ? { html: `<p>read ${++reads}</p>`, revision: reads, assets_expire_at: null } : read(id);
  const { router } = renderApp(pagePath(install.id), server.app);
  expect((await screen.findByRole("article")).innerHTML).toBe("<p>read 1</p>");
  await act(() => router.navigate(pagePath(guide.id)));
  await screen.findByRole("heading", { level: 1, name: "Guide" });

  await act(() => router.navigate(`${pagePath(install.id)}#nw-none`));
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>read 2</p>"));
  expect(document.activeElement).toBe(heading);
  expect(viewReads(server)).toBe(2);
  expect(scrolled).toEqual([]);
});

/**
 * heldServer is a server whose views of Install are htmls in turn, the
 * second held until release is called; reads() is how many were asked for.
 */
function heldServer(htmls: string[]) {
  let release!: () => void;
  const held = new Promise<void>((resolve) => (release = resolve));
  let reads = 0;
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/view": async (request) => {
        if (!request.url.includes(install.id)) {
          return json({ html: "<p>Guide</p>", revision: 1, assets_expire_at: null });
        }
        const read = ++reads;
        if (read === 2) {
          await held;
        }
        return json({ html: htmls[Math.min(read, htmls.length) - 1], revision: read, assets_expire_at: null });
      },
    },
  });
  return { server, release, reads: () => reads };
}

/** openedFromTheCache has Install opened, left for Guide and opened again at anchor, its view from the cache. */
async function openedFromTheCache(
  server: ReturnType<typeof pageServer>,
  anchor: string,
  enhancements: Enhancement[] = []
) {
  const rendered = renderApp(pagePath(install.id), server.app, { enhancements });
  await screen.findByRole("article");
  await act(() => rendered.router.navigate(pagePath(guide.id)));
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  await act(() => rendered.router.navigate(`${pagePath(install.id)}${anchor}`));
  return rendered;
}

test("while the view is read again for the anchor the page opened at, the page has taken the address in: a link of the page to no element reads nothing more, and the read moves no focus", async () => {
  const scrolled = scrolls();
  const { server, release, reads } = heldServer(['<p><a href="#nw-gone">gone</a></p>', '<h2 id="nw-none">None</h2>']);
  const { router } = await openedFromTheCache(server, "#nw-none");
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(reads()).toBe(2);

  // A link of the page clicked, as Safari leaves it: the focus nowhere.
  heading.blur();
  await act(() => router.navigate(`${pagePath(install.id)}#nw-gone`));
  expect(reads()).toBe(2);
  expect(document.activeElement).toBe(document.body);
  await act(async () => release());
  await screen.findByRole("heading", { level: 2, name: "None" });
  expect(document.activeElement).toBe(document.body);
  expect(scrolled).toEqual([]);
});

test("once the view read again for the anchor is in without its element, a later read with it moves no focus", async () => {
  const scrolled = scrolls();
  const { server, release } = heldServer(["<p>one</p>", "<p>two</p>", '<h2 id="nw-x">X</h2>']);
  const reloads: (() => void)[] = [];
  await openedFromTheCache(server, "#nw-x", [
    (_container, context) => {
      reloads.push(context.reload);
      return undefined;
    },
  ]);
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  await act(async () => release());
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>two</p>"));

  // Read again later, as an event has it.
  act(() => reloads.at(-1)?.());
  await screen.findByRole("heading", { level: 2, name: "X" });
  expect(document.activeElement).toBe(heading);
  expect(scrolled).toEqual([]);
});

test.each(["wheel", "touchmove", "pointerdown", "keydown"])(
  "the element the view read again brings moves nothing once the reader did something: %s",
  async (type) => {
    const scrolled = scrolls();
    const { server, release } = heldServer(["<p>Install</p>", '<h2 id="nw-x">X</h2>']);
    await openedFromTheCache(server, "#nw-x");
    const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
    await waitFor(() => expect(document.activeElement).toBe(heading));

    fireEvent(window, new Event(type));
    await act(async () => release());
    await screen.findByRole("heading", { level: 2, name: "X" });
    expect(document.activeElement).toBe(heading);
    expect(scrolled).toEqual([]);
  }
);

test("the wait hears an input stopped where it went: it listens in the capture phase", async () => {
  const scrolled = scrolls();
  const { server, release } = heldServer(["<p>Install</p>", '<h2 id="nw-x">X</h2>']);
  await openedFromTheCache(server, "#nw-x");
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));

  const article = screen.getByRole("article");
  article.addEventListener("keydown", (event) => event.stopPropagation());
  fireEvent.keyDown(article, { key: "ArrowDown" });
  await act(async () => release());
  await screen.findByRole("heading", { level: 2, name: "X" });
  expect(document.activeElement).toBe(heading);
  expect(scrolled).toEqual([]);
});

test("the wait's listeners go once its read settles", async () => {
  scrolls();
  const added = vi.spyOn(window, "addEventListener");
  const removed = vi.spyOn(window, "removeEventListener");
  onTestFinished(() => {
    added.mockRestore();
    removed.mockRestore();
  });
  // The inputs listened to in the capture phase, as each listener's types, by the listener.
  const inputs = (spy: typeof added | typeof removed) => {
    const by = new Map<unknown, string[]>();
    for (const [type, listener, options] of spy.mock.calls) {
      const capture = options === true || (typeof options === "object" && options.capture === true);
      if (capture && ["wheel", "touchmove", "pointerdown", "keydown"].includes(type)) {
        by.set(listener, [...(by.get(listener) ?? []), type]);
      }
    }
    return by;
  };
  // What still listens, by the listener: the wait's, the tree's watch of the reader for its row's reveal having ended
  // as the row came into view (M13/P2/S2), as it had for the pages before.
  const listening = () => {
    const removedBy = inputs(removed);
    return [...inputs(added)].flatMap(([listener, types]) =>
      types.filter((type) => !(removedBy.get(listener) ?? []).includes(type)).map((type) => ({ listener, type }))
    );
  };
  const { server, release } = heldServer(["<p>Install</p>", '<h2 id="nw-x">X</h2>']);
  await openedFromTheCache(server, "#nw-x");
  await waitFor(() => expect(listening()).toHaveLength(4));
  const waits = new Set(listening().map(({ listener }) => listener));

  await act(async () => release());
  await screen.findByRole("heading", { level: 2, name: "X" });
  expect(listening().filter(({ listener }) => waits.has(listener))).toEqual([]);
  // The one left is the outline's, come with the view: its watch of the reader in the right column, while its list is
  // up (M13/P4/S1, decision 886).
  expect(listening().map(({ type }) => type)).toEqual(["wheel", "touchmove", "pointerdown", "keydown"]);
  expect(screen.getByRole("navigation", { name: "Outline" })).toBeTruthy();
});

test("the browser's own scrolling during the wait is not the reader's: the element the read brings takes the focus", async () => {
  const scrolled = scrolls();
  const { server, release } = heldServer(["<p>Install</p>", '<h2 id="nw-x">X</h2>']);
  await openedFromTheCache(server, "#nw-x");
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));

  fireEvent(window, new Event("scroll"));
  await act(async () => release());
  const x = await screen.findByRole("heading", { level: 2, name: "X" });
  await waitFor(() => expect(document.activeElement).toBe(x));
  expect(scrolled).toEqual([x]);
});

test("while the view is read again for the anchor the page opened at, another address of the page ends the wait: the read moves nothing", async () => {
  const scrolled = scrolls();
  const { server, release } = heldServer(["<p>Install</p>", '<h2 id="nw-x">X</h2><h2 id="nw-y">Y</h2>']);
  const { router } = await openedFromTheCache(server, "#nw-x");
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));

  // As an address typed in: neither the focus nor the page moved.
  await act(() => router.navigate(`${pagePath(install.id)}#nw-y`));
  await act(async () => release());
  await screen.findByRole("heading", { level: 2, name: "Y" });
  expect(document.activeElement).toBe(heading);
  expect(scrolled).toEqual([]);
});

test("while the view is read again for the anchor the page opened at, a link of the page to an element goes there, and the read moves nothing", async () => {
  const scrolled = scrolls();
  const { server, release } = heldServer([
    '<h2 id="nw-intro">Intro</h2>',
    '<h2 id="nw-intro">Intro</h2><h2 id="nw-x">X</h2>',
  ]);
  const { router } = await openedFromTheCache(server, "#nw-x");
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));

  await act(() => router.navigate(`${pagePath(install.id)}#nw-intro`));
  const intro = screen.getByRole("heading", { level: 2, name: "Intro" });
  expect(document.activeElement).toBe(intro);
  await act(async () => release());
  await screen.findByRole("heading", { level: 2, name: "X" });
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 2, name: "Intro" }));
  expect(scrolled).toEqual([intro]);
});

test("the element the view read again brings takes the focus only if the reader has not moved it", async () => {
  const scrolled = scrolls();
  const { server, release } = heldServer(["<p>Install</p>", '<h2 id="nw-x">X</h2>']);
  await openedFromTheCache(server, "#nw-x");
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));

  const outside = document.createElement("button");
  document.body.append(outside);
  onTestFinished(() => outside.remove());
  outside.focus();
  await act(async () => release());
  await screen.findByRole("heading", { level: 2, name: "X" });
  expect(document.activeElement).toBe(outside);
  expect(scrolled).toEqual([]);
});

test("an anchor the address changes to has the view go there, though the history's key is the same (an address typed in)", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(install.id, {
    html: '<h2 id="nw-a">A</h2><h2 id="nw-b">B</h2>',
    revision: 1,
    assets_expire_at: null,
  });
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
  server.views.set(install.id, { html: '<h2 id="nw-a" data-at="10">A</h2>', revision: 1, assets_expire_at: null });
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
    server.views.set(install.id, { html, revision, assets_expire_at: null });
    act(() => reloads.at(-1)?.());
    await waitFor(() => expect(screen.getByRole("article").textContent).toBe(html.replace(/<[^>]*>/g, "")));
  };
  await screen.findByRole("heading", { level: 2, name: "A" });

  // It showed and shows still: the focus, no scroll; focusable as the anchor's target is.
  await readAgain('<h2 id="nw-a" data-at="20">A, still</h2>', 2);
  expect(document.activeElement).toBe(section("A, still"));
  expect(section("A, still").getAttribute("tabindex")).toBe("-1");
  expect(scroll).not.toHaveBeenCalled();

  // It showed and no longer does, its text another: no heading of its text, the focus is where it was, the block
  // in its place, not scrolled (M12/P2/S4, 4.4.3's step 4).
  await readAgain('<h2 id="nw-a" data-at="2000">A, moved</h2>', 3);
  expect(document.activeElement).toBe(section("A, moved"));
  expect(scroll).not.toHaveBeenCalled();

  // It did not show: the focus, no scroll.
  await readAgain('<h2 id="nw-a" data-at="3000">A, out of view</h2>', 4);
  expect(document.activeElement).toBe(section("A, out of view"));
  expect(scroll).not.toHaveBeenCalled();

  // Not in the new HTML: the focus is where it was, on the block in its place, not scrolled (M12/P2/S4, 4.4.3's step
  // 4).
  await readAgain('<p id="nw-p">gone</p>', 5);
  expect(document.activeElement).toBe(screen.getByText("gone"));
  expect(screen.getByText("gone").getAttribute("tabindex")).toBe("-1");
  expect(scroll).not.toHaveBeenCalled();
});

test("an anchor of no element that a link of the page goes to leaves the focus where it is, the view not read again", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(install.id, {
    html: '<p><a href="#nw-gone">gone</a> <a href="#nw-none">none</a></p>',
    revision: 1,
    assets_expire_at: null,
  });
  const { router } = renderApp(pagePath(install.id), server.app);
  await screen.findByRole("article");
  // Opened again, its view from the cache.
  await act(() => router.navigate(pagePath(guide.id)));
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  await act(() => router.navigate(pagePath(install.id)));
  const link = within(await screen.findByRole("article")).getByRole("link", { name: "gone" });
  const reads = viewReads(server);

  // Nothing focused, as Safari leaves a link clicked.
  await act(() => router.navigate(`${pagePath(install.id)}#nw-none`));
  expect(router.state.location.hash).toBe("#nw-none");
  expect(document.activeElement).toBe(document.body);
  // The link focused, as other browsers leave it.
  link.focus();
  await act(() => router.navigate(`${pagePath(install.id)}#nw-gone`));
  expect(router.state.location.hash).toBe("#nw-gone");
  expect(document.activeElement).toBe(link);
  expect(viewReads(server)).toBe(reads);
  expect(scrolled).toEqual([]);
});

test("an anchor of no element as the page opens leaves the focus where the reader put it meanwhile", async () => {
  const outside = document.createElement("button");
  document.body.append(outside);
  onTestFinished(() => outside.remove());
  outside.focus();
  renderApp(`${pagePath(install.id)}#nw-none`, pageServer().app);

  expect((await screen.findByRole("article")).innerHTML).toBe("<p>Install</p>");
  expect(document.activeElement).toBe(outside);
});

test("an anchor that only the view read again has, the one shown from the cache, is gone to once it is in", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  const { router } = renderApp(pagePath(install.id), server.app);
  expect((await screen.findByRole("article")).innerHTML).toBe("<p>Install</p>");
  await act(() => router.navigate(pagePath(guide.id)));
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  // Within SWR's deduplication: the view, shown from its cache, is read again for the anchor.
  server.views.set(install.id, { html: '<h2 id="nw-x">X</h2>', revision: 2, assets_expire_at: null });

  await act(() => router.navigate(`${pagePath(install.id)}#nw-x`));
  const x = await screen.findByRole("heading", { level: 2, name: "X" });
  expect(document.activeElement).toBe(x);
  expect(scrolled).toEqual([x]);
});

test("going back to an address with an anchor goes to it again, after an address of the page without one", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1, assets_expire_at: null });
  const { router } = renderApp(`${pagePath(install.id)}#nw-a`, server.app);
  const a = await screen.findByRole("heading", { level: 2, name: "A" });

  await act(() => router.navigate(pagePath(install.id)));
  await act(() => router.navigate(-1));
  expect(router.state.location.hash).toBe("#nw-a");
  expect(scrolled).toEqual([a, a]);
});

test("coming back from editing goes to no anchor: the navigation was taken in", async () => {
  const scrolled = scrolls();
  const server = pageServer();
  server.views.set(guide.id, { html: '<h2 id="nw-a">A</h2>', revision: 1, assets_expire_at: null });
  renderApp(`${pagePath(guide.id)}#nw-a`, server.app);
  const a = await screen.findByRole("heading", { level: 2, name: "A" });
  expect(scrolled).toEqual([a]);

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await pageEditor();
  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  const edit = await screen.findByRole("button", { name: "Edit" });
  await screen.findByRole("heading", { level: 2, name: "A" });
  await waitFor(() => expect(document.activeElement).toBe(edit));
  expect(scrolled).toEqual([a]);
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
    assets_expire_at: null,
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

/**
 * signedAt is the address of node's content as the server writes it in HTML, of one blob, expiring at the hour e
 * after 2026-11-18 10:00, signed s (22 of it).
 */
function signedAt(node: string, e: number, s: string): string {
  const expiry = (1_795_003_200 + (e - 1) * 3_600).toString();
  return `/api/v0/assets/${node}/content?b=0199a2b4-0000-7000-8000-0000000b0001&amp;e=${expiry}&amp;s=${s.repeat(22)}`;
}

/** plain is an address as an attribute reads, its &amp; a &. */
const plain = (html: string) => html.replaceAll("&amp;", "&");

const chart = "0199a2b4-0000-7000-8000-0000000a0001";

/** signedView is a view of an image, a link to it, a callout and a paragraph, its addresses signed by s, expiring at e. */
const signedView = (e: number, s: string) =>
  `<p><img class="nw-asset" src="${signedAt(chart, e, s)}" alt="c.png"></p>` +
  `<p><a class="nw-wikilink nw-asset" href="${signedAt(chart, e, s)}" download="">c.png</a></p>` +
  "<details><summary>Tip</summary><p>in</p></details><p>tail</p>";

// M12/P2/S4, 17.1 (U3): each hour, the view read again differs in its addresses' expiry and signature alone. Their
// elements take the new ones, no node taken out or put in: what the reader has selected and opened stays.
test("a view read again whose attachments' addresses alone changed keeps every node, the selection and the callouts, runs no enhancement again, and the links and images have the new addresses", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  server.views.set(install.id, { html: signedView(1, "A"), revision: 1, assets_expire_at: inMinutes(10) });
  const log: string[] = [];
  renderApp(pagePath(install.id), server.app, { enhancements: [recording(log, "a")] });
  const article = await screen.findByRole("article", { name: "Install" });
  await waitFor(() => expect(log).toHaveLength(1));
  const details = article.querySelector("details") as HTMLDetailsElement;
  details.open = true;
  const tail = screen.getByText("tail").firstChild as Text;
  document.getSelection()?.setBaseAndExtent(tail, 1, tail, 3);
  const changes = childChanges(article);

  server.views.set(install.id, { html: signedView(2, "B"), revision: 1, assets_expire_at: inMinutes(70) });
  await act(() => vi.advanceTimersByTimeAsync(9.1 * 60_000));
  await waitFor(() => expect(article.querySelector("img")?.getAttribute("src")).toBe(plain(signedAt(chart, 2, "B"))));

  expect(article.querySelector("a")?.getAttribute("href")).toBe(plain(signedAt(chart, 2, "B")));
  expect([changes(), log.length, details.open, details.isConnected]).toEqual([0, 1, true, true]);
  expect(document.getSelection()?.anchorNode).toBe(tail);
  expect(document.getSelection()?.toString()).toBe("ai");
});

// M12/P2/S4, U2: a save that changed the content's blank lines alone is read again as the same HTML at another
// revision: nothing is written, and the next tick is sent on the revision read.
test("the same HTML read again at another revision writes nothing and runs no enhancement again; a tick is sent on the new revision", async () => {
  const server = pageServer({ role: "editor" });
  server.withTasks(install.id, "- [ ] a\n");
  const log: string[] = [];
  const contexts: ReadingContext[] = [];
  const capture: Enhancement = (_container, context) => void contexts.push(context);
  renderApp(pagePath(install.id), server.app, { enhancements: [taskToggle, recording(log, "a"), capture] });
  const article = await screen.findByRole("article", { name: "Install" });
  const box = await within(article).findByRole<HTMLInputElement>("checkbox", { name: "a" });
  await waitFor(() => expect(box.disabled).toBe(false));
  const html = article.innerHTML;
  const changes = childChanges(article);
  const runs = log.length;

  server.withTasks(install.id, "- [ ] a\n", 2);
  act(() => contexts.at(-1)?.reload());
  await waitFor(() => expect(contexts.at(-1)?.revision).toBe(2));
  expect([changes(), log.length, article.innerHTML]).toEqual([0, runs, html]);

  await userEvent.click(box);
  await waitFor(() =>
    expect(server.sent.filter((line) => line.startsWith("TOGGLE"))).toEqual([
      `TOGGLE Install ${box.dataset.task ?? ""} true on 2`,
    ])
  );
});

/** inMinutes is the time minutes from now, as the server writes it. */
const inMinutes = (minutes: number) => new Date(Date.now() + minutes * 60_000).toISOString();

test("a view is read again a minute before its attachments' addresses expire, once for its readers; one without them is not", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1, assets_expire_at: inMinutes(10) });
  renderApp(pagePath(install.id), server.app);
  const reads = () => server.sent.filter((line) => line === "GET view Install").length;
  // The outline reads it too.
  expect(await screen.findByRole("link", { name: "A" })).toBeTruthy();
  expect(reads()).toBe(1);

  server.views.set(install.id, { html: '<h2 id="nw-a">A, again</h2>', revision: 1, assets_expire_at: null });
  // Held: both readers' timers come while the read is out.
  server.viewsHeld = true;
  await act(() => vi.advanceTimersByTimeAsync(8.9 * 60_000));
  expect(reads()).toBe(1);
  await act(() => vi.advanceTimersByTimeAsync(0.2 * 60_000));
  expect(reads()).toBe(2);
  server.viewsHeld = false;
  act(() => server.release());
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe('<h2 id="nw-a">A, again</h2>'));
  expect(reads()).toBe(2);
  await act(() => vi.advanceTimersByTimeAsync(3 * 60 * 60_000));
  expect(reads()).toBe(2);
});

// M12/P2/S5, 17.6: a timer that falls behind the clock, the machine asleep, is caught up with.
test("a view whose timer falls behind, the machine asleep, is read again within a minute of waking", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1, assets_expire_at: inMinutes(10) });
  renderApp(pagePath(install.id), server.app);
  const reads = () => server.sent.filter((line) => line === "GET view Install").length;
  expect(await screen.findByRole("link", { name: "A" })).toBeTruthy();
  expect(reads()).toBe(1);

  // Asleep three hours: the clock goes on, the timers do not.
  vi.setSystemTime(Date.now() + 3 * 60 * 60_000);
  await act(() => vi.advanceTimersByTimeAsync(60_000));

  expect(reads()).toBe(2);
});

test("a view whose timer falls behind is read again at once as the page is shown again from the back-forward cache", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1, assets_expire_at: inMinutes(10) });
  renderApp(pagePath(install.id), server.app);
  const reads = () => server.sent.filter((line) => line === "GET view Install").length;
  expect(await screen.findByRole("link", { name: "A" })).toBeTruthy();
  expect(reads()).toBe(1);

  vi.setSystemTime(Date.now() + 3 * 60 * 60_000);
  act(() => void window.dispatchEvent(new Event("pageshow")));

  await waitFor(() => expect(reads()).toBe(2));
});

test("a view from the cache whose attachments' addresses had expired is not shown: it is read again", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  server.views.set(install.id, { html: "<p>Signed</p>", revision: 1, assets_expire_at: inMinutes(10) });
  const { router } = renderApp(pagePath(install.id), server.app);
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed</p>"));

  // Before they expire, the cache's shows at once.
  await act(() => router.navigate(pagePath(guide.id)));
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Guide</p>"));
  await act(() => vi.advanceTimersByTimeAsync(5 * 60_000));
  await act(() => router.navigate(pagePath(install.id)));
  expect(screen.getByRole("article").innerHTML).toBe("<p>Signed</p>");
  // Due to be read again, a minute before they expire, it still shows at once.
  await act(() => router.navigate(pagePath(guide.id)));
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Guide</p>"));
  await act(() => vi.advanceTimersByTimeAsync(4.5 * 60_000));
  await act(() => router.navigate(pagePath(install.id)));
  expect(screen.getByRole("article").innerHTML).toBe("<p>Signed</p>");

  await act(() => router.navigate(pagePath(guide.id)));
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Guide</p>"));
  await act(() => vi.advanceTimersByTimeAsync(1.5 * 60_000));
  server.views.set(install.id, { html: "<p>Signed anew</p>", revision: 1, assets_expire_at: inMinutes(70) });
  const before = server.sent.filter((line) => line === "GET view Install").length;
  server.viewsHeld = true;
  await act(() => router.navigate(pagePath(install.id)));
  expect(screen.queryByText("Signed")).toBeNull();
  // Read once, as SWR reads what it has from the cache: the expired one is read no more.
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  expect(server.sent.filter((line) => line === "GET view Install").length).toBe(before + 1);
  server.viewsHeld = false;
  act(() => server.release());
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed anew</p>"));
});

test("an answer read again the same is read again before its addresses expire: the server signs them for the hour", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  // Read half a minute into the server's hour: the same addresses until the next hour's end.
  server.views.set(install.id, { html: "<p>Signed</p>", revision: 1, assets_expire_at: inMinutes(119.5) });
  renderApp(pagePath(install.id), server.app);
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed</p>"));
  const reads = () => server.sent.filter((line) => line === "GET view Install").length;
  await act(() => vi.advanceTimersByTimeAsync(59.2 * 60_000));
  expect(reads()).toBe(2);
  server.views.set(install.id, { html: "<p>Signed anew</p>", revision: 1, assets_expire_at: inMinutes(120) });
  await act(() => vi.advanceTimersByTimeAsync(59 * 60_000));
  expect(reads()).toBe(3);
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed anew</p>"));
});

test("an expiry the view cannot read is none: the view is read once", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  server.views.set(install.id, { html: "<p>Signed</p>", revision: 1, assets_expire_at: "soon" });
  renderApp(pagePath(install.id), server.app);
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed</p>"));
  await act(() => vi.advanceTimersByTimeAsync(5 * 60_000));
  expect(server.sent.filter((line) => line === "GET view Install")).toHaveLength(1);
});

test("a clock far ahead of the server's: the view shows, is read again each half minute, and shows as the page is come back to", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  // By this clock, the addresses expired two hours ago.
  server.views.set(install.id, { html: "<p>Signed</p>", revision: 1, assets_expire_at: inMinutes(-120) });
  const { router } = renderApp(pagePath(install.id), server.app);
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed</p>"));
  const reads = () => server.sent.filter((line) => line === "GET view Install").length;
  await act(() => vi.advanceTimersByTimeAsync(31_000));
  expect(reads()).toBe(2);
  await act(() => vi.advanceTimersByTimeAsync(31_000));
  expect(reads()).toBe(3);

  await act(() => router.navigate(pagePath(guide.id)));
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Guide</p>"));
  await act(() => vi.advanceTimersByTimeAsync(60_000));
  await act(() => router.navigate(pagePath(install.id)));
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed</p>"));
});

test("a view that came as no reader had it, its addresses expired since, is not shown: it is read again", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  server.views.set(install.id, { html: "<p>Signed</p>", revision: 1, assets_expire_at: inMinutes(10) });
  server.viewsHeld = true;
  const { router } = renderApp(pagePath(install.id), server.app);
  await waitFor(() => expect(server.sent).toContain("GET view Install"));
  server.viewsHeld = false;
  await act(() => router.navigate(pagePath(guide.id)));
  act(() => server.release());
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Guide</p>"));
  await act(() => vi.advanceTimersByTimeAsync(15 * 60_000));
  server.views.set(install.id, { html: "<p>Signed anew</p>", revision: 1, assets_expire_at: inMinutes(70) });
  await act(() => router.navigate(pagePath(install.id)));
  expect(screen.queryByText("Signed")).toBeNull();
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed anew</p>"));
});

test("a view from the cache whose addresses had expired, read again in vain, says so; Try again reads it", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = pageServer();
  server.views.set(install.id, { html: "<p>Signed</p>", revision: 1, assets_expire_at: inMinutes(10) });
  const { router } = renderApp(pagePath(install.id), server.app);
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed</p>"));
  await act(() => router.navigate(pagePath(guide.id)));
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Guide</p>"));
  await act(() => vi.advanceTimersByTimeAsync(11 * 60_000));

  server.viewsDown = true;
  await act(() => router.navigate(pagePath(install.id)));
  const [again] = await screen.findAllByRole("button", { name: "Try again" });
  expect(screen.queryByText("Signed")).toBeNull();
  server.viewsDown = false;
  server.views.set(install.id, { html: "<p>Signed anew</p>", revision: 1, assets_expire_at: inMinutes(70) });
  const before = server.sent.filter((line) => line === "GET view Install").length;
  await user.click(again as HTMLElement);
  expect(server.sent.filter((line) => line === "GET view Install")).toHaveLength(before + 1);
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed anew</p>"));
});

test("a hidden tab's view is not read again as it is due, but as the tab is shown", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  server.views.set(install.id, { html: "<p>Signed</p>", revision: 1, assets_expire_at: inMinutes(10) });
  renderApp(pagePath(install.id), server.app);
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed</p>"));
  const reads = () => server.sent.filter((line) => line === "GET view Install").length;
  let hidden = true;
  const shown = (visible: boolean) => {
    hidden = !visible;
    document.dispatchEvent(new Event("visibilitychange"));
  };
  const spies = [
    vi.spyOn(document, "hidden", "get").mockImplementation(() => hidden),
    vi.spyOn(document, "visibilityState", "get").mockImplementation(() => (hidden ? "hidden" : "visible")),
  ];
  onTestFinished(() => spies.forEach((spy) => spy.mockRestore()));
  act(() => shown(false));
  await act(() => vi.advanceTimersByTimeAsync(30 * 60_000));
  expect(reads()).toBe(1);

  server.views.set(install.id, { html: "<p>Signed anew</p>", revision: 1, assets_expire_at: inMinutes(70) });
  act(() => shown(true));
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Signed anew</p>"));
  expect(reads()).toBe(2);
});

test("a view read again that says the same runs no enhancement again", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  server.views.set(install.id, { html: "<p>Signed</p>", revision: 1, assets_expire_at: "2100-01-01T00:00:00Z" });
  const log: string[] = [];
  renderApp(pagePath(install.id), server.app, { enhancements: [recording(log, "a")] });
  await waitFor(() => expect(log).toHaveLength(1));
  // Read again as the window has the focus back, past SWR's throttle of those reads.
  await act(() => vi.advanceTimersByTimeAsync(10_000));
  server.viewsHeld = true;
  act(() => window.dispatchEvent(new Event("focus")));
  await waitFor(() => expect(server.sent.filter((line) => line === "GET view Install")).toHaveLength(2));
  server.viewsHeld = false;
  await act(async () => server.release());
  await act(() => vi.advanceTimersByTimeAsync(100));
  expect(log).toHaveLength(1);
});

test("an enhancement has the view's expiry, and an attachment's address signed anew; one gone, it rejects", async () => {
  const server = pageServer();
  const sound = { ...assetNode(90, "a.mp3"), parent_id: install.id };
  server.nodes = [...server.nodes, sound];
  server.views.set(install.id, {
    html: `<p><audio class="nw-asset" src="${signedAt(sound.id, 1, "A")}"></audio></p>`,
    revision: 1,
    assets_expire_at: "2100-01-01T00:00:00Z",
  });
  const contexts: ReadingContext[] = [];
  renderApp(pagePath(install.id), server.app, {
    enhancements: [
      (_container, context) => {
        contexts.push(context);
        return undefined;
      },
    ],
  });
  await waitFor(() => expect(contexts).toHaveLength(1));

  expect(contexts[0]?.assetsExpire).toBe("2100-01-01T00:00:00Z");
  await expect(contexts[0]?.assetAddress(sound.id)).resolves.toBe(`/api/v0/assets/${sound.id}/content?sig=1`);
  await expect(contexts[0]?.assetAddress(guide.id)).rejects.toMatchObject({ status: 404 });
  expect(server.sent.filter((line) => line.startsWith("GET asset "))).toEqual([
    "GET asset a.mp3",
    `GET asset ${guide.id}`,
  ]);

  // Its addresses alone signed anew (U3): the enhancements are not run again, and have the new expiry (M12/P2/S4).
  server.views.set(install.id, {
    html: `<p><audio class="nw-asset" src="${signedAt(sound.id, 2, "B")}"></audio></p>`,
    revision: 1,
    assets_expire_at: "2100-01-01T01:00:00Z",
  });
  act(() => contexts[0]?.reload());
  await waitFor(() =>
    expect(screen.getByRole("article").querySelector("audio")?.getAttribute("src")).toBe(
      plain(signedAt(sound.id, 2, "B"))
    )
  );
  expect(contexts).toHaveLength(1);
  expect(contexts[0]?.assetsExpire).toBe("2100-01-01T01:00:00Z");
});

test("with the app's enhancements, a link to an attachment the browser shows opens in a tab of its own; a download does not", async () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    }
  );
  const server = pageServer();
  server.views.set(install.id, {
    html:
      '<p><a class="nw-asset" href="/api/v0/assets/x/content?b=1" data-nw-size="3">x.pdf</a> ' +
      '<a class="nw-asset" href="/api/v0/assets/y/content?b=1" data-nw-size="3" download="">y.zip</a></p>',
    revision: 1,
    assets_expire_at: null,
  });
  renderApp(pagePath(install.id), server.app, { enhancements: readingEnhancements });

  const article = await screen.findByRole("article");
  const shown = await within(article).findByRole("link", { name: "x.pdf (opens in a new tab)" });
  expect(shown.getAttribute("target")).toBe("_blank");
  expect(within(article).getByRole("link", { name: "y.zip" }).hasAttribute("target")).toBe(false);
  expect(article.textContent).toBe("x.pdf (opens in a new tab) (3 B) y.zip (3 B)");

  // In the reader's language, as it changes.
  act(() => server.app.preferences.setLocale("zh-CN"));
  await waitFor(() => expect(article.textContent).toBe("x.pdf （在新标签页打开）（3 B） y.zip（3 B）"));
});

/** childChanges watches element's subtree for nodes put in or taken out: it answers how many such changes came. */
function childChanges(element: Element): () => number {
  let changes = 0;
  const watching = new MutationObserver((records) => (changes += records.length));
  watching.observe(element, { childList: true, subtree: true });
  onTestFinished(() => watching.disconnect());
  return () => (changes += watching.takeRecords().length);
}

/**
 * writerWithEvents opens Install for an editor of Plans, a task item a and a link x in its view, through taskToggle
 * and recording(log): its stream connected, the view read again for it, the checkbox enabled. It answers the stream
 * and how the notebook changes, as the mode event has the list read again.
 */
async function writerWithEvents(log: string[]) {
  const events = eventServer();
  const server = pageServer({ role: "editor", answers: { "GET /api/v0/events": events.answer } });
  server.withTasks(install.id, "- [ ] a\n");
  server.views.set(install.id, {
    html: `${server.views.get(install.id)?.html ?? ""}<p><a href="/x">x</a></p>`,
    revision: 1,
    assets_expire_at: null,
  });
  const contexts: ReadingContext[] = [];
  const capture: Enhancement = (_container, context) => void contexts.push(context);
  renderApp(pagePath(install.id), withEvents(server.app), {
    enhancements: [taskToggle, recording(log, "a"), capture],
    eventHandlers,
  });
  const article = await screen.findByRole("article", { name: "Install" });
  const box = await within(article).findByRole<HTMLInputElement>("checkbox", { name: "a" });
  await waitFor(() => expect(box.disabled).toBe(false));
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const stream = events.last();
  act(() => stream.hello());
  // The connection reads the view again: the same HTML, which runs nothing again.
  await waitFor(() => expect(viewReads(server)).toBeGreaterThanOrEqual(2));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  const change = (notebook: Partial<typeof server.notebook>) => {
    server.notebook = { ...server.notebook, ...notebook };
    act(() => stream.send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id }));
  };
  return { server, article, box, change, contexts };
}

// M12/P2/S4, 48 (U7): whether the page can be written changes its checkboxes alone; the HTML stays, so do the
// reader's place and focus. The toggle asks again as it is clicked: the context's toggleTask is always there.
test("the notebook's upgrade starting writes no HTML: the checkboxes become disabled, a link keeps the focus", async () => {
  const log: string[] = [];
  const { server, article, box, change, contexts } = await writerWithEvents(log);
  const link = within(article).getByRole("link", { name: "x" });
  link.focus();
  const changes = childChanges(article);
  const runs = log.length;

  change({ upgrade: upgradeOf() });
  await waitFor(() => expect(box.disabled).toBe(true));
  expect(within(article).getByRole("checkbox", { name: "a" })).toBe(box);
  expect([changes(), log.length]).toEqual([0, runs]);
  expect(document.activeElement).toBe(link);
  // A tick asked for now writes nothing: whether the page can be written is read as it is clicked.
  await act(async () => {
    await contexts.at(-1)?.toggleTask?.(Number(box.dataset.task), true);
  });
  expect(server.sent.filter((line) => line.startsWith("TOGGLE"))).toEqual([]);

  change({ upgrade: null });
  await waitFor(() => expect(box.disabled).toBe(false));
  expect(box.getAttribute("aria-label")).toBe("a");
  expect([changes(), log.length]).toEqual([0, runs]);
  expect(document.activeElement).toBe(link);
});

// M12/P2/S4, 48 (U7): a page under the source root is a source, whose body no one writes (M10).
test("a page that becomes a source by the mode event, and a page again, has its checkboxes follow, the view's nodes kept", async () => {
  const log: string[] = [];
  const { article, box, change } = await writerWithEvents(log);
  const changes = childChanges(article);

  change({ mode: "llm_wiki", source_root_id: guide.id });
  await waitFor(() => expect(box.disabled).toBe(true));
  expect(within(article).getByRole("checkbox", { name: "a" })).toBe(box);
  expect(changes()).toBe(0);

  change({ mode: "standard", source_root_id: null });
  await waitFor(() => expect(box.disabled).toBe(false));
  expect(within(article).getByRole("checkbox", { name: "a" })).toBe(box);
  expect(changes()).toBe(0);
});

/**
 * readable opens Install, its view html, through enhancements and one that reads the view again: readAgain has it read
 * html at the next revision, and waits for its text.
 */
async function readable(html: string, enhancements: Enhancement[] = []) {
  const server = pageServer();
  server.views.set(install.id, { html, revision: 1, assets_expire_at: null });
  const reloads: (() => void)[] = [];
  const reading: Enhancement = (_container, context) => void reloads.push(context.reload);
  renderApp(pagePath(install.id), server.app, { enhancements: [...enhancements, reading] });
  const article = await screen.findByRole("article", { name: "Install" });
  await waitFor(() => expect(reloads).toHaveLength(1));
  let revision = 1;
  // The view read again shows next's text, or what its enhancements make of it (a code block's head).
  const readAgain = async (next: string, shown = next.replace(/<[^>]*>/g, "")) => {
    revision += 1;
    server.views.set(install.id, { html: next, revision, assets_expire_at: null });
    act(() => reloads.at(-1)?.());
    await waitFor(() => expect(article.textContent).toBe(shown));
  };
  return { server, article, readAgain };
}

// M12/P2/S4, 3 (U4): another's write is written where it changed the view, the rest kept as the reader has it.
/** written is a view of a link, a callout, a paragraph to select in, and last at the page's end. */
const written = (last: string) =>
  '<p><a href="/x">x</a></p>\n<details><summary>S</summary><p>in</p></details>\n<p>words to select</p>\n' +
  `<p id="last">${last}</p>\n`;

test("another's write elsewhere in the page keeps the focus on a link without an id, the selection in a paragraph and a callout opened", async () => {
  const { article, readAgain } = await readable(written("tail"));
  const link = within(article).getByRole("link", { name: "x" });
  const details = article.querySelector("details") as HTMLDetailsElement;
  details.open = true;
  // The focus first: jsdom moves the selection to what takes the focus.
  link.focus();
  const words = screen.getByText("words to select").firstChild as Text;
  document.getSelection()?.setBaseAndExtent(words, 0, words, 5);

  await readAgain(written("tail, edited"));

  expect(document.activeElement).toBe(link);
  expect(article.querySelector("details")).toBe(details);
  expect([details.open, details.isConnected]).toEqual([true, true]);
  expect(document.getSelection()?.anchorNode).toBe(words);
  expect(document.getSelection()?.toString()).toBe("words");
});

// M12/P2/S4, P2-21: a link's target made, its links far apart change at once; the blocks between are matched by
// what they are.
/** toX is a link to X, a page made or not. */
const toX = (made: boolean) =>
  made
    ? `<a class="nw-wikilink" href="${pagePath(guide.id)}" data-nw-node="${guide.id}">X</a>`
    : '<a class="nw-wikilink nw-unresolved" data-nw-target="X">X</a>';

/** farApart is a view with links to X at its ends, a callout and a paragraph between. */
const farApart = (made: boolean) =>
  `<p>a ${toX(made)}</p>\n<details><summary>Tip</summary><p>in</p></details>\n<p>mid</p>\n<p>b ${toX(made)}</p>\n`;

test("a write that changes two links far apart keeps the blocks between, a callout opened and the focus in them", async () => {
  const { article, readAgain } = await readable(farApart(false));
  const details = article.querySelector("details") as HTMLDetailsElement;
  const summary = details.querySelector("summary") as HTMLElement;
  details.open = true;
  summary.focus();

  await readAgain(farApart(true));

  expect(article.querySelectorAll("a[href]")).toHaveLength(2);
  expect(article.querySelector("details")).toBe(details);
  expect(details.querySelector("summary")).toBe(summary);
  expect(details.open).toBe(true);
  expect(document.activeElement).toBe(summary);
});

// M12/P2/S4, 8.2: the section the reader was in keeps its heading, renumbered; it is not the heading that now has
// its id.
test("a heading of the same name put before the focused one leaves the focus on the one the reader had", async () => {
  const { article, readAgain } = await readable('<h2 id="nw-notes">Notes</h2>\n<p>one</p>\n');
  const heading = section("Notes");
  heading.setAttribute("tabindex", "-1");
  heading.focus();

  await readAgain('<h2 id="nw-notes">Notes</h2>\n<p>zero</p>\n<h2 id="nw-notes-1">Notes</h2>\n<p>one</p>\n');

  expect(article.querySelectorAll("h2")).toHaveLength(2);
  expect(document.activeElement).toBe(heading);
  expect(heading.id).toBe("nw-notes-1");
  expect(heading.nextElementSibling?.textContent).toBe("one");
});

// M12/P2/S4, 12.2: a callout's summary has no id; written again, the callout's summary of the same text takes the
// focus, the callout as the view writes it.
test("a callout's summary that had the focus has it back, its callout written again by another", async () => {
  const { article, readAgain } = await readable("<details><summary>Tip</summary><p>a</p></details>\n");
  const before = article.querySelector("summary") as HTMLElement;
  before.focus();

  await readAgain("<details><summary>Tip</summary><p>b</p></details>\n");

  const summary = article.querySelector("summary");
  expect(summary).not.toBe(before);
  expect(document.activeElement).toBe(summary);
  expect(article.querySelector("details")?.open).toBe(false);
});

// M12/P2/S4, 48 (S1's 35.2): an LLM Wiki's index and log are pages a link to them goes to, by the notebook's mode.
// The mode changing, the enhancements run again on the HTML as it is (U5), as the language's does.
test("the notebook's mode changing runs the enhancements again without writing HTML", async () => {
  const events = eventServer();
  const server = pageServer({ role: "editor", answers: { "GET /api/v0/events": events.answer } });
  server.notebook = { ...server.notebook, upgrade: upgradeOf() };
  server.views.set(install.id, {
    html: '<p>See <a class="nw-wikilink nw-unresolved" data-nw-target="index">index</a></p>',
    revision: 1,
    assets_expire_at: null,
  });
  const log: string[] = [];
  renderApp(pagePath(install.id), withEvents(server.app), {
    enhancements: [unresolvedLinks, recording(log, "a")],
    eventHandlers,
  });
  const article = await screen.findByRole("article", { name: "Install" });
  const index = await within(article).findByText("index");
  await waitFor(() => expect(events.streams).toHaveLength(1));
  act(() => events.last().hello());
  await waitFor(() => expect(viewReads(server)).toBeGreaterThanOrEqual(2));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(index.getAttribute("role")).toBe("button");
  const changes = childChanges(article);
  const runs = log.length;

  // The upgrade approved: an LLM Wiki now.
  server.notebook = { ...server.notebook, mode: "llm_wiki", upgrade: null };
  act(() => events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id }));

  await waitFor(() => expect(index.getAttribute("role")).toBe("link"));
  expect(log.slice(runs).map((line) => line.split(" on ")[0])).toEqual(["undo a", "a"]);
  expect(within(article).getByText("index")).toBe(index);
  expect(changes()).toBe(0);
});

/** drawing is a view of before, a diagram's source in an opened callout, and a paragraph after it. */
const drawing = (before: string) =>
  `${before}<details open=""><summary>S</summary><pre><code class="language-mermaid">graph TD; s4-->drawn</code></pre></details>\n<p>after</p>\n`;

/** seen shows each diagram at once. */
const seen = (_element: Element, see: () => void) => {
  see();
  return () => undefined;
};

/** drawn opens html with a mermaid that draws at once, its drawings wider than they show, and the scroll regions'. */
async function drawn(html: string) {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    }
  );
  // Wide by its class, on the prototype: a wrapper drawn anew is wide as the scroll regions see it (review A's N13).
  const width = vi.spyOn(HTMLElement.prototype, "scrollWidth", "get").mockImplementation(function (this: HTMLElement) {
    return this.classList.contains("nw-diagram") ? 1000 : 0;
  });
  onTestFinished(() => width.mockRestore());
  const draw: Drawer = {
    initialize: () => undefined,
    detectType: () => "flowchart",
    render: async (id, text) => ({ svg: `<svg id="${id}"><text>${text}</text></svg>` }),
  };
  const opened = await readable(html, [diagrams(async () => draw, seen), scrollRegions]);
  const wrapper = () => opened.article.querySelector<HTMLElement>(".nw-diagram");
  await waitFor(() => expect(wrapper()?.getAttribute("tabindex")).toBe("0"));
  return { ...opened, wrapper };
}

// M12/P2/S4, 11.6 (review A's M2, revision check A's M12): a drawing's wrapper is drawn anew as the enhancements run
// again; the scroll region of the block it is in, the same callout, has the focus back. In a callout, not at the
// top: there the block where it was would be the new wrapper itself.
test("a drawing in a callout that had the focus has it back after the language changes", async () => {
  const { server, article, wrapper } = await drawn(drawing(""));
  const callout = article.querySelector("details");
  const before = wrapper();
  before?.focus();

  act(() => server.app.preferences.setLocale("zh-CN"));

  await waitFor(() => expect(wrapper()).not.toBe(before));
  expect(article.querySelector("details")).toBe(callout);
  expect(document.activeElement).toBe(wrapper());
  expect(callout?.contains(document.activeElement)).toBe(true);
});

// M12/P2/S4, 4.4.3 (revision check A's M11): the block it was in, found through the patch's places; by its place
// before, the paragraph now there.
test("a drawing that had the focus has it back as a paragraph is put before its block", async () => {
  const { article, wrapper, readAgain } = await drawn(drawing("<p>zero</p>\n"));
  const callout = article.querySelector("details");
  const before = wrapper();
  before?.focus();

  await readAgain(drawing("<p>new</p>\n<p>zero</p>\n"));

  await waitFor(() => expect(wrapper()?.getAttribute("tabindex")).toBe("0"));
  expect(article.querySelector("details")).toBe(callout);
  expect(document.activeElement).toBe(wrapper());
  expect(callout?.contains(document.activeElement)).toBe(true);
});

/** twice is a view of first, a paragraph, and a link to x far below, each laid out at its data-at. */
const twice = (first: string, edit = "") =>
  `<p data-at="100">${first}</p>\n<p data-at="200">middle${edit}</p>\n<p data-at="3000">far, see <a href="/x">x</a>${edit}</p>\n`;

// M12/P2/S4, 4.4.3's step 3 (review A's M4): a link a page has twice, far apart; the reader at the first. Another
// rewrites its paragraph without it: the focus is the paragraph's, where it was, not the far link's, and the view
// stays where it is.
test("a link whose paragraph is written without it gives the focus to the paragraph, not to a link of its key far off", async () => {
  const scroll = vi.fn();
  Element.prototype.scrollIntoView = scroll;
  const laidOut = vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    const top = Number(this.closest("[data-at]")?.getAttribute("data-at") ?? 0);
    return { top, bottom: top + 20, left: 0, right: 100 } as DOMRect;
  });
  onTestFinished(() => laidOut.mockRestore());
  const { article, readAgain } = await readable(twice('see <a href="/x">x</a> here'));
  const [near, far] = article.querySelectorAll("a");
  near?.focus();
  scroll.mockClear();

  await readAgain(twice("rewritten, no link"));

  expect(document.activeElement).not.toBe(far);
  expect(document.activeElement).toBe(article.querySelector("p"));
  expect(scroll).not.toHaveBeenCalled();
});

/** twoBlocks is a view of a paragraph, then two code blocks, the second's code second. */
const twoBlocks = (first: string, second: string) =>
  `<p>${first}</p>\n<pre><code>make build</code></pre>\n<pre><code>${second}</code></pre>\n`;

/** copyable reads a view through the app's enhancements, the page given a clipboard: each code block has its Copy. */
async function copyable(html: string) {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    }
  );
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: () => Promise.resolve() } });
  onTestFinished(() => void Reflect.deleteProperty(navigator, "clipboard"));
  const read = await readable(html, [...readingEnhancements]);
  const copies = () => within(read.article).getAllByRole("button", { name: "Copy" });
  await waitFor(() => expect(copies()).toHaveLength(2));
  return { ...read, copies };
}

/** shownOf is what a view of twoBlocks shows, each code block's head before its code. */
const shownOf = (first: string, second: string) => `${first}\nCopymake build\nCopy${second}\n`;

// M13/P4/S2 (decision 869; review C's M9): a code block's head is made anew as the view is written, its block kept or
// not; the Copy that had the focus has it back, found by which of its block's it is.
test("a code block's Copy that had the focus has it back as its block is written anew by another", async () => {
  const { article, readAgain, copies } = await copyable(twoBlocks("one", "go test"));
  const before = copies()[1] as HTMLElement;
  before.focus();

  await readAgain(twoBlocks("one", "go vet"), shownOf("one", "go vet"));

  const blocks = article.querySelectorAll("pre");
  expect(blocks[1]?.textContent).toBe("Copygo vet");
  expect(document.activeElement).not.toBe(before);
  expect(document.activeElement).toBe(blocks[1]?.querySelector("button.nw-code-copy"));
});

test("a code block's Copy that had the focus has it back, made anew, as another block is written by another", async () => {
  const { article, readAgain, copies } = await copyable(twoBlocks("one", "go test"));
  const kept = article.querySelectorAll("pre")[1];
  const before = copies()[1] as HTMLElement;
  before.focus();

  await readAgain(twoBlocks("two", "go test"), shownOf("two", "go test"));

  const blocks = article.querySelectorAll("pre");
  // The block kept, its head made anew: one head in each block, the focus on the new Copy of the second.
  expect(blocks[1]).toBe(kept);
  expect([...blocks].map((block) => block.querySelectorAll(".nw-code-head").length)).toEqual([1, 1]);
  expect(before.isConnected).toBe(false);
  expect(document.activeElement).toBe(blocks[1]?.querySelector("button.nw-code-copy"));
});

/** sized has the ResizeObserver tell each element it observes its size as it starts, as a browser does at the next rendering. */
function sized() {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      private readonly told: ResizeObserverCallback;
      private observing = true;
      constructor(told: ResizeObserverCallback) {
        this.told = told;
      }
      observe(element: Element) {
        setTimeout(() => {
          if (this.observing) {
            this.told([{ target: element } as ResizeObserverEntry], this as unknown as ResizeObserver);
          }
        }, 10);
      }
      disconnect() {
        this.observing = false;
      }
    }
  );
}

/** beforeCode is a view of first, then a code block and a paragraph. */
const beforeCode = (first: string) => `${first}<pre><code>make build</code></pre>\n<p>end</p>\n`;

/** told lets the resize observers tell what they observe. */
const told = () => act(() => new Promise((resolve) => setTimeout(resolve, 50)));

// M12/P2/S4, 4.4.3's step 4 (review A's M3): the block the focus is given back to may be a code block that scrolls no
// more than it shows; the scroll regions, told its size, take away only the tabindex they gave, not the -1 it was given.
test("the code block the focus is given back to keeps it as the scroll regions are told it is not wide", async () => {
  sized();
  const { article, readAgain } = await readable(beforeCode('<p>see <a href="/x">x</a></p>\n'), [scrollRegions]);
  await told();
  within(article).getByRole("link", { name: "x" }).focus();

  await readAgain(beforeCode(""));
  const pre = article.querySelector("pre") as HTMLElement;
  await told();

  expect(document.activeElement).toBe(pre);
  expect(pre.getAttribute("tabindex")).toBe("-1");
});

test("the article the focus is given back to, the page emptied, keeps it as the scroll regions are told it is not wide", async () => {
  sized();
  const { article, readAgain } = await readable('<p>see <a href="/x">x</a></p>\n', [scrollRegions]);
  await told();
  within(article).getByRole("link", { name: "x" }).focus();

  await readAgain("");
  await told();

  expect(document.activeElement).toBe(article);
  expect(article.getAttribute("tabindex")).toBe("-1");
});

/**
 * wentBack opens Install at its anchor's heading A, goes to the page without one and back, the next frame held:
 * then meanwhile does what it is given (the browser's scroll put back, the reader's key, the focus moved), with A
 * laid out at top, and lets the frame come. It answers the elements scrolled into view as it opened and came back,
 * and since.
 */
async function wentBack(top: number, meanwhile: (heading: HTMLElement) => void) {
  const scrolled = scrolls();
  const frames: FrameRequestCallback[] = [];
  const frame = vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => frames.push(callback));
  onTestFinished(() => frame.mockRestore());
  const server = pageServer();
  server.views.set(install.id, { html: '<h2 id="nw-a">A</h2>', revision: 1, assets_expire_at: null });
  const { router } = renderApp(`${pagePath(install.id)}#nw-a`, server.app);
  const a = await screen.findByRole("heading", { level: 2, name: "A" });
  await act(() => router.navigate(pagePath(install.id)));
  frames.length = 0;
  await act(() => router.navigate(-1));
  expect(document.activeElement).toBe(a);
  const back = scrolled.length;

  const laidOut = vi.spyOn(a, "getBoundingClientRect").mockReturnValue({ top, bottom: top + 20 } as DOMRect);
  onTestFinished(() => laidOut.mockRestore());
  meanwhile(a);
  act(() => {
    for (const callback of frames.splice(0)) {
      callback(0);
    }
  });
  return { a, back: scrolled.slice(0, back), since: scrolled.slice(back) };
}

/** scrolledBy has the window scrolled down by y, as the browser puts back a history entry's scroll. */
function scrolledBy(y: number) {
  const original = Object.getOwnPropertyDescriptor(window, "scrollY");
  Object.defineProperty(window, "scrollY", { configurable: true, get: () => y });
  onTestFinished(() => {
    if (original) {
      Object.defineProperty(window, "scrollY", original);
    }
  });
}

// M12/P2/S4, 8.1 (review C's M2): back at an address with an anchor, the browser puts back the scroll the page had,
// after the view went to it. At the next frame, the window scrolled since, the heading with the focus still, the
// reader having done nothing, and it out of view, the view shows it again; not one of these, it does not.
test("back at an address with an anchor, its heading out of view as the window was scrolled since, is shown again at the next frame", async () => {
  const { a, back, since } = await wentBack(2000, () => scrolledBy(1500));

  expect(back).toEqual([a, a]);
  expect(since).toEqual([a]);
});

test.each([
  ["the window not scrolled since", 2000, () => undefined],
  [
    "the reader pressed a key meanwhile",
    2000,
    () => {
      scrolledBy(1500);
      fireEvent.keyDown(window, { key: "ArrowDown" });
    },
  ],
  [
    "the focus moved elsewhere meanwhile",
    2000,
    (heading: HTMLElement) => {
      scrolledBy(1500);
      heading.blur();
    },
  ],
  ["its heading in view", 100, () => scrolledBy(1500)],
] as const)(
  "back at an address with an anchor, the view does not show it again at the next frame: %s",
  async (_, top, meanwhile) => {
    const { a, back, since } = await wentBack(top, meanwhile);

    expect(back).toEqual([a, a]);
    expect(since).toEqual([]);
  }
);

// M12/P2/S4, 4.4.3's step 3 (review A's C1-M1): one save puts a paragraph before the reader's and changes the
// reader's, its link kept. The reader's block is placed at the paragraph put in first; the link is in the other put
// in where it was, next to it.
test("a link whose paragraph changed, a paragraph put before it in the same save, has the focus back in its paragraph", async () => {
  const { article, readAgain } = await readable('<p>start</p>\n<p>see <a href="/x">x</a> here</p>\n<p>end</p>\n');
  within(article).getByRole("link", { name: "x" }).focus();

  await readAgain('<p>start</p>\n<p>a new paragraph</p>\n<p>see <a href="/x">x</a> here, edited</p>\n<p>end</p>\n');

  expect(document.activeElement).toBe(within(article).getByRole("link", { name: "x" }));
  expect(document.activeElement?.parentElement?.textContent).toBe("see x here, edited");
});

// M12/P2/S4, 4.4.3's step 3 (review A's C2-M1, its probe W1): one save rewrites every block from the reader's to the
// far one, the reader's without its link, as many as there were. The gap reaches the far link; the reader's new
// version is no farther than the next block, so the focus is the paragraph's and the view stays where it is.
test("a link whose paragraph is written without it, the blocks to a far link of its key changed in the same save, gives the focus to the paragraph", async () => {
  const scroll = vi.fn();
  Element.prototype.scrollIntoView = scroll;
  const laidOut = vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    const top = Number(this.closest("[data-at]")?.getAttribute("data-at") ?? 0);
    return { top, bottom: top + 20, left: 0, right: 100 } as DOMRect;
  });
  onTestFinished(() => laidOut.mockRestore());
  const { article, readAgain } = await readable(twice('see <a href="/x">x</a> here'));
  article.querySelector("a")?.focus();
  scroll.mockClear();

  await readAgain(twice("rewritten, no link", " (edited)"));

  expect(document.activeElement).not.toBe(article.querySelector("a"));
  expect(document.activeElement).toBe(article.querySelector("p"));
  expect(scroll).not.toHaveBeenCalled();
});
