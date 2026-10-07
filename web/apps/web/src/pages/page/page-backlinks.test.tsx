import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import { json, problem } from "../../test/fakes";
import { panel, readAgain, section, shownPanel } from "../../test/page-panel";
import { guide, install, linux, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { readersInput } from "./readers-input";

// The right column's backlinks (M6/P7 design 9).

afterEach(() => {
  vi.useRealTimers();
});

/** backlinked is what the backlinks list: for each page, its link's text, how many links, and its lines. */
function backlinked(): (string | null)[][] {
  return [...section("Backlinks").querySelectorAll(":scope > ul > li")].map((item) =>
    [...item.querySelectorAll("a, span, li")].map((each) => each.textContent)
  );
}

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

test("pages of one title that link here are named as the tree names them, by where each is", async () => {
  // A second Install, at the root: the tree tells the two apart.
  const other = pageNode(5, "Install");
  const server = pageServer({ nodes: [guide, install, linux, notes, other] });
  server.backlinks.set(notes.id, [
    {
      data: [
        { id: install.id, count: 1, contexts: [] },
        { id: other.id, count: 1, contexts: [] },
      ],
      next_cursor: null,
    },
  ]);
  renderApp(pagePath(notes.id), server.app);

  const list = await within(await shownPanel()).findByRole("list", { name: "Backlinks" });
  const links = within(list).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href")])).toEqual([
    ["Install (in Guide)", pagePath(install.id)],
    ["Install (in Plans)", pagePath(other.id)],
  ]);
});

test("backlinks that could not be read say why, and are read again on Try again", async () => {
  let fail = true;
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/backlinks": () => (fail ? failing() : json(backlinkPage(guide.id, 1, null))),
    },
  });
  renderApp(pagePath(install.id), server.app);

  const retry = await within(await shownPanel()).findByRole("button", { name: "Try again" });
  expect(within(section("Backlinks")).getByRole("alert").textContent).not.toBe("");
  fail = false;
  await userEvent.click(retry);
  expect(await within(section("Backlinks")).findByRole("link", { name: "Guide" })).toBeTruthy();
  expect(within(section("Backlinks")).queryByRole("alert")).toBeNull();
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

test("the last more leaves the focus where the reader put it meanwhile, or left it doing something: on another link, on the content clicked, on More scrolled away by the wheel or a key, the column pressed elsewhere", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  // What the reader does as More reads, and where the focus is left.
  for (const elsewhere of [
    (guideLink: HTMLElement) => {
      act(() => guideLink.focus());
      return guideLink;
    },
    async () => {
      await user.click(screen.getByRole("article"));
      return document.body;
    },
    // More has the focus still, which goes with it.
    (_guideLink: HTMLElement, more: HTMLElement) => {
      fireEvent.wheel(more);
      return document.body;
    },
    (_guideLink: HTMLElement, more: HTMLElement) => {
      fireEvent.keyDown(more, { key: "PageDown" });
      return document.body;
    },
    // The middle button's press, which scrolls as the pointer moves; the right one's.
    (_guideLink: HTMLElement, more: HTMLElement) => {
      fireEvent.pointerDown(more, { button: 1 });
      return document.body;
    },
    (_guideLink: HTMLElement, more: HTMLElement) => {
      fireEvent.pointerDown(more, { button: 2 });
      return document.body;
    },
    () => {
      fireEvent.pointerDown(within(panel()).getByRole("heading", { level: 2, name: "Backlinks" }));
      return document.body;
    },
  ]) {
    const { server, held } = heldMore();
    const { unmount } = renderApp(pagePath(install.id), server.app);
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    const guideLink = await within(await shownPanel()).findByRole("link", { name: "Guide" });

    const more = within(section("Backlinks")).getByRole("button", { name: "More backlinks" });
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    await user.click(more);
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    const left = await elsewhere(guideLink, more);
    held.answer?.();
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    await within(section("Backlinks")).findByRole("link", { name: "Notes" });
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(document.activeElement).toBe(left);
    unmount();
  }
});

test("a last more clicked as Safari does, which gives More no focus, takes the focus to the first page it adds, without a scroll", async () => {
  const { server, held } = heldMore();
  renderApp(pagePath(install.id), server.app);
  const more = await within(await shownPanel()).findByRole("button", { name: "More backlinks" });
  const focus = vi.spyOn(HTMLElement.prototype, "focus");
  const added = vi.spyOn(window, "addEventListener");
  const removed = vi.spyOn(window, "removeEventListener");
  onTestFinished(() => void vi.restoreAllMocks());

  fireEvent.click(more);
  expect(document.activeElement).toBe(document.body);
  await waitFor(() => expect(held.answer).toBeDefined());
  held.answer?.();
  const addedPage = await within(section("Backlinks")).findByRole("link", { name: "Notes" });
  await waitFor(() => expect(document.activeElement).toBe(addedPage));
  expect(focus).toHaveBeenLastCalledWith({ preventScroll: true });
  // What the reader did as More read is watched no more.
  for (const type of readersInput) {
    const of = (spy: typeof added | typeof removed) =>
      spy.mock.calls.filter(([each]) => each === type).map(([, listener]) => listener);
    expect(of(added)).toHaveLength(1);
    expect(of(removed)).toEqual(of(added));
  }
});

