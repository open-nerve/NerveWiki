import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import { readingEnhancements, type Enhancement, type ReadingContext } from "../../reading/enhancement";
import { json, notebookJSON } from "../../test/fakes";
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

  server.views.set(install.id, { html: "<p>Install, changed</p>", revision: 2, assets_expire_at: null });
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
  server.views.set(install.id, { html: "<p>Install, again</p>", revision: 2, assets_expire_at: null });

  act(() => reloads[0]?.());

  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Install, again</p>"));
  expect(server.sent.filter((line) => line === "GET view Install")).toHaveLength(2);
});

test("a change of the theme runs no enhancement again, those that follow it hearing it; one of the language runs them again", async () => {
  const server = pageServer();
  const log: string[] = [];
  const heard: string[] = [];
  renderApp(pagePath(install.id), server.app, {
    enhancements: [
      recording(log, "a"),
      (_container, { theme, onThemeChange }) => onThemeChange(() => heard.push(theme())),
    ],
  });
  await waitFor(() => expect(log).toHaveLength(1));

  act(() => server.app.preferences.setTheme("dark"));
  expect(heard).toEqual(["dark"]);
  expect(log).toHaveLength(1);

  act(() => server.app.preferences.setLocale("zh-CN"));
  await waitFor(() => expect(log).toHaveLength(3));
  expect(log[1]).toBe("undo a on <p>Install</p>");
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
  // The inputs listened to in the capture phase: the wait's.
  const inputs = (spy: typeof added | typeof removed) =>
    spy.mock.calls
      .filter(([type]) => ["wheel", "touchmove", "pointerdown", "keydown"].includes(type))
      .filter(([, , options]) => options === true || (typeof options === "object" && options.capture === true))
      .map(([type]) => type);
  const { server, release } = heldServer(["<p>Install</p>", '<h2 id="nw-x">X</h2>']);
  await openedFromTheCache(server, "#nw-x");
  await waitFor(() => expect(inputs(added)).toHaveLength(4));
  expect(inputs(removed)).toEqual([]);

  await act(async () => release());
  await screen.findByRole("heading", { level: 2, name: "X" });
  expect(new Set(inputs(removed))).toEqual(new Set(inputs(added)));
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

  await act(() => router.navigate(pagePath(guide.id)));
  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Guide</p>"));
  await act(() => vi.advanceTimersByTimeAsync(6 * 60_000));
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

test("an enhancement has the view's expiry, and an attachment's address signed anew; one gone, it rejects", async () => {
  const server = pageServer();
  const sound = { ...assetNode(90, "a.mp3"), parent_id: install.id };
  server.nodes = [...server.nodes, sound];
  server.views.set(install.id, { html: "<p>Install</p>", revision: 1, assets_expire_at: "2100-01-01T00:00:00Z" });
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
});
