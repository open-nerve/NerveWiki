import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";
import { Outlet } from "react-router";
import { afterEach, expect, test, vi } from "vitest";

import { routes } from "../../app/routes";
import { eventHandlers } from "../../events/handlers";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, problem, tokensJSON, workspaceJSON } from "../../test/fakes";
import {
  deferred,
  healthAsset,
  healthFolder,
  healthHome,
  healthPath,
  healthRaw,
  healthServer,
  wantedFacts,
} from "../../test/health-server";
import { assetNode, guide, pageNode, pagePath } from "../../test/page-server";
import { renderApp } from "../../test/render";

afterEach(() => vi.useRealTimers());

function StrictRoutes() {
  return (
    <StrictMode>
      <Outlet />
    </StrictMode>
  );
}

function wanted() {
  const section = screen.getByRole("heading", { name: "Wanted pages" }).closest("section");
  if (section === null) throw new Error("missing wanted card");
  return within(section);
}

test("every Health opening is fresh, including clicking the current link and Back/Forward", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  const { router } = renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "Wanted pages" });
  const initial = router.state.location.key;
  await user.click(screen.getByRole("link", { name: "Health" }));
  await waitFor(() => expect(fixture.summaries).toHaveLength(2));
  expect(router.state.location.key).not.toBe(initial);
  const entry = router.state.location.key;
  await act(() => router.navigate(pagePath(guide.id)));
  await screen.findByRole("article", { name: "Guide" });
  fixture.value.computed_at = "2026-11-01T17:25:00Z";
  await act(() => router.navigate(-1));
  await screen.findByText(/Nov 1, 2026/);
  expect(router.state.location.key).toBe(entry);
  expect(fixture.summaries).toHaveLength(3);
  await act(() => router.navigate(1));
  await screen.findByRole("article", { name: "Guide" });
  await act(() => router.navigate(-1));
  await screen.findByRole("heading", { name: "Wanted pages" });
  expect(fixture.summaries).toHaveLength(4);
});

test("leaving with a pending summary and returning within dedupe starts an independent request, ignoring the late answer", async () => {
  const fixture = healthServer();
  const old = deferred<Response>();
  fixture.responses.summary = (request) => (fixture.summaries.length === 1 ? old.promise : fixture.summary(request));
  const { router } = renderApp(healthPath, fixture.server.app);
  await waitFor(() => expect(fixture.summaries).toHaveLength(1));
  await act(() => router.navigate(healthHome));
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  fixture.value.computed_at = "2026-11-01T17:25:00Z";
  await act(() => router.navigate(-1));
  await screen.findByText(/Nov 1, 2026/);
  expect(fixture.summaries).toHaveLength(2);
  const output = screen.getByRole("status", { name: "Health results" });
  const before = output.textContent;
  fixture.value.computed_at = "2026-10-01T08:00:00Z";
  await act(async () => old.resolve(await fixture.summary(fixture.summaries[0]!)));
  expect(screen.queryByText(/Oct 1, 2026/)).toBeNull();
  expect(screen.getByText(/Nov 1, 2026/)).toBeTruthy();
  expect(output.textContent).toBe(before);
});

test("the service summary signal is aborted when a new check supersedes it, and on leaving", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  const held = deferred<Response>();
  fixture.responses.summary = () => held.promise;
  const { router } = renderApp(healthPath, fixture.server.app);
  await waitFor(() => expect(fixture.summaries).toHaveLength(1));
  await user.click(screen.getByRole("button", { name: "Recheck" }));
  await waitFor(() => expect(fixture.summaries).toHaveLength(2));
  expect(fixture.summaries[0]?.signal.aborted).toBe(true);
  expect(fixture.summaries[1]?.signal.aborted).toBe(false);
  await act(() => router.navigate(healthHome));
  expect(fixture.summaries[1]?.signal.aborted).toBe(true);
  await act(async () => held.resolve(await fixture.summary(fixture.summaries[0]!)));
});

