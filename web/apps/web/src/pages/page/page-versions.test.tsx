import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { json, problem } from "../../test/fakes";
import { readAgain, section, shownPanel } from "../../test/page-panel";
import { ada, bob, install, notes, pagePath, pageServer, versionJSON } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The right column's versions (M8 design 4.3, 4.13).

afterEach(() => {
  vi.useRealTimers();
});

/** now is when the tests read the versions: their times are as long ago as they are from it. */
const now = new Date("2026-10-03T10:00:00Z");

/** listed is what the versions list: for each, its link's text and title, its marks, and who wrote it and what. */
function listed(): string[][] {
  return [...section("Versions").querySelectorAll(":scope > ul > li")].map((item) => {
    const time = item.querySelector("a time");
    const marks = [...item.querySelectorAll("span")].map((mark) => mark.textContent ?? "");
    return [time?.textContent ?? "", time?.getAttribute("title") ?? ""].concat(marks, [
      item.querySelector("p")?.textContent ?? "",
    ]);
  });
}

/** many is count versions of the page, each written on the one before, its revision its number. */
function many(count: number) {
  return Array.from({ length: count }, (_, i) =>
    versionJSON(i + 1, i === 0 ? null : i, `Install ${(i + 1).toString()}\n`, { at: "2026-10-01T09:00:00Z" })
  );
}

test("the versions are the page's, newer first: as long ago as each was written, the time in full as its title, who wrote it and what; the newest current, one being edited marked; each goes to its version's page", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(now);
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = pageServer();
  server.withVersions(install.id, [
    versionJSON(1, null, "Install\n", { kind: "import", by: bob, at: "2026-09-19T10:00:00Z" }),
    versionJSON(3, 1, "Install\nsteps\n", { at: "2026-10-01T09:00:00Z" }),
    versionJSON(5, 3, "Install\n", { kind: "version_restore", at: "2026-10-02T09:00:00Z" }),
    versionJSON(6, 5, "Install\nback\n", { kind: "trash_restore", at: "2026-10-03T08:30:00Z" }),
    versionJSON(7, 6, "Install\n", { kind: "revert", at: "2026-10-03T09:00:00Z" }),
    versionJSON(9, 7, "Install\nnew\n", { by: bob, at: "2026-10-03T09:58:00Z", editing: true }),
  ]);
  const { router } = renderApp(pagePath(install.id), server.app);

  const list = await within(await shownPanel()).findByRole("list", { name: "Versions" });
  expect(within(section("Versions")).getByText("One version per edit.")).toBeTruthy();
  expect(listed()).toEqual([
    ["2 minutes ago", "Oct 3, 2026, 9:58 AM", "Current", "Editing", "Bob · Edit"],
    ["1 hour ago", "Oct 3, 2026, 9:00 AM", "Ada · Undo change"],
    ["1 hour ago", "Oct 3, 2026, 8:30 AM", "Ada · Restore from trash"],
    ["yesterday", "Oct 2, 2026, 9:00 AM", "Ada · Restore"],
    ["2 days ago", "Oct 1, 2026, 9:00 AM", "Ada · Edit"],
    ["2 weeks ago", "Sep 19, 2026, 10:00 AM", "Bob · Import"],
  ]);
  const links = within(list).getAllByRole("link");
  expect(links.map((link) => link.getAttribute("href"))).toEqual(
    [9, 7, 6, 5, 3, 1].map((revision) => `${pagePath(install.id)}/versions/${revision.toString()}`)
  );
  // Named as they read, the version and the time in full telling apart those of one hour.
  const times = listed().map(([, title]) => title ?? "");
  expect(within(list).getByRole("link", { name: `1 hour ago, version 7, ${times[1] ?? ""}` })).toBe(links[1]);
  expect(within(list).getByRole("link", { name: `1 hour ago, version 6, ${times[2] ?? ""}` })).toBe(links[2]);
  expect(within(section("Versions")).queryByRole("button", { name: "More versions" })).toBeNull();

  await user.click(within(list).getByRole("link", { name: /^yesterday,/ }));
  const heading = await screen.findByRole("heading", { level: 1, name: "Version 5" });
  expect(router.state.location.pathname).toBe(`${pagePath(install.id)}/versions/5`);
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

test("in Chinese, the versions say the same", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(now);
  const server = pageServer();
  server.withVersions(install.id, [
    versionJSON(1, null, "Install\n", { kind: "import", by: bob, at: "2026-10-02T10:00:00Z" }),
    versionJSON(2, 1, "Install\nmore\n", { kind: "revert", by: ada, at: "2026-10-03T09:55:00Z", editing: true }),
  ]);
  server.app.preferences.setLocale("zh-CN");
  renderApp(pagePath(install.id), server.app);

  await within(await screen.findByRole("complementary", { name: "页面信息" })).findByRole("list", { name: "版本" });
  expect(within(screen.getByRole("complementary", { name: "页面信息" })).getByText("一次编辑一个版本。")).toBeTruthy();
  const items = within(screen.getByRole("list", { name: "版本" })).getAllByRole("listitem");
  expect(items.map((item) => item.textContent)).toEqual(["5分钟前当前编辑中Ada · 撤销", "昨天Bob · 导入"]);
});

