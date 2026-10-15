import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { eventHandlers } from "../../events/handlers";
import type { Notebook } from "../../services/notebook.service";
import type { SearchHit, SearchPage, TreeNode } from "../../services/page.service";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, problem, workspaceJSON, type Answer } from "../../test/fakes";
import { assetNode, guide, install, linux, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { readAgain } from "../../test/page-panel";
import { renderApp } from "../../test/render";

// A workspace's search's results (M8 design 4.12, 4.13; M8/P5 design 4.3, 4.4).

afterEach(() => {
  vi.useRealTimers();
});

const home = `/lab/notebooks/${notebookJSON.id}`;
const nbsp = String.fromCharCode(0xa0);

/** searchPath is the results page of q, in the notebook notebook or in all. */
function searchPath(q: string, notebook?: string): string {
  const query = new URLSearchParams({ q });
  if (notebook !== undefined) {
    query.set("notebook_id", notebook);
  }
  return `/lab/search?${query.toString()}`;
}

/** changed is node, changed minute minutes past 8 on Oct 5, 2026. */
const changed = (node: TreeNode, minute: number): TreeNode => ({
  ...node,
  updated_at: `2026-10-05T08:${minute.toString().padStart(2, "0")}:00Z`,
});

/** gate is a wait a test opens: what waits for its promise goes on once open is called. */
function gate() {
  let open!: () => void;
  const promise = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { promise, open };
}

/** A notebook of Lab's other than Plans, which Ada sees. */
const ideas: Notebook = { ...notebookJSON, id: "0199a2b4-0000-7000-8000-0000000000c2", name: "Ideas" };

const database = changed(pageNode(11, "数据库设计"), 50);
const storage = changed(pageNode(12, "Storage", install), 40);
const report = changed(assetNode(13, "数据库备份.pdf", guide), 45);

/**
 * searched is pageServer with Plans' pages, and three that 数据库 finds: the
 * page 数据库设计 and the attachment 数据库备份.pdf by their names, and
 * Storage by its content, far into it; answers answer instead of it.
 */
function searched(answers: Record<string, Answer> = {}) {
  const server = pageServer({ nodes: [guide, install, linux, notes, database, storage, report], answers });
  server.contents.set(database.id, { content: "表结构。\n", revision: 1 });
  server.contents.set(storage.id, {
    content: `${"Pages are kept in rows. ".repeat(3)}Our 数据库 keeps them.${" More text.".repeat(20)}`,
    revision: 1,
  });
  return server;
}

/** reads are the searches server answered. */
const reads = (server: { sent: string[] }) => server.sent.filter((each) => each.startsWith("GET search"));

/** hits are the rows the results list. */
const hits = () => within(screen.getByRole("list", { name: "Search results" })).getAllByRole("listitem");

/** statusLine is the line that says what was found; told, what More and Keep looking said last. */
const statusLine = () => screen.getAllByRole("status")[0] as HTMLElement;
const told = () => screen.getAllByRole("status")[1] as HTMLElement;

/** marks are the hits marked in element. */
const marks = (element: Element) => [...element.querySelectorAll("mark")].map((mark) => mark.textContent);

const heading = (name: string) => screen.findByRole("heading", { level: 1, name });

test("the results list the names' hits, then the contents', each with its kind's icon, its name and its hits marked, where it is, its content around its first hit, when it last changed, an attachment's type and size; the page is titled for the search, its status line describing its title", async () => {
  vi.useFakeTimers({ now: Date.parse("2026-10-05T10:01:30Z"), toFake: ["Date"] });
  const server = searched();
  renderApp(searchPath("数据库"), server.app);

  const title = await heading("Search “数据库”");
  expect(title.querySelector("bdi")?.textContent).toBe("数据库");
  await waitFor(() => expect(document.title).toBe("Search: 数据库 · Lab · Nerve Wiki"));
  await screen.findByRole("list", { name: "Search results" });
  expect(statusLine().textContent).toBe("Results: 3.");
  expect(title.getAttribute("aria-describedby")).toBe(statusLine().id);
  const [page, attachment, body] = hits() as HTMLElement[];

  const pageLink = within(page as HTMLElement).getByRole("link", { name: "数据库设计" });
  expect(pageLink.getAttribute("href")).toBe(pagePath(database.id));
  expect(marks(pageLink)).toEqual(["数据库"]);
  expect(pageLink.querySelector("mark")?.className).toContain("nw-hit");
  expect(pageLink.querySelector("bdi")).not.toBeNull();
  expect(page?.querySelector("svg")?.getAttribute("aria-hidden")).toBe("true");
  expect(page?.textContent).toContain("Plans");
  expect(within(page as HTMLElement).getByTitle("Oct 5, 2026, 8:50 AM").textContent).toBe("1 hour ago");
  // Its content holds no hit: no excerpt.
  expect(page?.textContent).not.toContain("表结构");

  const attachmentLink = within(attachment as HTMLElement).getByRole("link", { name: "数据库备份.pdf" });
  expect(marks(attachmentLink)).toEqual(["数据库"]);
  expect(attachment?.querySelector("svg")?.getAttribute("aria-hidden")).toBe("true");
  expect(attachment?.textContent).toContain("Plans / Guide");
  expect(attachment?.textContent).toContain("PDF · 1 KB");

  expect(
    within(body as HTMLElement)
      .getByRole("link", { name: "Storage" })
      .getAttribute("href")
  ).toBe(pagePath(storage.id));
  expect(body?.textContent).toContain("Plans / Guide / Install");
  // Cut at both ends, the content going on.
  const excerpt = [...(body as HTMLElement).querySelectorAll("p bdi")].at(-1);
  expect(excerpt?.textContent).toMatch(/^….*Our 数据库 keeps them\..*…$/);
  expect(marks(excerpt as Element)).toEqual(["数据库"]);
  expect(reads(server)).toEqual(["GET search 数据库"]);
});

test("the results speak Chinese too", async () => {
  const server = searched();
  server.app.preferences.setLocale("zh-CN");
  renderApp(searchPath("数据库"), server.app);

  expect(await heading("搜索「数据库」")).toBeTruthy();
  await waitFor(() => expect(document.title).toBe("搜索：数据库 · Lab · Nerve Wiki"));
  const list = await screen.findByRole("list", { name: "搜索结果" });
  expect(statusLine().textContent).toBe("3 条。");
  expect(within(list).getAllByRole("listitem")[1]?.textContent).toContain("PDF · 1 KB");
  expect(screen.getByRole("combobox", { name: "在哪里找" })).toBeTruthy();
});

test("the counts the status line says: about, at least, 1,000 and more, none yet; and that the index is being built", async () => {
  const server = searched();
  const cases: [typeof server.searchCount, boolean, string][] = [
    [{ value: 240, kind: "about" }, false, "Results: about 240."],
    [{ value: 800, kind: "at_least" }, false, "Results: at least 800."],
    [{ value: 1000, kind: "at_least" }, false, "Results: 1,000+."],
    [null, false, "None found yet; you can keep looking."],
    [{ value: 3, kind: "exact" }, true, "Results: 3. The index is being built; results may be incomplete."],
  ];
  const { router } = renderApp(searchPath("数据库"), server.app);
  await screen.findByRole("list", { name: "Search results" });
  for (const [count, building, text] of cases) {
    server.searchCount = count;
    server.searchBuilding = building;
    // oxlint-disable-next-line no-await-in-loop -- asked again, a count at a time
    await act(() => router.navigate(searchPath("数据库")));
    // oxlint-disable-next-line no-await-in-loop -- its line said
    await waitFor(() => expect(statusLine().textContent).toBe(text));
  }
});

test("the hits marked are the server's ranges: past the BMP, side by side, cut at the text's end", async () => {
  const hit: SearchHit = {
    node: { id: database.id, notebook_id: notebookJSON.id, kind: "page", name: "📘 数据库" },
    ancestors: [],
    title_hits: [{ start: 3, end: 6 }],
    excerpt: {
      text: "全文检索与数据库",
      hits: [
        { start: 0, end: 2 },
        { start: 2, end: 4 },
        { start: 5, end: 9 },
      ],
      cut_start: false,
      cut_end: false,
    },
    changed_at: "2026-10-05T08:00:00Z",
    attachment: null,
  };
  const answer: SearchPage = { data: [hit], next_cursor: null, count: { value: 1, kind: "exact" }, building: false };
  const server = searched({ "GET /api/v0/workspaces/lab/search": () => json(answer) });
  renderApp(searchPath("全文 检索 数据库"), server.app);

  await screen.findByRole("list", { name: "Search results" });
  const [row] = hits();
  const link = within(row as HTMLElement).getByRole("link", { name: "📘 数据库" });
  expect(marks(link)).toEqual(["数据库"]);
  const excerpt = [...(row as HTMLElement).querySelectorAll("p bdi")].at(-1) as Element;
  expect(excerpt.textContent).toBe("全文检索与数据库");
  expect(marks(excerpt)).toEqual(["全文", "检索", "数据库"]);
});

test("a page leads to itself, arrived at; an attachment to its page's attachments, its row taking the focus, or, at the notebook's top level, to the notebook's home", async () => {
  const user = userEvent.setup();
  const top = changed(assetNode(14, "数据库.png"), 44);
  const server = searched();
  server.nodes = [...server.nodes, top];
  const { router } = renderApp(searchPath("数据库"), server.app);
  await screen.findByRole("list", { name: "Search results" });

  const topLink = screen.getByRole("link", { name: "数据库.png" });
  expect(topLink.getAttribute("href")).toBe(home);
  await user.click(topLink);
  await waitFor(() => expect(router.state.location.pathname).toBe(home));
  expect(router.state.location.state).toEqual({ attachment: top.id });

  await act(() => router.navigate(-1));
  await user.click(await screen.findByRole("link", { name: "数据库备份.pdf" }));
  expect(await heading("Guide")).toBeTruthy();
  expect(router.state.location.state).toEqual({ attachment: report.id });
  await waitFor(() => expect(document.activeElement?.getAttribute("data-attachment")).toBe(report.id));

  await act(() => router.navigate(-1));
  await user.click(await screen.findByRole("link", { name: "数据库设计" }));
  const page = await heading("数据库设计");
  expect(router.state.location.state).toEqual({ arrived: true });
  await waitFor(() => expect(document.activeElement).toBe(page));
});

test("hits of one name are told apart: by where they are, and by the end of their id where that is the same too, as Where to look tells notebooks of one name", async () => {
  const twin: Notebook = { ...notebookJSON, id: "0199a2b4-0000-7000-8000-0000000000c3" };
  const deeper = pageNode(31, "Notes", guide);
  const elsewhere: TreeNode = { ...pageNode(32, "Notes"), notebook_id: twin.id };
  const server = pageServer({ nodes: [guide, install, notes, deeper] });
  server.others = [twin];
  server.otherNodes = [elsewhere];
  renderApp(searchPath("notes"), server.app);
  await screen.findByRole("list", { name: "Search results" });

  const names = hits().map((row) => within(row).getByRole("link").getAttribute("aria-label"));
  expect(names).toEqual(["Notes (Plans, ID 000132)", "Notes (Plans / Guide)", "Notes (Plans, ID 000104)"]);
  expect(hits()[0]?.textContent).toContain("Plans · ID 000132");
  expect(
    within(hits()[0] as HTMLElement)
      .getByRole("link")
      .getAttribute("href")
  ).toBe(`/lab/notebooks/${twin.id}/pages/${elsewhere.id}`);
  expect(hits()[1]?.textContent).not.toContain("ID ");
  const scope = screen.getByRole("combobox", { name: "Where to look" });
  await waitFor(() =>
    expect([...scope.querySelectorAll("option")].map((option) => option.textContent)).toEqual([
      "All notebooks",
      "Plans (ID 0000c1)",
      "Plans (ID 0000c3)",
    ])
  );
});

/** hitOf is a hit of a page of Plans' named as node, of no ranges. */
function hitOf(node: TreeNode): SearchHit {
  return {
    node: { id: node.id, notebook_id: notebookJSON.id, kind: "page", name: node.name },
    ancestors: [],
    title_hits: [],
    excerpt: null,
    changed_at: node.updated_at,
    attachment: null,
  };
}

test("a hit a later page lists again, a rename between them, is listed once, and said as no more; what More said goes as another search shows", async () => {
  const user = userEvent.setup();
  const pages: Record<string, SearchPage> = {
    first: {
      data: [hitOf(guide), hitOf(notes)],
      next_cursor: "second",
      count: { value: 3, kind: "about" },
      building: false,
    },
    second: { data: [hitOf(notes), hitOf(linux)], next_cursor: null, count: null, building: false },
  };
  const server = searched({
    "GET /api/v0/workspaces/lab/search": (request) =>
      json(pages[new URL(request.url).searchParams.get("cursor") ?? "first"]),
  });
  const { router } = renderApp(searchPath("n"), server.app);
  await screen.findByRole("list", { name: "Search results" });
  expect(statusLine().textContent).toBe("Results: about 3.");

  await user.click(screen.getByRole("button", { name: "More results" }));
  await waitFor(() => expect(hits()).toHaveLength(3));
  expect(hits().map((row) => row.textContent?.split("Plans")[0])).toEqual(["Guide", "Notes", "Linux"]);
  expect(told().textContent).toBe("1 more found.");
  expect(statusLine().textContent).toBe("Results: 3.");
  // Read whole, the focus goes to the first it added.
  expect(document.activeElement).toBe(screen.getByRole("link", { name: "Linux" }));

  await act(() => router.navigate(searchPath("m")));
  await waitFor(() => expect(statusLine().textContent).toBe("Results: about 3."));
  expect(told().textContent).toBe("");
});

test("More reads the next page and says what it added; after a stretch that found none it is Keep looking, saying so, again with a space that does not break; the focus stays on it until the last page, then goes to the first hit added", async () => {
  const user = userEvent.setup();
  const pages = [50, 40, 30, 20].map((minute, i) => changed(pageNode(21 + i, `Page ${(i + 1).toString()}`), minute));
  const server = pageServer({ nodes: pages });
  for (const page of [pages[0], pages[3]]) {
    server.contents.set(page?.id ?? "", { content: "全文检索", revision: 1 });
  }
  server.searchWalk = 1;
  renderApp(searchPath("检索"), server.app);
  await screen.findByRole("list", { name: "Search results" });
  expect(hits()).toHaveLength(1);
  expect(statusLine().textContent).toBe("Results: about 2.");
  expect(told().textContent).toBe("");

  await user.click(screen.getByRole("button", { name: "More results" }));
  const keepLooking = await screen.findByRole("button", { name: "Keep looking" });
  expect(told().textContent).toBe("Nothing found in this stretch; you can keep looking.");
  expect(document.activeElement).toBe(keepLooking);
  expect(statusLine().textContent).toBe("Results: about 2.");

  await user.click(keepLooking);
  await waitFor(() => expect(told().textContent).toBe(`Nothing found in this stretch; you can keep looking.${nbsp}`));
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Keep looking" }));

  await user.click(screen.getByRole("button", { name: "Keep looking" }));
  await waitFor(() => expect(hits()).toHaveLength(2));
  expect(told().textContent).toBe("1 more found.");
  expect(statusLine().textContent).toBe("Results: 2.");
  expect(screen.queryByRole("button", { name: /More results|Keep looking/ })).toBeNull();
  expect(document.activeElement).toBe(screen.getByRole("link", { name: "Page 4" }));
  expect(reads(server)).toEqual([
    "GET search 检索",
    "GET search 检索 more",
    "GET search 检索 more",
    "GET search 检索 more",
  ]);
});

test("a search asked again is read again, what changed elsewhere shown and its line said anew; gone back to, it shows as it was, unread; one before it is read again", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = searched();
  const { router } = renderApp(searchPath("数据库"), server.app);
  await screen.findByRole("list", { name: "Search results" });
  const box = screen.getByRole("searchbox", { name: "Search terms" });

  // Asked again unchanged: read, the line emptied as it is, then said again.
  const held = gate();
  server.searchWaits.first = held.promise;
  await user.click(box);
  await user.keyboard("{Enter}");
  await waitFor(() => expect(reads(server)).toHaveLength(2));
  expect(statusLine().textContent).toBe("");
  held.open();
  await waitFor(() => expect(statusLine().textContent).toBe("Results: 3."));
  expect(document.activeElement).toBe(box);

  // Deleted in another tab, then asked again.
  server.trashNode(storage.id);
  await user.keyboard("{Enter}");
  await waitFor(() => expect(hits()).toHaveLength(2));
  expect(statusLine().textContent).toBe("Results: 2.");
  expect(reads(server)).toHaveLength(3);

  // Back from a hit: as it was.
  await user.click(screen.getByRole("link", { name: "数据库设计" }));
  await heading("数据库设计");
  await act(() => router.navigate(-1));
  await screen.findByRole("list", { name: "Search results" });
  expect(hits()).toHaveLength(2);
  expect(reads(server)).toHaveLength(3);

  // Back to the one before, past SWR's deduping of its read: read again.
  await act(() => vi.advanceTimersByTimeAsync(2_000));
  await act(() => router.navigate(-1));
  await waitFor(() => expect(reads(server)).toHaveLength(4));
  await waitFor(() => expect(statusLine().textContent).toBe("Results: 2."));
});

