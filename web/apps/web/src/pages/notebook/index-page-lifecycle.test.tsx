import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";
import { Outlet } from "react-router";
import { afterEach, expect, test, vi } from "vitest";

import { routes } from "../../app/routes";
import { eventHandlers } from "../../events/handlers";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, tokensJSON, workspaceJSON } from "../../test/fakes";
import { deferred } from "../../test/health-server";
import { indexFacts, indexHome, indexPath, indexRaw, indexServer } from "../../test/index-server";
import { guide, pagePath } from "../../test/page-server";
import { renderApp } from "../../test/render";

// Observes the real hook's key callback, without substituting fetch, cache, retry, or hook results.
const swrKeys = vi.hoisted((): unknown[][] => []);
vi.mock("swr/infinite", async (original) => {
  const actual = await original<typeof import("swr/infinite")>();
  const real = actual.default as (...args: unknown[]) => unknown;
  const observed = (getKey: (page: number, previous: unknown) => unknown, ...rest: unknown[]) =>
    real(
      (page: number, previous: unknown) => {
        const key = getKey(page, previous);
        if (Array.isArray(key)) swrKeys.push(Array.from(key));
        return key;
      },
      ...rest
    );
  return { ...actual, default: observed };
});

afterEach(() => {
  vi.useRealTimers();
  swrKeys.length = 0;
});
function StrictRoutes() {
  return (
    <StrictMode>
      <Outlet />
    </StrictMode>
  );
}
function shown() {
  return within(screen.getByRole("region", { name: "Directory pages" }))
    .getAllByRole("link")
    .filter((link) => link.getAttribute("href")?.includes("/pages/"))
    .map((link) => link.textContent);
}

test("P4/key_generation: same URL opens and Back/Forward reread; ignored abort cannot resurrect an old response", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  const held = deferred<Response>();
  fixture.answer = () => (fixture.requests.length === 1 ? held.promise : json(indexFacts()));
  const { router } = renderApp(indexPath, fixture.server.app);
  await waitFor(() => expect(fixture.requests).toHaveLength(1));
  const oldKey = router.state.location.key;
  await user.click(screen.getByRole("link", { name: "Directory" }));
  await screen.findByRole("region", { name: "Directory pages" });
  expect(router.state.location.key).not.toBe(oldKey);
  expect(fixture.requests).toHaveLength(2);
  expect(fixture.requests[0]!.signal.aborted).toBe(true);
  await act(async () =>
    held.resolve(json({ ...indexFacts(), data: [{ ...indexFacts().data[0]!, title: "Obsolete opening" }] }))
  );
  expect(shown()).toEqual(["Alpha", "Bravo", "Charlie"]);
  const opened = router.state.location.key;
  await act(() => router.navigate(pagePath(guide.id)));
  await screen.findByRole("article", { name: "Guide" });
  await act(() => router.navigate(-1));
  await screen.findByRole("region", { name: "Directory pages" });
  expect(router.state.location.key).toBe(opened);
  expect(fixture.requests).toHaveLength(3);
  await act(() => router.navigate(1));
  await screen.findByRole("article", { name: "Guide" });
  await act(() => router.navigate(-1));
  await screen.findByRole("region", { name: "Directory pages" });
  expect(fixture.requests).toHaveLength(4);
});

test.each(["first", "cursor"] as const)(
  "P4/late_%s: a retired filter response cannot append to the current list",
  async (part) => {
    const user = userEvent.setup();
    const fixture = indexServer();
    const held = deferred<Response>();
    fixture.value.next_cursor = "old-more";
    fixture.answer = (request) => {
      const query = new URL(request.url).searchParams;
      if (query.get("type") === "Topic")
        return json({ ...indexFacts(), data: [{ ...indexFacts().data[2]!, title: "Current Topic" }] });
      if (part === "first" || query.has("cursor")) return held.promise;
      return json(fixture.value);
    };
    const { router } = renderApp(indexPath, fixture.server.app);
    await waitFor(() => expect(fixture.requests).toHaveLength(1));
    if (part === "cursor") {
      await user.click(await screen.findByRole("button", { name: "More" }));
      await waitFor(() => expect(fixture.requests).toHaveLength(2));
    }
    const old = fixture.requests.at(-1)!;
    await act(() => router.navigate(`${indexPath}?type=Topic`));
    await screen.findByRole("link", { name: "Current Topic" });
    const count = fixture.requests.length;
    expect(old.signal.aborted).toBe(true);
    await act(async () =>
      held.resolve(json({ ...indexFacts(), data: [{ ...indexFacts().data[0]!, title: "Retired item" }] }))
    );
    expect(shown()).toEqual(["Current Topic"]);
    expect(fixture.requests).toHaveLength(count);
  }
);

test("P4/key_contract: actual SWR page keys carry opening, both exact filters and native cursor", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  fixture.value.next_cursor = "literal-cursor";
  fixture.answer = (request) =>
    new URL(request.url).searchParams.has("cursor") ? json({ ...indexFacts(), data: [] }) : json(fixture.value);
  const { router } = renderApp(`${indexPath}?type=Note&tag=plan%2Fq4`, fixture.server.app);
  await screen.findByRole("region", { name: "Directory pages" });
  const locationKey = router.state.location.key;
  await user.click(screen.getByRole("button", { name: "More" }));
  await waitFor(() => expect(fixture.requests).toHaveLength(2));
  expect(
    swrKeys.some(
      (key) =>
        key[0] === "notebook-index" &&
        key[3] === notebookJSON.id &&
        key[4] === locationKey &&
        key[5] === "Note" &&
        key[6] === "plan/q4" &&
        key[7] === null
    )
  ).toBe(true);
  expect(
    swrKeys.some(
      (key) =>
        key[0] === "notebook-index" &&
        key[4] === locationKey &&
        key[5] === "Note" &&
        key[6] === "plan/q4" &&
        key[7] === "literal-cursor"
    )
  ).toBe(true);
  expect(fixture.requests.map((request) => Array.from(new URL(request.url).searchParams))).toEqual([
    [
      ["type", "Note"],
      ["tag", "plan/q4"],
    ],
    [
      ["type", "Note"],
      ["tag", "plan/q4"],
      ["cursor", "literal-cursor"],
    ],
  ]);
});