test("more reads the next page of twenty and adds it, the focus kept on it until the last, which takes it to the first version it adds; read again, or come back to, the list is as many pages, from the first", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(now);
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = pageServer();
  server.withVersions(install.id, many(45));
  const { router } = renderApp(pagePath(install.id), server.app);
  const list = await within(await shownPanel()).findByRole("list", { name: "Versions" });
  expect(within(list).getAllByRole("listitem")).toHaveLength(20);
  const more = within(section("Versions")).getByRole("button", { name: "More versions" });

  more.focus();
  let before = server.sent.length;
  await user.keyboard("{Enter}");
  await waitFor(() => expect(within(list).getAllByRole("listitem")).toHaveLength(40));
  await act(() => vi.advanceTimersByTimeAsync(0));
  expect(document.activeElement).toBe(more);
  expect(server.sent.slice(before).filter((each) => each.startsWith("GET versions"))).toEqual([
    "GET versions Install 26",
  ]);

  await user.keyboard("{Enter}");
  await waitFor(() => expect(within(list).getAllByRole("listitem")).toHaveLength(45));
  expect(within(section("Versions")).queryByRole("button", { name: "More versions" })).toBeNull();
  const added = within(list).getAllByRole("link")[40];
  expect(added?.getAttribute("href")).toBe(`${pagePath(install.id)}/versions/5`);
  await waitFor(() => expect(document.activeElement).toBe(added));

  // Come back to, the page shows the versions read, and reads as many.
  await act(() => router.navigate(pagePath(notes.id)));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  // Past SWR's deduping of the reads it starts.
  await act(() => vi.advanceTimersByTimeAsync(3_000));
  before = server.sent.length;
  await act(() => router.navigate(pagePath(install.id)));
  await within(await shownPanel()).findByRole("list", { name: "Versions" });
  await waitFor(() =>
    expect(server.sent.slice(before).filter((each) => each.startsWith("GET versions Install"))).toEqual([
      "GET versions Install",
      "GET versions Install 26",
      "GET versions Install 6",
    ])
  );
  expect(within(section("Versions")).getAllByRole("listitem")).toHaveLength(45);

  // Read again, a version written elsewhere shows, the list as long as it was and one more.
  server.withVersions(install.id, [
    ...(server.versions.get(install.id) ?? []),
    versionJSON(46, 45, "Install 46\n", { by: bob, at: "2026-10-03T09:59:00Z" }),
  ]);
  before = server.sent.length;
  await readAgain();
  await waitFor(() => expect(within(section("Versions")).getAllByRole("listitem")).toHaveLength(46));
  expect(listed()[0]).toEqual(["1 minute ago", "Oct 3, 2026, 9:59 AM", "Current", "Bob · Edit"]);
  expect(server.sent.slice(before).filter((each) => each.startsWith("GET versions"))).toEqual([
    "GET versions Install",
    "GET versions Install 27",
    "GET versions Install 7",
  ]);
});

