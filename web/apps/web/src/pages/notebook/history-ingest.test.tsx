import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test } from "vitest";

import { eventHandlers } from "../../events/handlers";
import { eventServer, withEvents } from "../../test/event-server";
import { notebookJSON, workspaceJSON, type Answer } from "../../test/fakes";
import { historyRoutes, wrote } from "../../test/history-server";
import { assetNode, guide, install, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

const home = `/lab/notebooks/${notebookJSON.id}`;
const history = `${home}/history`;
const listRoute = `GET /api/v0/notebooks/${notebookJSON.id}/changesets`;
const gone = "0199a2b4-0000-7000-8000-000000000999";

// Observe the real service's query while delegating every response to the
// existing history fake. No copy of its filtering or cursor algorithm.
function observedHistory() {
  let answer!: Answer;
  const requests: Request[] = [];
  const completed: Promise<Response>[] = [];
  const held = { next: undefined as Promise<void> | undefined };
  const server = pageServer({
    role: "reader",
    answers: {
      [listRoute]: (request) => {
        requests.push(request);
        const query = new URL(request.url).searchParams;
        const wait = query.has("cursor") && !query.has("ingest") ? held.next : undefined;
        const response = Promise.resolve(wait).then(() => answer(request));
        completed.push(response);
        return response;
      },
    },
  });
  const route = historyRoutes(server, "reader")[listRoute];
  if (route === undefined) throw new Error("the original history route is missing");
  answer = route;
  return { server, requests, completed, held };
}

const rows = () => [...screen.getByRole("list", { name: "Changes" }).children] as HTMLElement[];
const messages = () => rows().map((row) => row.querySelector("bdi")?.textContent);
const ingestion = () => screen.getByRole("combobox", { name: "Ingestion" });
const sources = () => screen.getByRole("list", { name: "Ingested sources" });

test("P4/W05 filter_key: URL and boolean request select ingestion without losing the other filters or focus", async () => {
  const { server, requests } = observedHistory();
  server.log("ordinary", { client: "api", message: "Ordinary edit", changes: [wrote(guide, 1, 2)] });
  server.log("ingest", {
    client: "api",
    message: "Ingest edit",
    ingestCount: 1,
    ingestSources: [install.id],
    changes: [wrote(guide, 2, 3)],
  });
  const { router } = renderApp(`${history}?client=api&page=${guide.id}`, server.app);
  await waitFor(() => expect(rows()).toHaveLength(2));
  const ordinary = rows().find((row) => row.querySelector("bdi")?.textContent === "Ordinary edit");
  expect(ordinary).toBeDefined();
  expect(within(ordinary as HTMLElement).queryByText("Ingest")).toBeNull();
  expect(within(ordinary as HTMLElement).queryByRole("list", { name: "Ingested sources" })).toBeNull();
  const select = ingestion();
  await userEvent.selectOptions(select, "Only ingestion");
  await waitFor(() => expect(messages()).toEqual(["Ingest edit"]));
  expect(router.state.location.search).toBe(`?client=api&page=${guide.id}&ingest=1`);
  expect(document.activeElement).toBe(select);
  const query = new URL(requests.at(-1)?.url ?? "").searchParams;
  expect([query.get("ingest"), query.get("client"), query.get("page_id")]).toEqual(["true", "api", guide.id]);
  await userEvent.selectOptions(select, "All changes");
  await waitFor(() => expect(rows()).toHaveLength(2));
  expect(router.state.location.search).toBe(`?client=api&page=${guide.id}`);
  expect(new URL(requests[0]?.url ?? "").searchParams.has("ingest")).toBe(false);
  expect(document.activeElement).toBe(select);
});

test("P4/W05 filter_key: ingest=1 opens a filtered list and clearing invalid filters also clears ingestion", async () => {
  const { server, requests } = observedHistory();
  server.log("ordinary", { message: "Ordinary edit" });
  server.log("ingest", { message: "Ingest edit", ingestCount: 1, ingestSources: [guide.id] });
  const { router } = renderApp(`${history}?ingest=1`, server.app);
  await waitFor(() => expect(messages()).toEqual(["Ingest edit"]));
  expect((ingestion() as HTMLSelectElement).value).toBe("1");
  expect(new URL(requests[0]?.url ?? "").searchParams.get("ingest")).toBe("true");
  await act(() => router.navigate(`${history}?ingest=1&actor=invalid`));
  await userEvent.click(await screen.findByRole("button", { name: "Clear filters" }));
  await waitFor(() => expect(rows()).toHaveLength(2));
  expect(router.state.location.search).toBe("");
  expect((ingestion() as HTMLSelectElement).value).toBe("");
});

test("P4/W05 filter_key: an old More completes into its own cache while ingestion paginates independently", async () => {
  const { server, requests, completed, held } = observedHistory();
  for (let n = 1; n <= 57; n++) {
    server.log(`row ${n.toString()}`, {
      at: new Date(Date.parse("2026-10-05T08:00:00Z") + n * 60_000).toISOString(),
      message: n <= 51 ? `Ingest ${n.toString()}` : `Ordinary ${n.toString()}`,
      ingestCount: n <= 51 ? 1 : 0,
      ingestSources: n <= 51 ? [guide.id] : [],
    });
  }
  renderApp(history, server.app);
  await waitFor(() => expect(rows()).toHaveLength(50));
  let release!: () => void;
  held.next = new Promise<void>((resolve) => {
    release = resolve;
  });
  onTestFinished(() => release());
  await userEvent.click(screen.getByRole("button", { name: "More changes" }));
  await waitFor(() => expect(requests).toHaveLength(2));
  const oldMore = completed[1];
  expect(oldMore).toBeDefined();
  await userEvent.selectOptions(ingestion(), "Only ingestion");
  await waitFor(() => expect(messages()[0]).toBe("Ingest 51"));
  expect(rows()).toHaveLength(50);
  expect(screen.getByRole("button", { name: "More changes" }).getAttribute("aria-busy")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "More changes" }));
  await waitFor(() => expect(rows()).toHaveLength(51));
  expect(messages().at(-1)).toBe("Ingest 1");
  ingestion().focus();
  await act(async () => {
    release();
    await oldMore;
  });
  expect(rows()).toHaveLength(51);
  expect(messages().some((message) => message?.startsWith("Ordinary"))).toBe(false);
  expect(document.activeElement).toBe(ingestion());
  expect(
    requests.slice(0, 4).map((request) => {
      const query = new URL(request.url).searchParams;
      return [query.get("ingest"), query.has("cursor")];
    })
  ).toEqual([
    [null, false],
    [null, true],
    ["true", false],
    ["true", true],
  ]);
  await userEvent.selectOptions(ingestion(), "All changes");
  await waitFor(() => expect(rows()).toHaveLength(57));
  expect(messages().slice(0, 2)).toEqual(["Ordinary 57", "Ordinary 56"]);
});