test("P4/retry: an ordinary network failure really retries and then renders three pages", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const fixture = indexServer();
  fixture.answer = () => {
    if (fixture.requests.length === 1) throw new TypeError("offline");
    return json(indexFacts());
  };
  renderApp(indexPath, fixture.server.app);
  await screen.findByRole("alert");
  await act(() => vi.advanceTimersByTimeAsync(5_000));
  await screen.findByRole("region", { name: "Directory pages" });
  expect(fixture.requests).toHaveLength(2);
  expect(shown()).toEqual(["Alpha", "Bravo", "Charlie"]);
});

test("P4/later_retry: a later page is not read again by itself, whose end would take the focus; More reads it", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const fixture = indexServer();
  fixture.value.data = [indexFacts().data[0]!];
  fixture.value.next_cursor = "next";
  let offline = true;
  fixture.answer = (request) => {
    if (!new URL(request.url).searchParams.has("cursor")) return json(fixture.value);
    if (offline) throw new TypeError("offline");
    return json({ ...indexFacts(), data: [indexFacts().data[1]!] });
  };
  renderApp(indexPath, fixture.server.app);
  const more = await screen.findByRole("button", { name: "More" });
  more.focus();
  await user.keyboard("{Enter}");
  await screen.findByRole("alert");
  offline = false;
  await act(() => vi.advanceTimersByTimeAsync(300_000));
  expect(fixture.requests).toHaveLength(2);
  expect(document.activeElement).toBe(more);
  await user.keyboard("{Enter}");
  const added = await screen.findByRole("link", { name: "Bravo" });
  await waitFor(() => expect(document.activeElement).toBe(added));
});

test("P4/retired_retry: leaving retires a scheduled retry and isolates the next login", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const fixture = indexServer();
  fixture.answer = () => {
    throw new TypeError("offline");
  };
  const { router } = renderApp(indexPath, fixture.server.app);
  await screen.findByRole("alert");
  await act(() => router.navigate(indexHome));
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  await act(() => vi.advanceTimersByTimeAsync(300_000));
  expect(fixture.requests).toHaveLength(1);
  fixture.answer = () => json(indexFacts());
  await act(() => fixture.server.app.session.tokens.signIn(tokensJSON));
  await act(() => router.navigate(indexPath));
  await screen.findByRole("region", { name: "Directory pages" });
  expect(shown()).toEqual(["Alpha", "Bravo", "Charlie"]);
});

test("P4/strict_cleanup: a real StrictMode route remains readable and remounts fresh", async () => {
  const fixture = indexServer();
  const { router } = renderApp(indexPath, fixture.server.app, {
    routes: [{ Component: StrictRoutes, children: routes }],
  });
  await screen.findByRole("region", { name: "Directory pages" });
  const before = fixture.requests.length;
  await act(() => router.navigate(indexHome));
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  await act(() => router.navigate(-1));
  await screen.findByRole("region", { name: "Directory pages" });
  expect(fixture.requests.length).toBeGreaterThan(before);
  expect(shown()).toEqual(["Alpha", "Bravo", "Charlie"]);
});

test.each(["mode", "reconnect"] as const)(
  "P4/event_staleness/%s: real handlers update notebook mode without recomputing Directory",
  async (change) => {
    const events = eventServer();
    let notebooks = 0;
    const fixture = indexServer({
      "GET /api/v0/events": events.answer,
      "GET /api/v0/workspaces/lab/notebooks": () => {
        notebooks += 1;
        return json({ data: [fixture.server.notebook] });
      },
    });
    fixture.server.notebook.mode = "llm_wiki";
    fixture.server.notebook.source_root_id = indexRaw.id;
    fixture.value.mode = "llm_wiki";
    fixture.value.next_cursor = "next";
    renderApp(indexPath, withEvents(fixture.server.app), { eventHandlers });
    await screen.findByRole("region", { name: "Directory pages" });
    await waitFor(() => expect(events.streams).toHaveLength(1));
    const initial = notebooks;
    events.last().hello();
    await waitFor(() => expect(notebooks).toBe(initial + 1));
    const treeReads = fixture.server.sent.filter((line) => line === "GET nodes").length;
    events
      .last()
      .send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });
    events.last().send("links", {
      workspace_id: workspaceJSON.id,
      notebook_id: notebookJSON.id,
      pages: [],
      targets: [],
      aliases: false,
    });
    await waitFor(() =>
      expect(fixture.server.sent.filter((line) => line === "GET nodes").length).toBeGreaterThan(treeReads)
    );
    expect(fixture.requests).toHaveLength(1);
    const before = notebooks;
    fixture.server.notebook = { ...fixture.server.notebook, mode: "standard", source_root_id: null };
    if (change === "mode") events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });
    else {
      events.last().send("reset", { reason: "expired" });
      await waitFor(() => expect(events.streams).toHaveLength(2));
      events.last().hello();
    }
    await screen.findByText("The notebook mode has changed. Reopen its directory.");
    expect(notebooks).toBe(before + 1);
    expect(fixture.requests).toHaveLength(1);
    expect(shown()).toEqual(["Alpha", "Bravo", "Charlie"]);
    expect(screen.queryByRole("button", { name: "More" })).toBeNull();
  }
);