/** listing is a page of the versions of revisions, as the list answers it, written by, the next after cursor. */
function listing(revisions: number[], cursor: string | null, by = ada) {
  return {
    data: revisions.map((revision) => {
      const { content: _, ...version } = versionJSON(revision, revision - 1, "", { by, at: "2026-10-01T09:00:00Z" });
      return version;
    }),
    next_cursor: cursor,
  };
}

/** writers is who wrote each version listed. */
const writers = () =>
  within(section("Versions"))
    .getAllByRole("listitem")
    .map((item) => item.querySelector("p")?.textContent);

test("a More answered as the page come back to reads its list again has the list read again whole: SWR drops that read", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  let by = ada;
  let holding = false;
  const answers: (() => void)[] = [];
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/versions": (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        const page = () => json(cursor === null ? listing([5], "5", by) : listing([4], null, by));
        return holding && request.url.includes(install.id)
          ? new Promise((resolve) => answers.push(() => resolve(page())))
          : page();
      },
    },
  });
  const { router } = renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("list", { name: "Versions" });
  holding = true;
  await user.click(within(section("Versions")).getByRole("button", { name: "More versions" }));

  await act(() => router.navigate(pagePath(notes.id)));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  // Past SWR's deduping: the page come back to reads its list again.
  await act(() => vi.advanceTimersByTimeAsync(3_000));
  await act(() => router.navigate(pagePath(install.id)));
  await within(await shownPanel()).findByRole("list", { name: "Versions" });
  await waitFor(() => expect(answers).toHaveLength(2));
  by = bob;
  holding = false;
  // More answers first: the read of the page come back to is dropped.
  answers[0]?.();
  await waitFor(() => expect(writers()).toEqual(["Bob · Edit", "Bob · Edit"]));
  answers[1]?.();
});

test("a next page added while the list is read again has the list read again whole: SWR drops that read", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  let first = listing([5], "5");
  let held = Promise.resolve();
  let answer!: (response: Response) => void;
  let next = (): Promise<Response> => new Promise((resolve) => (answer = resolve));
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/versions": async (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        server.sent.push(`GET versions ${cursor ?? ""}`.trim());
        if (cursor !== null) {
          return next();
        }
        await held;
        return json(first);
      },
    },
  });
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("list", { name: "Versions" });
  const more = within(section("Versions")).getByRole("button", { name: "More versions" });
  await user.click(more);
  await waitFor(() => expect(more.getAttribute("aria-busy")).toBe("true"));

  // The list is read again, its first page's answer held, as the next page comes: SWR drops that read.
  let release!: () => void;
  held = new Promise((resolve) => (release = resolve));
  first = listing([5], "5", bob);
  await readAgain();
  await waitFor(() => expect(server.sent.filter((each) => each === "GET versions")).toHaveLength(2));
  next = () => Promise.resolve(json(listing([4], null, bob)));
  answer(json(listing([4], null)));
  await waitFor(() => expect(writers()).toEqual(["Ada · Edit", "Ada · Edit"]));
  release();
  await waitFor(() => expect(writers()).toEqual(["Bob · Edit", "Bob · Edit"]));
});

test("a next page read after a cursor the list read again no longer ends with is not added", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  let first = listing([5], "5");
  let answer!: (response: Response) => void;
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/versions": (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        return cursor === null ? json(first) : new Promise((resolve) => (answer = resolve));
      },
    },
  });
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("list", { name: "Versions" });
  const more = within(section("Versions")).getByRole("button", { name: "More versions" });

  await user.click(more);
  await waitFor(() => expect(more.getAttribute("aria-busy")).toBe("true"));
  // A version written meanwhile: the list's first page ends lower.
  first = listing([6], "6");
  await readAgain();
  await waitFor(() =>
    expect(within(section("Versions")).getAllByRole("link")[0]?.getAttribute("href")).toMatch(/\/versions\/6$/)
  );
  answer(json(listing([5, 4], null)));
  await waitFor(() => expect(more.getAttribute("aria-busy")).toBeNull());
  expect(within(section("Versions")).getAllByRole("listitem")).toHaveLength(1);
});