test("Where to look lists all the notebooks, then each; a choice replaces the address, read in it, the focus staying on it", async () => {
  const user = userEvent.setup();
  const idea = { ...pageNode(41, "全文检索"), notebook_id: ideas.id };
  const server = pageServer({ nodes: [guide, changed(pageNode(42, "检索方法"), 30)] });
  server.others = [ideas];
  server.otherNodes = [idea];
  const { router } = renderApp(searchPath("检索"), server.app);
  await screen.findByRole("list", { name: "Search results" });
  expect(hits()).toHaveLength(2);
  const scope = screen.getByRole("combobox", { name: "Where to look" });
  expect((scope as HTMLSelectElement).value).toBe("");
  const entries = router.state.historyAction;

  await user.selectOptions(scope, "Ideas");
  await waitFor(() => expect(hits()).toHaveLength(1));
  expect(router.state.location.search).toBe(`?q=${encodeURIComponent("检索")}&notebook_id=${ideas.id}`);
  expect(router.state.historyAction).toBe("REPLACE");
  expect(entries).toBe("POP");
  expect(statusLine().textContent).toBe("Results: 1.");
  expect(document.activeElement).toBe(scope);
  expect(reads(server).at(-1)).toBe("GET search 检索 in Ideas");

  await user.selectOptions(scope, "All notebooks");
  await waitFor(() => expect(hits()).toHaveLength(2));
  expect(router.state.location.search).toBe(`?q=${encodeURIComponent("检索")}`);
  expect(document.activeElement).toBe(scope);
});