test("P4/W09 history_source: three and twenty positions keep nulls, the original edit kind and the 10000 lower bound", async () => {
  const attached = assetNode(81, "a.txt", guide);
  const rootAsset = assetNode(82, "root.txt");
  const extra = Array.from({ length: 17 }, (_, n) =>
    pageNode(200 + n, `Source ${(n + 1).toString().padStart(2, "0")}`)
  );
  const server = pageServer({ role: "reader", nodes: [guide, install, notes, attached, rootAsset, ...extra] });
  const edit = server.log("ingest", {
    message: "Kept original edit",
    changes: [wrote(guide, 1, 2)],
    ingestCount: 10000,
    ingestSources: [attached.id, gone, guide.id, rootAsset.id, ...extra.map((node) => node.id)],
  });
  const { router } = renderApp(history, server.app);
  const preview = await screen.findByRole("list", { name: "Ingested sources" });
  expect(
    within(preview)
      .getAllByRole("listitem")
      .map((row) => row.textContent)
  ).toEqual(["Guide/a.txt", "Deleted source", "Guide.md"]);
  expect(
    within(preview)
      .getAllByRole("link")
      .map((a) => a.getAttribute("href"))
  ).toEqual([pagePath(guide.id), pagePath(guide.id)]);
  const removed = within(preview).getByText("Deleted source");
  expect(removed.closest("a")).toBeNull();
  expect(screen.getByText("Sources: 10000+")).toBeTruthy();
  const editLink = within(rows()[0] as HTMLElement).getByRole("link", { name: /^Edit · Ada ·/ });
  expect(editLink.textContent).toBe("Edit · Ada");
  expect(editLink.parentElement?.textContent).toContain("Ingest");
  await userEvent.click(editLink);
  expect(await screen.findByRole("heading", { level: 1, name: "Edit · Ada Ingest" })).toBeTruthy();
  const detail = await screen.findByRole("list", { name: "Ingested sources" });
  expect(
    within(detail)
      .getAllByRole("listitem")
      .map((row) => row.textContent)
  ).toEqual([
    "Guide/a.txt",
    "Deleted source",
    "Guide.md",
    "root.txt",
    "Source 01.md",
    "Source 02.md",
    "Source 03.md",
    "Source 04.md",
    "Source 05.md",
    "Source 06.md",
    "Source 07.md",
    "Source 08.md",
    "Source 09.md",
    "Source 10.md",
    "Source 11.md",
    "Source 12.md",
    "Source 13.md",
    "Source 14.md",
    "Source 15.md",
    "Source 16.md",
  ]);
  expect(within(detail).queryByText("Source 17.md")).toBeNull();
  expect(within(detail).getAllByRole("link")).toHaveLength(19);
  await userEvent.click(within(detail).getByRole("link", { name: "Guide/a.txt" }));
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  expect(router.state.location.pathname).toBe(pagePath(guide.id));
  await waitFor(() => expect(document.activeElement?.getAttribute("data-attachment")).toBe(attached.id));
  await act(() => router.navigate(`${history}/${edit.id}`));
  await screen.findByRole("list", { name: "Ingested sources" });
  expect(within(sources()).getByRole("link", { name: "root.txt" }).getAttribute("href")).toBe(home);
  await userEvent.click(within(sources()).getByRole("link", { name: "root.txt" }));
  await waitFor(() => expect(router.state.location.pathname).toBe(home));
  await waitFor(() => expect(document.activeElement?.getAttribute("data-attachment")).toBe(rootAsset.id));
});