test("a More that fails says why beside it, which reads again", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  let fail = true;
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/versions": (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        if (cursor !== null && fail) {
          return Promise.reject(new TypeError("offline"));
        }
        const all = many(25).toReversed();
        return json(
          cursor === null ? { data: all.slice(0, 20), next_cursor: "20" } : { data: all.slice(20), next_cursor: null }
        );
      },
    },
  });
  renderApp(pagePath(install.id), server.app);
  const list = await within(await shownPanel()).findByRole("list", { name: "Versions" });

  await user.click(within(section("Versions")).getByRole("button", { name: "More versions" }));
  expect(await within(section("Versions")).findByRole("alert")).toBeTruthy();
  fail = false;
  await user.click(within(section("Versions")).getByRole("button", { name: "More versions" }));
  await waitFor(() => expect(within(list).getAllByRole("listitem")).toHaveLength(25));
  expect(within(section("Versions")).queryByRole("alert")).toBeNull();
});

test("versions that could not be read say why, and are read again on Try again", async () => {
  let fail = true;
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/versions": () =>
        fail ? problem(500, "internal") : json({ data: [versionJSON(1, null, "Install\n")], next_cursor: null }),
    },
  });
  renderApp(pagePath(install.id), server.app);

  const retry = await within(await shownPanel()).findByRole("button", { name: "Try again" });
  expect(within(section("Versions")).getByRole("alert").textContent).not.toBe("");
  fail = false;
  await userEvent.click(retry);
  expect(await within(section("Versions")).findByRole("list", { name: "Versions" })).toBeTruthy();
  expect(within(section("Versions")).queryByRole("alert")).toBeNull();
});

// M8/P1 review B, M2: a lease that ends is not an event.
test("an edit whose lease ran out, its lock read free, has the versions read again: none is marked editing", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  const first = versionJSON(1, null, "Install\n");
  const edited = versionJSON(2, 1, "Install\nmore\n", { by: bob, editing: true });
  server.withVersions(install.id, [first, edited]);
  const session = server.hold(install.id, bob, 30);
  renderApp(pagePath(install.id), server.app);
  expect(await screen.findByText("Bob is editing this page.")).toBeTruthy();
  await waitFor(() => expect(within(section("Versions")).getByText("Editing")).toBeTruthy());

  // Bob's tab went away: his lease runs out, and no event says so.
  server.sessions.delete(session);
  server.withVersions(install.id, [first, { ...edited, editing: false }]);
  await act(() => vi.advanceTimersByTimeAsync(31_000));

  await waitFor(() => expect(screen.queryByText("Bob is editing this page.")).toBeNull());
  await waitFor(() => expect(within(section("Versions")).queryByText("Editing")).toBeNull());
});

test("leaving an edit reads the versions at once, the edit's among them", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  const first = versionJSON(1, null, "Install\n");
  server.withVersions(install.id, [first]);
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("list", { name: "Versions" });

  await user.click(screen.getByRole("button", { name: "Edit" }));
  await screen.findByRole("textbox", { name: "Page content" });
  // What the edit wrote, its session over: no event tells this tab.
  server.withVersions(install.id, [first, versionJSON(2, 1, "Install\nmore\n")]);
  await act(async () => {});
  expect(within(section("Versions")).getAllByRole("listitem")).toHaveLength(1);
  const before = server.sent.length;
  await user.click(screen.getByRole("button", { name: "Done" }));

  await screen.findByRole("button", { name: "Edit" });
  await waitFor(() => expect(within(section("Versions")).getAllByRole("listitem")).toHaveLength(2));
  expect(server.sent.slice(before)).toContain("GET versions Install");
});