test("an address whose notebook_id is no id is not read; one of a notebook not seen is 404, the notebooks read again: each says the notebook is not found, with Search the whole workspace, which, gone with the focus, gives it to the title", async () => {
  const user = userEvent.setup();
  let listed = 0;
  const server = searched({
    "GET /api/v0/workspaces/lab/notebooks": () => {
      listed += 1;
      return json({ data: [notebookJSON] });
    },
  });
  const { router } = renderApp(searchPath("数据库", "nope"), server.app);

  expect((await screen.findByRole("alert")).textContent).toBe(
    "This notebook is not found: it does not exist, or you have no access to it."
  );
  expect(reads(server)).toEqual([]);
  const scope = screen.getByRole("combobox", { name: "Where to look" });
  await waitFor(() =>
    expect((scope as HTMLSelectElement).selectedOptions[0]?.textContent).toBe("A notebook not found")
  );
  const everywhere = screen.getByRole("link", { name: "Search the whole workspace" });
  expect(everywhere.getAttribute("href")).toBe(searchPath("数据库"));
  await user.click(everywhere);
  await screen.findByRole("list", { name: "Search results" });
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1 }));

  const before = listed;
  await act(() => router.navigate(searchPath("数据库", ideas.id)));
  expect((await screen.findByRole("alert")).textContent).toBe(
    "This notebook is not found: it does not exist, or you have no access to it."
  );
  expect(reads(server).at(-1)).toBe("GET search 数据库");
  expect(screen.getByRole("link", { name: "Search the whole workspace" })).toBeTruthy();
  await waitFor(() => expect(listed).toBe(before + 1));
});