test("the service card signal is aborted by refresh; the next pending summary is aborted by a new login", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  const card = deferred<Response>();
  const summary = deferred<Response>();
  fixture.responses.page = () => card.promise;
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "Wanted pages" });
  await user.click(wanted().getByRole("button", { name: "More" }));
  fixture.responses.summary = () => summary.promise;
  await user.click(screen.getByRole("button", { name: "Recheck" }));
  await waitFor(() => expect(fixture.summaries).toHaveLength(2));
  expect(fixture.pages[0]?.signal.aborted).toBe(true);
  const pending = fixture.summaries[1]!;
  fixture.responses.summary = fixture.summary;
  await act(() => fixture.server.app.session.tokens.signIn(tokensJSON));
  await screen.findByRole("heading", { name: "Wanted pages" });
  expect(pending.signal.aborted).toBe(true);
  await act(async () => {
    card.resolve(fixture.pageOf([{ ...wantedFacts(6)[5]!, target: "Old login card" }]));
    summary.resolve(await fixture.summary(pending));
  });
  expect(screen.queryByText("Old login card")).toBeNull();
});

test("Health cleanup keeps an in-flight public search and the shared notebook tree intact", async () => {
  const fixture = healthServer();
  const search = deferred<void>();
  fixture.server.searchWaits.first = search.promise;
  const { router } = renderApp("/lab/search?q=Guide", fixture.server.app);
  await waitFor(() => expect(fixture.server.sent.some((line) => line.startsWith("GET search"))).toBe(true));
  await act(() => router.navigate(healthPath));
  await screen.findByRole("heading", { name: "Wanted pages" });
  const trees = fixture.server.sent.filter((line) => line === "GET nodes").length;
  expect(trees).toBe(1);
  await act(() => router.navigate(-1));
  await act(async () => search.resolve());
  const results = await screen.findByRole("list", { name: "Search results" });
  expect(within(results).getByRole("link", { name: "Guide" })).toBeTruthy();
  expect(fixture.server.sent.filter((line) => line.startsWith("GET search"))).toHaveLength(1);
});

test("the lint provider survives StrictMode cleanup and a real route remount", async () => {
  const fixture = healthServer();
  const { router } = renderApp(healthPath, fixture.server.app, {
    routes: [{ Component: StrictRoutes, children: routes }],
  });
  await screen.findByRole("heading", { name: "Wanted pages" });
  await act(() => router.navigate(healthHome));
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  fixture.value.computed_at = "2026-11-01T17:25:00Z";
  await act(() => router.navigate(-1));
  expect(await screen.findByText(/Nov 1, 2026/)).toBeTruthy();
});

test.each([false, true])("summary busy is manual with retained data=%s, and never retries itself", async (retained) => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const fixture = healthServer();
  let busy = !retained;
  fixture.responses.summary = (request) =>
    busy ? problem(503, "server_busy", {}, { "Retry-After": "1" }) : fixture.summary(request);
  renderApp(healthPath, fixture.server.app);
  if (retained) {
    await screen.findByRole("heading", { name: "Wanted pages" });
    busy = true;
    await user.click(screen.getByRole("button", { name: "Recheck" }));
  }
  await screen.findByRole("alert");
  const count = fixture.summaries.length;
  await act(() => vi.advanceTimersByTimeAsync(300_000));
  expect(fixture.summaries).toHaveLength(count);
  if (retained) {
    expect(wanted().getAllByRole("listitem")).toHaveLength(5);
    expect(screen.getByText(/Oct 31, 2026/)).toBeTruthy();
  }
  busy = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));
  await screen.findByRole("heading", { name: "Wanted pages" });
  await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  expect(fixture.summaries).toHaveLength(count + 1);
});

test("summary network failures still receive the existing bounded retries", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const fixture = healthServer();
  fixture.responses.summary = () => {
    throw new TypeError("offline");
  };
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("alert");
  await act(() => vi.advanceTimersByTimeAsync(300_000));
  expect(fixture.summaries).toHaveLength(6);
});

