import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { readingEnhancements, type Enhancement } from "../../reading/enhancement";
import { notebookJSON } from "../../test/fakes";
import { guide, install, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The reading view's enhancements (M4/P5 design 3.8).

afterEach(() => {
  vi.useRealTimers();
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

test("an anchor the view has no element of goes nowhere", async () => {
  const scrolled = scrolls();
  renderApp(`${pagePath(install.id)}#nw-%E0%A4`, pageServer().app);
  expect((await screen.findByRole("article")).innerHTML).toBe("<p>Install</p>");
  expect(scrolled).toEqual([]);
});

test("a link to a page goes there through the router, with the app's enhancements, arriving at the page unless at an anchor", async () => {
  scrolls();
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