test("a q the server refuses says why, under the title", async () => {
  const server = searched();
  const { router } = renderApp(searchPath("a b c d e f g h i"), server.app);
  expect((await screen.findByRole("alert")).textContent).toBe(
    "At most 256 bytes and 8 terms, and at most 64 ASCII letters and digits in a row: shorten it."
  );
  await act(() => router.navigate(searchPath("!! —")));
  await waitFor(() =>
    expect(screen.getByRole("alert").textContent).toBe(
      "Signs and punctuation alone find nothing: search for a letter, a digit or a CJK character."
    )
  );
});

test("an address without q says how to search, and reads nothing", async () => {
  const server = searched();
  renderApp("/lab/search", server.app);

  expect(await heading("Search")).toBeTruthy();
  await waitFor(() => expect(document.title).toBe("Search · Lab · Nerve Wiki"));
  expect(
    screen.getByText("Type what to find in the search box at the top of the left column, and press Enter.")
  ).toBeTruthy();
  expect(screen.queryByRole("combobox", { name: "Where to look" })).toBeNull();
  expect(reads(server)).toEqual([]);
});

test("a search the server fails says why, with Try again, which reads it again", async () => {
  const user = userEvent.setup();
  const server = searched();
  server.searchDown = true;
  renderApp(searchPath("数据库"), server.app);
  expect((await screen.findByRole("alert")).textContent).toBe("Something went wrong (internal).");
  expect(statusLine().textContent).toBe("");

  server.searchDown = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));
  await screen.findByRole("list", { name: "Search results" });
  expect(statusLine().textContent).toBe("Results: 3.");
});