test("P4/W09 history_source: an asset uses its current parent after a move while the history stays mounted", async () => {
  const attached = assetNode(81, "a.txt", guide);
  const events = eventServer();
  const server = pageServer({
    role: "reader",
    nodes: [guide, notes, attached],
    answers: { "GET /api/v0/events": events.answer },
  });
  server.log("ingest", { ingestCount: 1, ingestSources: [attached.id], changes: [wrote(guide, 1, 2)] });
  renderApp(history, withEvents(server.app), { eventHandlers });
  await screen.findByRole("link", { name: "Guide/a.txt" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const stream = events.last();
  act(() => stream.hello());
  await waitFor(() => expect(server.sent.filter((each) => each === "GET nodes").length).toBeGreaterThanOrEqual(2));
  server.nodes = [guide, notes, { ...attached, parent_id: notes.id }];
  act(() =>
    stream.send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] })
  );
  const link = await screen.findByRole("link", { name: "Notes/a.txt" });
  expect(link.getAttribute("href")).toBe(pagePath(notes.id));
  await userEvent.click(link);
  await waitFor(() => expect(document.activeElement?.getAttribute("data-attachment")).toBe(attached.id));
});

test("P4/W09 history_source: ingestion controls and source text have Chinese equivalents", async () => {
  const server = pageServer({ role: "reader" });
  server.app.preferences.setLocale("zh-CN");
  server.log("ingest", { ingestCount: 1, ingestSources: [gone], changes: [wrote(guide, 1, 2)] });
  renderApp(`${history}?ingest=1`, server.app);
  expect(await screen.findByRole("combobox", { name: "消化" })).toBeTruthy();
  expect(screen.getByRole("option", { name: "全部变更" })).toBeTruthy();
  expect(screen.getByRole("option", { name: "仅含消化" })).toBeTruthy();
  const list = await screen.findByRole("list", { name: "已消化来源" });
  expect(within(list).getByText("已删除的来源").closest("a")).toBeNull();
  expect(screen.getByText("已消化")).toBeTruthy();
  expect(screen.getByText("来源：1")).toBeTruthy();
  expect(within(rowsChinese()).getByRole("link", { name: /^编辑 · Ada ·/ }).textContent).toBe("编辑 · Ada");
});

function rowsChinese(): HTMLElement {
  const first = screen.getByRole("list", { name: "改动" }).firstElementChild;
  if (!(first instanceof HTMLElement)) throw new Error("the Chinese history is empty");
  return first;
}