test.each(["mode event", "missed mode then reconnect"] as const)(
  "real handlers discover %s without rerunning lint and disable More",
  async (change) => {
    const user = userEvent.setup();
    const events = eventServer();
    let notebooks = 0;
    const fixture = healthServer({
      "GET /api/v0/events": events.answer,
      "GET /api/v0/workspaces/lab/notebooks": () => {
        notebooks += 1;
        return json({ data: [fixture.server.notebook] });
      },
    });
    renderApp(healthPath, withEvents(fixture.server.app), { eventHandlers });
    await screen.findByRole("heading", { name: "Wanted pages" });
    await waitFor(() => expect(events.streams).toHaveLength(1));
    const firstReads = notebooks;
    events.last().hello();
    await waitFor(() => expect(notebooks).toBe(firstReads + 1));
    await waitFor(() =>
      expect(fixture.server.sent.filter((line) => line === "GET nodes").length).toBeGreaterThanOrEqual(2)
    );
    const before = { summaries: fixture.summaries.length, pages: fixture.pages.length, notebooks };
    fixture.server.notebook = { ...fixture.server.notebook, mode: "standard", source_root_id: null };
    fixture.value.mode = "standard";
    if (change === "mode event")
      events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });
    else {
      events.last().send("reset", { reason: "expired" });
      await waitFor(() => expect(events.streams).toHaveLength(2));
      events.last().hello();
    }
    await screen.findByText("The notebook mode has changed. Recheck its health.");
    expect(notebooks).toBe(before.notebooks + 1);
    expect(fixture.summaries).toHaveLength(before.summaries);
    await user.click(wanted().getByRole("button", { name: "More" }));
    expect(fixture.pages).toHaveLength(before.pages);
    await user.click(screen.getByRole("button", { name: "Recheck" }));
    await waitFor(() => expect(screen.queryByText("The notebook mode has changed. Recheck its health.")).toBeNull());
    expect(screen.queryByRole("heading", { name: "Missing summary" })).toBeNull();
    expect(fixture.summaries).toHaveLength(before.summaries + 1);
  }
);

test("the same summary result is announced again through the output that existed before the first answer", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  const answer = deferred<Response>();
  fixture.responses.summary = () => answer.promise;
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { level: 1, name: "Health" });
  const output = screen.getByRole("status", { name: "Health results" });
  expect(output.textContent).toBe("");
  await act(async () => answer.resolve(await fixture.summary(fixture.summaries[0]!)));
  await waitFor(() => expect(output.textContent).toContain("8 checks completed."));
  const first = output.textContent;
  fixture.responses.summary = fixture.summary;
  await user.click(screen.getByRole("button", { name: "Recheck" }));
  await waitFor(() => expect(output.textContent).not.toBe(first));
  await waitFor(() => expect(output.textContent?.trim()).toBe("8 checks completed."));
  expect(output.textContent).not.toBe(first);
  expect(screen.getByRole("status", { name: "Health results" })).toBe(output);
});

test.each([false, true])(
  "the zero-result undigested anchor opens its disclosure and respects reader action=%s",
  async (acted) => {
    const fixture = healthServer();
    const answer = deferred<Response>();
    fixture.responses.summary = () => answer.promise;
    renderApp(`${healthPath}#undigested_sources`, fixture.server.app);
    await screen.findByRole("heading", { level: 1, name: "Health" });
    const button = screen.getByRole("button", { name: "Recheck" });
    if (acted) {
      button.focus();
      fireEvent.wheel(window);
    }
    await act(async () => answer.resolve(await fixture.summary(fixture.summaries[0]!)));
    const heading = await screen.findByRole("heading", { name: "Undigested sources" });
    expect(heading.closest("details")?.open).toBe(true);
    await waitFor(() => expect(document.activeElement).toBe(acted ? button : heading));
  }
);

test("a summary transport that ignores abort cannot replace a newer check's results or announcement", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  const old = deferred<Response>();
  const oldResponse = await fixture.summary(
    new Request(`http://nervewiki.test/api/v0/notebooks/${notebookJSON.id}/lint`)
  );
  fixture.responses.summary = (request) => (fixture.summaries.length === 1 ? old.promise : fixture.summary(request));
  renderApp(healthPath, fixture.server.app);
  await waitFor(() => expect(fixture.summaries).toHaveLength(1));
  fixture.value.rules.wanted = [];
  fixture.value.computed_at = "2026-11-01T17:25:00Z";
  await user.click(screen.getByRole("button", { name: "Recheck" }));
  await screen.findByText(/Nov 1, 2026/);
  const output = screen.getByRole("status", { name: "Health results" });
  const said = output.textContent;
  const at = document.activeElement;
  await act(async () => old.resolve(oldResponse));
  expect(screen.queryByText("Missing 001")).toBeNull();
  expect(screen.queryByText(/Oct 31, 2026/)).toBeNull();
  expect(output.textContent).toBe(said);
  expect(document.activeElement).toBe(at);
});