test("a page More read as another search shows goes to its own: nothing is said of it, nor listed, in the other", async () => {
  const user = userEvent.setup();
  const pages = [50, 40, 30].map((minute, i) => changed(pageNode(21 + i, `Page ${(i + 1).toString()}`), minute));
  const server = pageServer({ nodes: pages });
  for (const page of pages) {
    server.contents.set(page.id, { content: "全文检索", revision: 1 });
  }
  server.searchWalk = 1;
  const { router } = renderApp(searchPath("检索"), server.app);
  await screen.findByRole("list", { name: "Search results" });
  const held = gate();
  server.searchWaits.next = held.promise;
  await user.click(screen.getByRole("button", { name: "More results" }));
  await waitFor(() => expect(reads(server)).toEqual(["GET search 检索", "GET search 检索 more"]));

  await act(() => router.navigate(searchPath("全文")));
  await waitFor(() => expect(statusLine().textContent).toBe("Results: about 3."));
  held.open();
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(hits()).toHaveLength(1);
  expect(told().textContent).toBe("");
});

test("a search that fails says why, with Try again; the workspace gone, the shell says the page is not found", async () => {
  const user = userEvent.setup();
  let workspaces = [workspaceJSON];
  let gone = false;
  const server = searched({
    "GET /api/v0/workspaces": () => json({ data: workspaces }),
    "GET /api/v0/workspaces/lab/search": () =>
      gone ? problem(404, "workspace.not_found") : problem(400, "bad_request"),
  });
  renderApp(searchPath("数据库"), server.app);
  expect((await screen.findByRole("alert")).textContent).toBe("The request was not understood.");

  gone = true;
  workspaces = [];
  await user.click(screen.getByRole("button", { name: "Try again" }));
  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
});

test("a search is not read again as the window has the focus back, on a connection, nor as the pages change", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const events = eventServer();
  const server = searched({ "GET /api/v0/events": events.answer });
  renderApp(searchPath("数据库"), withEvents(server.app), { eventHandlers });
  await screen.findByRole("list", { name: "Search results" });
  await waitFor(() => expect(events.streams).toHaveLength(1));

  events.last().hello();
  events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });
  await readAgain();
  act(() => void window.dispatchEvent(new Event("online")));
  await act(() => vi.advanceTimersByTimeAsync(6_000));

  expect(reads(server)).toEqual(["GET search 数据库"]);
});