test("More pressed again as it reads, a key on it that presses it or moves nothing, a touch, a double click, is no move elsewhere: the last takes the focus to the first page it adds", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  /** again presses More with Enter, then, as it reads, as fire does. */
  const again =
    (fire: (more: HTMLElement) => void) => async (more: HTMLElement, held: { answer: (() => void) | undefined }) => {
      more.focus();
      await user.keyboard("{Enter}");
      await waitFor(() => expect(held.answer).toBeDefined());
      fire(more);
    };
  for (const press of [
    ...[" ", "Escape", "Shift", "Control", "Alt", "Meta"].map((key) =>
      again((more) => fireEvent.keyDown(more, { key }))
    ),
    again((more) => fireEvent.pointerDown(more, { button: 0, pointerType: "touch" })),
    async (more: HTMLElement, held: { answer: (() => void) | undefined }) => {
      more.focus();
      await user.keyboard("{Enter}");
      await waitFor(() => expect(held.answer).toBeDefined());
      await user.keyboard("{Enter}");
    },
    async (more: HTMLElement, held: { answer: (() => void) | undefined }) => {
      await user.dblClick(more);
      await waitFor(() => expect(held.answer).toBeDefined());
    },
  ]) {
    const { server, held } = heldMore();
    const { unmount } = renderApp(pagePath(install.id), server.app);
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    const more = await within(await shownPanel()).findByRole("button", { name: "More backlinks" });
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    await press(more, held);
    held.answer?.();
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    const added = await within(section("Backlinks")).findByRole("link", { name: "Notes" });
    // oxlint-disable-next-line no-await-in-loop -- one app after another
    await waitFor(() => expect(document.activeElement).toBe(added));
    unmount();
  }
});

test("More pressed again as it reads reads nothing more: the next page is read once, More busy until it answers", async () => {
  const user = userEvent.setup();
  const { server, held } = heldMore();
  renderApp(pagePath(install.id), server.app);
  const more = await within(await shownPanel()).findByRole("button", { name: "More backlinks" });

  await user.click(more);
  await waitFor(() => expect(held.answer).toBeDefined());
  // A read it started now would be answered at once.
  await user.click(more);
  await act(async () => {});
  expect(more.getAttribute("aria-busy")).toBe("true");
  held.answer?.();
  await within(section("Backlinks")).findByRole("link", { name: "Notes" });
  expect(held.reads).toEqual([null, "1"]);
});

test("the focus the reader moves as the list grows stays where they put it", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const { server, held } = heldMore();
  renderApp(pagePath(install.id), server.app);
  const guideLink = await within(await shownPanel()).findByRole("link", { name: "Guide" });
  within(section("Backlinks")).getByRole("button", { name: "More backlinks" }).focus();
  await user.keyboard("{Enter}");
  await waitFor(() => expect(held.answer).toBeDefined());
  // As the list shows Notes, before the focus would fall there, the reader goes to Guide.
  const observer = new MutationObserver(() => {
    if (section("Backlinks").textContent?.includes("Notes") && document.activeElement !== guideLink) {
      guideLink.focus();
      observer.disconnect();
    }
  });
  observer.observe(section("Backlinks"), { childList: true, subtree: true });
  held.answer?.();
  await within(section("Backlinks")).findByRole("link", { name: "Notes" });
  await act(() => vi.advanceTimersByTimeAsync(50));
  expect(document.activeElement).toBe(guideLink);
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

test("a more answered as the page come back to reads its list again has the list read again whole: SWR drops that read", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  let links = 1;
  let holding = false;
  const answers: (() => void)[] = [];
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/backlinks": (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        const page = () => json(cursor === null ? backlinkPage(guide.id, links, "1") : backlinkPage(notes.id, 1, null));
        return holding && request.url.includes(install.id)
          ? new Promise((resolve) => answers.push(() => resolve(page())))
          : page();
      },
    },
  });
  const { router } = renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  holding = true;
  await user.click(within(section("Backlinks")).getByRole("button", { name: "More backlinks" }));

  await act(() => router.navigate(pagePath(notes.id)));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  // Past SWR's deduping: the page come back to reads its list again.
  await act(() => vi.advanceTimersByTimeAsync(3_000));
  await act(() => router.navigate(pagePath(install.id)));
  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  await waitFor(() => expect(answers).toHaveLength(2));
  links = 5;
  holding = false;
  // More answers first: the read of the page come back to is dropped.
  answers[0]?.();
  await waitFor(() => expect(backlinked()).toEqual([["Guide", " · 5 links"], ["Notes"]]));
  answers[1]?.();
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