test.each(["notebook", "login"] as const)("an ignored-abort old summary cannot appear in a new %s", async (change) => {
  const fixture = healthServer();
  const other = {
    ...fixture.server.notebook,
    id: "0199a2b4-0000-7000-8000-0000000000c2",
    name: "Other book",
    mode: "standard" as const,
    source_root_id: null,
  };
  fixture.server.others = [other];
  fixture.facts.set(other.id, { mode: "standard", computed_at: "2026-11-01T17:25:00Z", stale_pages: 0, rules: {} });
  const old = deferred<Response>();
  const oldResponse = await fixture.summary(
    new Request(`http://nervewiki.test/api/v0/notebooks/${notebookJSON.id}/lint`)
  );
  fixture.responses.summary = (request) => (fixture.summaries.length === 1 ? old.promise : fixture.summary(request));
  const { router } = renderApp(healthPath, fixture.server.app);
  await waitFor(() => expect(fixture.summaries).toHaveLength(1));
  fixture.value.rules = {};
  if (change === "login") await act(() => fixture.server.app.session.tokens.signIn(tokensJSON));
  else {
    await act(() => router.navigate(`/lab/notebooks/${other.id}/health`));
  }
  await screen.findByText(change === "login" ? "No issues (8 checks)" : "No issues (4 checks)");
  const said = screen.getByRole("status", { name: "Health results" }).textContent;
  await act(async () => old.resolve(oldResponse));
  expect(screen.queryByText("Missing 001")).toBeNull();
  expect(screen.getByRole("status", { name: "Health results" }).textContent).toBe(said);
});

test("a retained source attachment follows its current parent and focuses the actual attachment", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const fixture = healthServer({ "GET /api/v0/events": events.answer });
  const created = "2026-10-30T08:01:00Z";
  fixture.value.rules.undigested_sources = [
    {
      rule: "undigested_sources",
      item_key: `n:${healthAsset.id}`,
      node_id: healthAsset.id,
      kind: "asset",
      bytes: 0,
      created_at: created,
      cursor: btoa(
        JSON.stringify({
          v: 1,
          p: { v: 1, r: "undigested_sources", n: notebookJSON.id, s: "I", i: healthAsset.id, t: created },
        })
      )
        .replaceAll("+", "-")
        .replaceAll("/", "_")
        .replace(/=+$/, ""),
    },
  ];
  const { router } = renderApp(healthPath, withEvents(fixture.server.app), { eventHandlers });
  const link = await screen.findByRole("link", { name: "raw/Folder/evidence.pdf" });
  expect(link.getAttribute("href")).toBe(`${healthHome}/pages/${healthFolder.id}`);
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  await waitFor(() =>
    expect(fixture.server.sent.filter((line) => line === "GET nodes").length).toBeGreaterThanOrEqual(2)
  );
  fixture.server.nodes = fixture.server.nodes.map((node) =>
    node.id === healthAsset.id ? { ...node, parent_id: guide.id } : node
  );
  events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });
  await screen.findByRole("link", { name: "Guide/evidence.pdf" });
  await user.click(link);
  await screen.findByRole("article", { name: "Guide" });
  expect(router.state.location.state).toEqual({ attachment: healthAsset.id });
  await waitFor(() => expect(document.activeElement?.getAttribute("data-attachment")).toBe(healthAsset.id));
  expect(fixture.summaries).toHaveLength(1);
});

test("a completed More never refocuses its old addition after reader action and a real tree update", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const fixture = healthServer({ "GET /api/v0/events": events.answer });
  renderApp(healthPath, withEvents(fixture.server.app), { eventHandlers });
  await screen.findByRole("heading", { name: "Wanted pages" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  await waitFor(() =>
    expect(fixture.server.sent.filter((line) => line === "GET nodes").length).toBeGreaterThanOrEqual(2)
  );
  await user.click(wanted().getByRole("button", { name: "More" }));
  await screen.findByText("Missing 006");
  expect(document.activeElement).toBe(screen.getByText("Missing 006").closest("li"));
  await user.click(wanted().getByText("65 issues"));
  expect(document.activeElement).toBe(document.body);
  const said = wanted().getByRole("status").textContent;
  fixture.server.nodes = fixture.server.nodes.map((node) =>
    node.name === "Wiki 1" ? { ...node, name: "Revised Wiki" } : node
  );
  events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });
  await screen.findByRole("link", { name: "Revised Wiki" });
  expect(document.activeElement).toBe(document.body);
  expect(wanted().getByRole("status").textContent).toBe(said);
  expect(fixture.summaries).toHaveLength(1);
  expect(fixture.pages).toHaveLength(1);
});

test("missing More nodes share one tree reread, stay after its failure, and tree retry never reruns lint", async () => {
  const user = userEvent.setup();
  let reads = 0;
  let down = true;
  const fixture = healthServer({
    [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: () => {
      reads += 1;
      if (reads > 1 && down) throw new TypeError("tree offline");
      return json({ data: fixture.server.nodes });
    },
  });
  const sources = Array.from({ length: 5 }, (_, i) => pageNode(201 + i, `Source ${i + 1}`, healthRaw));
  const lost = [assetNode(206, "a.pdf", healthRaw), assetNode(207, "b.pdf", healthRaw)];
  fixture.server.nodes.push(...sources);
  const created = "2026-10-30T08:01:00Z";
  fixture.value.rules.undigested_sources = [...sources, ...lost].map((node) => ({
    rule: "undigested_sources",
    item_key: `n:${node.id}`,
    node_id: node.id,
    kind: node.kind,
    bytes: 0,
    created_at: created,
    cursor: btoa(
      JSON.stringify({ v: 1, p: { v: 1, r: "undigested_sources", n: notebookJSON.id, s: "I", i: node.id, t: created } })
    )
      .replaceAll("+", "-")
      .replaceAll("/", "_")
      .replace(/=+$/, ""),
  }));
  renderApp(healthPath, fixture.server.app);
  const heading = await screen.findByRole("heading", { name: "Undigested sources" });
  const card = within(heading.closest("section")!);
  await user.click(card.getByRole("button", { name: "More" }));
  const health = within(screen.getByRole("heading", { level: 1, name: "Health" }).closest("section")!);
  await health.findByRole("alert");
  expect(reads).toBe(2);
  expect(card.getAllByRole("listitem")).toHaveLength(7);
  expect(card.getAllByText("This node is no longer available.")).toHaveLength(2);
  expect(card.getByText("7 issues")).toBeTruthy();
  down = false;
  fixture.server.nodes.push(...lost);
  await user.click(health.getByRole("button", { name: "Try again" }));
  await card.findByRole("link", { name: "raw/a.pdf" });
  expect(card.getByRole("link", { name: "raw/b.pdf" })).toBeTruthy();
  expect(reads).toBe(3);
  expect(fixture.summaries).toHaveLength(1);
});

test.each([false, true])(
  "a real mode change stops an existing card's publication and automatic retry (pending=%s)",
  async (pending) => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    const events = eventServer();
    const fixture = healthServer({ "GET /api/v0/events": events.answer });
    const answer = deferred<Response>();
    fixture.responses.page = () => {
      if (pending) return answer.promise;
      throw new TypeError("offline");
    };
    renderApp(healthPath, withEvents(fixture.server.app), { eventHandlers });
    await screen.findByRole("heading", { name: "Wanted pages" });
    await waitFor(() => expect(events.streams).toHaveLength(1));
    events.last().hello();
    await waitFor(() =>
      expect(fixture.server.sent.filter((line) => line === "GET nodes").length).toBeGreaterThanOrEqual(2)
    );
    await user.click(wanted().getByRole("button", { name: "More" }));
    if (!pending) await wanted().findByRole("alert");
    fixture.server.notebook = { ...fixture.server.notebook, mode: "standard", source_root_id: null };
    events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });
    await screen.findByText("The notebook mode has changed. Recheck its health.");
    const output = wanted().getByRole("status");
    const said = output.textContent;
    if (pending) await act(async () => answer.resolve(fixture.pageOf(wantedFacts(6).slice(5), null, "standard")));
    else await act(() => vi.advanceTimersByTimeAsync(6_000));
    expect(fixture.pages).toHaveLength(1);
    expect(fixture.summaries).toHaveLength(1);
    expect(wanted().getAllByRole("listitem")).toHaveLength(5);
    expect(screen.queryByText("Missing 006")).toBeNull();
    expect(output.textContent).toBe(said);
  }
);
