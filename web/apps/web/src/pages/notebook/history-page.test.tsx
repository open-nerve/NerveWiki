import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import type { NotebookRole } from "../../services/notebook.service";
import { eventHandlers } from "../../events/handlers";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, userJSON, workspaceJSON, type Answer } from "../../test/fakes";
import { changesetId, created, deleted, moved, renamed, reordered, restored, wrote } from "../../test/history-server";
import { assetNode, bob, guide, install, linux, notes, pageNode, pageServer } from "../../test/page-server";
import { readAgain } from "../../test/page-panel";
import { renderApp } from "../../test/render";

// A notebook's history (M8 design 4.7, 4.13; M8/P3 design 4.2).

const home = `/lab/notebooks/${notebookJSON.id}`;
const historyPath = `${home}/history`;

afterEach(() => {
  vi.useRealTimers();
});

/** gate is a wait a test opens: what waits for its promise goes on once open is called. */
function gate() {
  let open!: () => void;
  const promise = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { promise, open };
}

/**
 * logged is pageServer with four changesets in its history, the newest
 * last: Ada's creation of Guide and Install from the web, with a message;
 * Bob's rename of Notes from an MCP client; Ada's write of Guide from the
 * API; and Ada's revert of Bob's rename.
 */
function logged(role: NotebookRole = "admin", answers: Record<string, Answer> = {}) {
  const server = pageServer({ role, answers });
  const create = server.log("create", {
    at: "2026-10-05T08:01:00Z",
    message: "Set up the <guide>",
    changes: [created(guide), created(install)],
  });
  const rename = server.log("rename", {
    at: "2026-10-05T08:02:00Z",
    by: bob,
    client: "mcp:claude",
    changes: [renamed(notes, "Old notes")],
  });
  const write = server.log("write", { at: "2026-10-05T08:03:00Z", client: "api", changes: [wrote(guide, 1, 2)] });
  const undo = server.log("undo", {
    at: "2026-10-05T08:04:00Z",
    kind: "revert",
    reverts: rename.id,
    changes: [renamed({ ...notes, name: "Old notes" }, "Notes")],
  });
  rename.revertedBy = [undo.id];
  return { server, create, rename, write, undo };
}

/** rows are the changesets the history lists, each row's text. */
const rows = () => within(screen.getByRole("list", { name: "Changes" })).getAllByRole("listitem");

/** links are the main links of the history's rows, by their names. */
const linkNames = () => rows().map((row) => within(row).getAllByRole("link")[0]?.getAttribute("aria-label") ?? "");

test("the history lists its changesets newer first, each by its kind and who made it, leading to its page, named by when; where it came from, as long ago as it is, its message, its first changes and its counts; the page and its tab are named for it", async () => {
  // Shown two hours after the first.
  vi.useFakeTimers({ now: Date.parse("2026-10-05T10:01:30Z"), toFake: ["Date"] });
  const { server, create, rename, undo } = logged();
  renderApp(historyPath, server.app);

  expect(await screen.findByRole("heading", { level: 1, name: "History" })).toBeTruthy();
  await waitFor(() => expect(document.title).toBe("History · Plans · Nerve Wiki"));
  await screen.findByRole("list", { name: "Changes" });
  expect(linkNames()).toEqual([
    "Undo change · Ada · Oct 5, 2026, 8:04 AM",
    "Edit · Ada · Oct 5, 2026, 8:03 AM",
    "Edit · Bob · Oct 5, 2026, 8:02 AM",
    "Edit · Ada · Oct 5, 2026, 8:01 AM",
  ]);
  const [reverting, writing, renaming, creating] = rows() as HTMLElement[];
  // The visible text starts each name.
  expect(within(creating as HTMLElement).getAllByRole("link")[0]?.textContent).toBe("Edit · Ada");
  expect(
    within(creating as HTMLElement)
      .getAllByRole("link")[0]
      ?.getAttribute("href")
  ).toBe(`${historyPath}/${create.id}`);
  expect(creating?.textContent).toContain("Web · 2 hours ago");
  expect(
    within(creating as HTMLElement)
      .getByTitle("Oct 5, 2026, 8:01 AM")
      .getAttribute("dateTime")
  ).toBe("2026-10-05T08:01:00Z");
  // Its message as written, isolated from the text around it.
  expect((creating as HTMLElement).querySelector("bdi")?.textContent).toBe("Set up the <guide>");
  expect(creating?.textContent).toContain("Created “Guide”, created “Install”");
  expect(creating?.textContent).toContain("Nodes: 2, contents: 2");
  expect(renaming?.textContent).toContain("MCP (claude) · ");
  expect(renaming?.textContent).toContain("Renamed “Old notes” to “Notes”");
  expect(renaming?.textContent).toContain("Nodes: 1, contents: 0");
  expect(writing?.textContent).toContain("API · ");
  expect(writing?.textContent).toContain("Wrote “Guide”");
  expect(writing?.textContent).toContain("Nodes: 0, contents: 1");
  // A revert's says what it undid, leading there; the one undone leads to its newest revert.
  expect(
    within(reverting as HTMLElement)
      .getByRole("link", { name: "Undid “Edit · Bob · Oct 5, 2026, 8:02 AM”" })
      .getAttribute("href")
  ).toBe(`${historyPath}/${rename.id}`);
  expect(
    within(renaming as HTMLElement)
      .getByRole("link", { name: "Undone: Edit · Bob · Oct 5, 2026, 8:02 AM" })
      .getAttribute("href")
  ).toBe(`${historyPath}/${undo.id}`);
  expect(within(renaming as HTMLElement).getByRole("link", { name: /^Undone/ }).textContent).toBe("Undone");
  expect(server.sent).toContain("GET changesets");
});

test("the history speaks Chinese too: its rows' texts and names", async () => {
  const { server } = logged();
  server.app.preferences.setLocale("zh-CN");
  renderApp(historyPath, server.app);

  expect(await screen.findByRole("heading", { level: 1, name: "历史" })).toBeTruthy();
  const list = await screen.findByRole("list", { name: "改动" });
  const [reverting, writing, renaming, creating] = within(list).getAllByRole("listitem") as HTMLElement[];
  expect(
    within(creating as HTMLElement)
      .getAllByRole("link")[0]
      ?.getAttribute("aria-label")
  ).toBe("编辑 · Ada · 2026年10月5日 08:01");
  expect(creating?.textContent).toContain("网页 · ");
  expect(creating?.textContent).toContain("新建「Guide」、新建「Install」");
  expect(creating?.textContent).toContain("共 2 个节点、2 篇正文");
  expect(renaming?.textContent).toContain("MCP（claude） · ");
  expect(renaming?.textContent).toContain("改名「Old notes」为「Notes」");
  expect(writing?.textContent).toContain("接口 · ");
  expect(writing?.textContent).toContain("写了「Guide」的正文");
  expect(within(reverting as HTMLElement).getAllByRole("link")[0]?.textContent).toBe("撤销 · Ada");
  expect(
    within(reverting as HTMLElement).getByRole("link", { name: "撤销了「编辑 · Bob · 2026年10月5日 08:02」" })
  ).toBeTruthy();
  expect(
    within(renaming as HTMLElement).getByRole("link", { name: "已撤销：编辑 · Bob · 2026年10月5日 08:02" }).textContent
  ).toBe("已撤销");
  expect(within(renaming as HTMLElement).getByRole("button", { name: "只看此人（Bob）" }).textContent).toBe("只看此人");
});

// M8/P3 design review B-M4 (v0.1 design 13.2, item 17).
test("changesets of one kind by one account in the same minute are told apart by the end of their ids, which their rows show, and so is a revert's link to one of them; others in that minute not", async () => {
  const server = pageServer();
  const first = server.log("first", { id: changesetId(1), at: "2026-10-05T09:00:05Z", changes: [wrote(guide, 1, 2)] });
  server.log("second", { id: changesetId(2), at: "2026-10-05T09:00:40Z", changes: [wrote(guide, 2, 3)] });
  // Bob's, and Ada's of another kind, in that minute: no twins.
  server.log("bob", { at: "2026-10-05T09:00:20Z", by: bob, changes: [wrote(notes, 1, 2)] });
  server.log("undo", {
    at: "2026-10-05T09:00:50Z",
    kind: "revert",
    reverts: first.id,
    changes: [wrote(guide, 3, 4)],
  });
  renderApp(historyPath, server.app);

  await screen.findByRole("list", { name: "Changes" });
  expect(linkNames()).toEqual([
    "Undo change · Ada · Oct 5, 2026, 9:00 AM",
    "Edit · Ada · Oct 5, 2026, 9:00 AM, ID 0d0002",
    "Edit · Bob · Oct 5, 2026, 9:00 AM",
    "Edit · Ada · Oct 5, 2026, 9:00 AM, ID 0d0001",
  ]);
  const [undo, second, other, firstRow] = rows();
  expect(
    within(undo as HTMLElement).getByRole("link", { name: "Undid “Edit · Ada · Oct 5, 2026, 9:00 AM, ID 0d0001”" })
  ).toBeTruthy();
  expect(undo?.textContent).not.toContain("· ID ");
  expect(other?.textContent).not.toContain("· ID ");
  expect(second?.textContent).toContain(" · ID 0d0002");
  expect(firstRow?.textContent).toContain(" · ID 0d0001");
});

test("a revert whose changeset the purge took says so, leading nowhere; one trimmed is marked so", async () => {
  const server = pageServer();
  server.log("trimmed", { trimmed: true, changes: [deleted(linux)] });
  server.log("undo", { kind: "revert", reverts: "0199a2b4-0000-7000-8000-0000000dffff", changes: [restored(notes)] });
  renderApp(historyPath, server.app);

  await screen.findByRole("list", { name: "Changes" });
  const [undo, trimmed] = rows();
  expect(undo?.textContent).toContain("Undid a change the purge took");
  expect(within(undo as HTMLElement).getAllByRole("link")).toHaveLength(1);
  expect(trimmed?.textContent).toContain("Trimmed");
  expect(within(trimmed as HTMLElement).getAllByRole("link")).toHaveLength(1);
});

test("counts are counted up to 1,000, which says as many or more; the first three changes summed up, more said by an ellipsis", async () => {
  const server = pageServer();
  server.log("import", {
    kind: "import",
    counts: { nodes: 1000, contents: 999 },
    changes: [created(guide), created(install), created(linux)],
  });
  server.log("move", { changes: [moved(install, null)] });
  renderApp(historyPath, server.app);

  await screen.findByRole("list", { name: "Changes" });
  const [move, imported] = rows();
  expect(imported?.textContent).toContain("Created “Guide”, created “Install”, created “Linux”…");
  expect(imported?.textContent).toContain("Nodes: 1,000+, contents: 999");
  expect(move?.textContent).toContain("Moved “Install”");
  expect(move?.textContent).not.toContain("…");

  act(() => server.app.preferences.setLocale("zh-CN"));
  const list = await screen.findByRole("list", { name: "改动" });
  expect(within(list).getAllByRole("listitem")[1]?.textContent).toContain("共 1,000 多 个节点、999 篇正文");
});

test("a history that could not be read says why, and is read again on Try again; an empty one says so", async () => {
  const user = userEvent.setup();
  const { server } = logged();
  server.historyDown = true;
  renderApp(historyPath, server.app);

  const retry = await screen.findByRole("button", { name: "Try again" });
  expect(screen.getByRole("alert").textContent).not.toBe("");
  server.historyDown = false;
  await user.click(retry);

  expect(await screen.findByRole("list", { name: "Changes" })).toBeTruthy();
  expect(rows()).toHaveLength(4);
  expect(screen.queryByRole("alert")).toBeNull();
});

test("an empty history says so", async () => {
  renderApp(historyPath, pageServer().app);

  expect(await screen.findByText("No changes.")).toBeTruthy();
  expect(screen.queryByRole("list", { name: "Changes" })).toBeNull();
});

test("a reader sees the history as a writer does", async () => {
  const { server } = logged("reader");
  renderApp(historyPath, server.app);

  await screen.findByRole("list", { name: "Changes" });
  expect(rows()).toHaveLength(4);
  expect(screen.getByRole("button", { name: "Only this person (Bob)" })).toBeTruthy();
});

/** loggedMany is pageServer with count changesets in its history, Change 1 the oldest, a minute apart. */
function loggedMany(count: number) {
  const server = pageServer();
  for (let n = 1; n <= count; n++) {
    server.log(`Change ${n.toString()}`, { message: `Change ${n.toString()}`, changes: [wrote(guide, n, n + 1)] });
  }
  return server;
}

/** more is the history's More. */
const more = () => screen.getByRole("button", { name: "More changes" });

test("More reads the next fifty changesets after those read, and leaves once the history is read whole, the focus on the first it added; read again, the list is as long as it was", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = loggedMany(51);
  renderApp(historyPath, server.app);

  await screen.findByRole("list", { name: "Changes" });
  expect(rows()).toHaveLength(50);
  await user.click(more());

  await waitFor(() => expect(rows()).toHaveLength(51));
  expect(screen.queryByRole("button", { name: "More changes" })).toBeNull();
  expect(document.activeElement?.closest("li")?.textContent).toContain("Change 1Wrote");
  expect(server.sent.filter((each) => each.startsWith("GET changesets"))).toEqual([
    "GET changesets",
    "GET changesets below Change 2",
  ]);

  // Read again: as many pages as were read, from the first.
  const before = server.sent.length;
  await readAgain();
  await waitFor(() =>
    expect(server.sent.slice(before).filter((each) => each.startsWith("GET changesets"))).toEqual([
      "GET changesets",
      "GET changesets below Change 2",
    ])
  );
  expect(rows()).toHaveLength(51);
});

test("a More that leaves pages to read keeps the focus on itself; the last takes it to the first changeset it adds", async () => {
  const user = userEvent.setup();
  renderApp(historyPath, loggedMany(101).app);
  await screen.findByRole("list", { name: "Changes" });
  const button = more();

  await user.click(button);
  await waitFor(() => expect(rows()).toHaveLength(100));
  expect(document.activeElement).toBe(button);

  await user.click(button);
  await waitFor(() => expect(rows()).toHaveLength(101));
  expect(screen.queryByRole("button", { name: "More changes" })).toBeNull();
  expect(document.activeElement?.closest("li")?.textContent).toContain("Change 1Wrote");
});

/** logLater has another changeset logged in server, after the others: the newest. */
function logLater(server: ReturnType<typeof pageServer>) {
  server.log("Late", { message: "Late", changes: [wrote(notes, 1, 2)] });
}

test("a next page added while the list is read again has the list read again whole: SWR drops that read", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = loggedMany(51);
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  const next = gate();
  server.historyWaits.next = next.promise;
  await user.click(more());
  await waitFor(() => expect(more().getAttribute("aria-busy")).toBe("true"));

  // The list is read again, its first page's answer held, as the next page comes; a changeset logged meanwhile.
  const first = gate();
  server.historyWaits.first = first.promise;
  logLater(server);
  await readAgain();
  await waitFor(() => expect(server.sent.filter((each) => each === "GET changesets")).toHaveLength(2));
  act(() => next.open());
  await waitFor(() => expect(rows()).toHaveLength(51));
  server.historyWaits.first = undefined;
  act(() => first.open());

  await waitFor(() => expect(rows()).toHaveLength(52));
  expect(rows()[0]?.textContent).toContain("Late");
});

test("a next page read after a cursor the list read again no longer ends with is not added", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = loggedMany(51);
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  const next = gate();
  server.historyWaits.next = next.promise;
  const button = more();
  await user.click(button);
  await waitFor(() => expect(button.getAttribute("aria-busy")).toBe("true"));

  // A changeset logged meanwhile: the list's first page ends a changeset higher.
  logLater(server);
  await readAgain();
  await waitFor(() => expect(rows()[0]?.textContent).toContain("Late"));
  act(() => next.open());

  await waitFor(() => expect(button.getAttribute("aria-busy")).toBeNull());
  expect(rows()).toHaveLength(50);
  expect(rows().some((row) => row.textContent?.includes("Change 1Wrote"))).toBe(false);
});

test("a More that fails says why beside it, which reads again", async () => {
  const user = userEvent.setup();
  const server = loggedMany(51);
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  server.historyDown = true;

  await user.click(more());
  expect((await screen.findByRole("alert")).textContent).not.toBe("");
  server.historyDown = false;
  await user.click(more());

  await waitFor(() => expect(rows()).toHaveLength(51));
  expect(screen.queryByRole("alert")).toBeNull();
});

/** whoField and clientField are the history's filters. */
const whoField = () => screen.getByRole("combobox", { name: "Who" });
const clientField = () => screen.getByRole("combobox", { name: "Client" });

test("a filter chosen is the address's, replacing it: the list it keeps is read, said politely, the focus left on the filter; Everyone and Every client take it off", async () => {
  const user = userEvent.setup();
  const { server } = logged();
  const { router } = renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  expect([...whoField().querySelectorAll("option")].map((option) => option.textContent)).toEqual([
    "Everyone",
    "Ada",
    "Bob",
  ]);

  await user.selectOptions(whoField(), "Bob");

  await waitFor(() => expect(rows()).toHaveLength(1));
  expect(router.state.location.search).toBe(`?actor=${bob.user_id}`);
  expect(router.state.historyAction).toBe("REPLACE");
  expect(document.activeElement).toBe(whoField());
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Listed: 1."));
  expect(server.sent).toContain(`GET changesets by ${bob.user_id}`);
  // Bob's rows have no Only this person: the list is his.
  expect(screen.queryByRole("button", { name: "Only this person (Bob)" })).toBeNull();

  await user.selectOptions(clientField(), "API");
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("No changes."));
  expect(router.state.location.search).toBe(`?actor=${bob.user_id}&client=api`);
  expect(screen.getByText("No changes.", { selector: "p" })).toBeTruthy();
  expect(document.activeElement).toBe(clientField());

  await user.selectOptions(whoField(), "Everyone");
  await waitFor(() => expect(rows()).toHaveLength(1));
  expect(router.state.location.search).toBe("?client=api");
  await user.selectOptions(clientField(), "Every client");
  await waitFor(() => expect(rows()).toHaveLength(4));
  expect(router.state.location.search).toBe("");
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Listed: 4."));
});

test("the clients are each one, every MCP client, and the MCP clients read; MCP lists every MCP client's", async () => {
  const user = userEvent.setup();
  const { server } = logged();
  server.log("other mcp", { client: "mcp:cursor", changes: [wrote(notes, 1, 2)] });
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  expect([...clientField().querySelectorAll("option")].map((option) => option.textContent)).toEqual([
    "Every client",
    "Web",
    "API",
    "CLI",
    "Every MCP client",
    "MCP (cursor)",
    "MCP (claude)",
  ]);

  await user.selectOptions(clientField(), "Every MCP client");

  await waitFor(() => expect(rows()).toHaveLength(2));
  expect(server.sent).toContain("GET changesets from mcp");
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Listed: 2."));
});

test("Only this person lists the changesets of who made it, the address's, said politely, the focus on Who, which holds the choice; its name says who", async () => {
  const user = userEvent.setup();
  const { server } = logged();
  const { router } = renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });

  await user.click(screen.getByRole("button", { name: "Only this person (Bob)" }));

  await waitFor(() => expect(rows()).toHaveLength(1));
  expect(router.state.location.search).toBe(`?actor=${bob.user_id}`);
  expect((whoField() as HTMLSelectElement).value).toBe(bob.user_id);
  expect(document.activeElement).toBe(whoField());
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Listed: 1."));
});

// M8/P3B review A-M2.
test("Only this person of a list read before, which shows at once, leaves the focus on Who", async () => {
  const user = userEvent.setup();
  const { server } = logged();
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  await user.selectOptions(whoField(), "Bob");
  await waitFor(() => expect(rows()).toHaveLength(1));
  await user.selectOptions(whoField(), "Everyone");
  await waitFor(() => expect(rows()).toHaveLength(4));

  await user.click(screen.getByRole("button", { name: "Only this person (Bob)" }));

  await waitFor(() => expect(rows()).toHaveLength(1));
  expect(document.activeElement).toBe(whoField());
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Listed: 1."));
  expect(document.activeElement).toBe(whoField());
});

/** member is the member n of Lab, person, by email. */
const member = (person: { user_id: string; display_name: string }, email: string, n: number) => ({
  id: `0199a2b4-0000-7000-8000-00000000f00${n.toString()}`,
  user_id: person.user_id,
  role: "member",
  display_name: person.display_name,
  email,
  created_at: "2026-10-01T08:00:00Z",
});

test("the accounts to choose are the workspace's members, by their emails where two have a name, and those of the changesets read that are not among them", async () => {
  const user = userEvent.setup();
  const other = { user_id: "0199a2b4-0000-7000-8000-0000000000aa", display_name: "Ada" };
  const { server } = logged("admin", {
    "GET /api/v0/workspaces/lab/members": () =>
      json({
        data: [
          member({ user_id: userJSON.id, display_name: "Ada" }, "ada@example.com", 1),
          member(other, "ada2@example.com", 2),
          member(bob, "bob@example.com", 3),
        ],
      }),
  });
  // Of an account no longer a member.
  server.log("former", { by: { user_id: "0199a2b4-0000-7000-8000-0000000000ab", display_name: "Zed" } });
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });

  await waitFor(() =>
    expect([...whoField().querySelectorAll("option")].map((option) => option.textContent)).toEqual([
      "Everyone",
      "Ada (ada@example.com)",
      "Ada (ada2@example.com)",
      "Bob",
      "Zed",
    ])
  );
  await user.selectOptions(whoField(), "Ada (ada2@example.com)");
  await waitFor(() => expect(screen.getByText("No changes.", { selector: "p" })).toBeTruthy());
  expect(server.sent).toContain(`GET changesets by ${other.user_id}`);
});

test("an account the address names that none read has is chosen all the same, by the end of its id", async () => {
  const { server } = logged();
  renderApp(`${historyPath}?actor=0199a2b4-0000-7000-8000-0000000000aa`, server.app);

  await screen.findByText("No changes.", { selector: "p" });
  expect(whoField().querySelector("option:checked")?.textContent).toBe("An account, ID 0000aa");
});

// M8/P3B reviews A-M5, B-3 (v0.1 design 13.2, item 17).
test("two accounts of one name are told apart: their changesets in a minute by the end of their ids, their Only this person as Who tells them apart", async () => {
  const user = userEvent.setup();
  const other = { user_id: "0199a2b4-0000-7000-8000-0000000000aa", display_name: "Ada" };
  const server = pageServer();
  server.log("mine", { id: changesetId(21), at: "2026-10-05T09:00:05Z", changes: [wrote(guide, 1, 2)] });
  server.log("theirs", { id: changesetId(22), at: "2026-10-05T09:00:40Z", by: other, changes: [wrote(guide, 2, 3)] });
  server.log("undo", {
    at: "2026-10-05T09:01:00Z",
    by: bob,
    kind: "revert",
    reverts: changesetId(22),
    changes: [wrote(guide, 3, 4)],
  });
  const { router } = renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });

  expect(linkNames()).toEqual([
    "Undo change · Bob · Oct 5, 2026, 9:01 AM",
    "Edit · Ada · Oct 5, 2026, 9:00 AM, ID 0d0022",
    "Edit · Ada · Oct 5, 2026, 9:00 AM, ID 0d0021",
  ]);
  expect(screen.getByRole("link", { name: "Undid “Edit · Ada · Oct 5, 2026, 9:00 AM, ID 0d0022”" })).toBeTruthy();
  const [, theirs, mine] = rows();
  expect(
    within(theirs as HTMLElement).getByRole("button", { name: "Only this person (Ada, ID 0000aa)" }).textContent
  ).toBe("Only this person");
  expect(within(mine as HTMLElement).getByRole("button", { name: "Only this person (Ada, ID 000001)" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Only this person (Bob)" })).toBeTruthy();

  await user.click(screen.getByRole("button", { name: "Only this person (Ada, ID 0000aa)" }));

  await waitFor(() => expect(rows()).toHaveLength(1));
  expect(router.state.location.search).toBe(`?actor=${other.user_id}`);
  expect((whoField() as HTMLSelectElement).value).toBe(other.user_id);
});

test("two accounts of one name in the workspace are told apart by their emails, as Who tells them apart, in Chinese too", async () => {
  const other = { user_id: "0199a2b4-0000-7000-8000-0000000000aa", display_name: "Ada" };
  const server = pageServer({
    answers: {
      "GET /api/v0/workspaces/lab/members": () =>
        json({
          data: [
            member({ user_id: userJSON.id, display_name: "Ada" }, "ada@example.com", 1),
            member(other, "ada2@example.com", 2),
          ],
        }),
    },
  });
  server.log("mine", { at: "2026-10-05T09:00:05Z", changes: [wrote(guide, 1, 2)] });
  server.log("theirs", { at: "2026-10-05T10:00:40Z", by: other, changes: [wrote(guide, 2, 3)] });
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });

  // Not in one minute: their rows are not twins.
  expect(linkNames()).toEqual(["Edit · Ada · Oct 5, 2026, 10:00 AM", "Edit · Ada · Oct 5, 2026, 9:00 AM"]);
  expect(await screen.findByRole("button", { name: "Only this person (Ada, ada2@example.com)" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Only this person (Ada, ada@example.com)" })).toBeTruthy();

  act(() => server.app.preferences.setLocale("zh-CN"));
  expect(await screen.findByRole("button", { name: "只看此人（Ada，ada2@example.com）" })).toBeTruthy();
});

test("a page's history, from the address, is said by its name, with a way to take it off: Remove, the focus to the title", async () => {
  const user = userEvent.setup();
  const { server } = logged();
  const { router } = renderApp(`${historyPath}?page=${guide.id}`, server.app);

  await waitFor(() => expect(rows()).toHaveLength(2));
  expect(server.sent).toContain("GET changesets of Guide");
  expect(screen.getByText("Page: Guide")).toBeTruthy();
  const remove = screen.getByRole("button", { name: "Remove the filter “Page: Guide”" });
  expect(remove.textContent).toBe("Remove");

  await user.click(remove);

  await waitFor(() => expect(rows()).toHaveLength(4));
  expect(router.state.location.search).toBe("");
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "History" }));
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Listed: 4."));
});

test("a page's history whose page the tree has not, in the trash, says it is deleted", async () => {
  const { server } = logged();
  server.trashNode(linux.id);
  server.log("deletion", { changes: [deleted(linux)] });
  renderApp(`${historyPath}?page=${linux.id}`, server.app);

  await waitFor(() => expect(rows()).toHaveLength(1));
  expect(screen.getByText("Page: a deleted page")).toBeTruthy();
});

test.each([
  { filter: "a client the server does not take", query: "client=nothing", sent: true },
  { filter: "an account that is no id", query: "actor=ada", sent: false },
  { filter: "a page that is no id", query: "page=guide", sent: false },
  { filter: "a page whose id has more after it", query: `page=${guide.id}0`, sent: false },
])(
  "an address with $filter says the filters cannot be read, with Clear filters, which lists the whole history",
  async ({ query, sent }) => {
    const user = userEvent.setup();
    const { server } = logged();
    const { router } = renderApp(`${historyPath}?${query}`, server.app);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toBe("These filters cannot be read: the address has one this history does not take.");
    expect(server.sent.some((each) => each.startsWith("GET changesets"))).toBe(sent);
    expect(screen.queryByRole("list", { name: "Changes" })).toBeNull();

    await user.click(screen.getByRole("button", { name: "Clear filters" }));

    await waitFor(() => expect(rows()).toHaveLength(4));
    expect(router.state.location.search).toBe("");
    expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "History" }));
  }
);

test("an address with a page not of the notebook, or purged, says so, with Clear filters", async () => {
  const user = userEvent.setup();
  const { server } = logged();
  renderApp(`${historyPath}?page=${pageNode(77, "Elsewhere").id}`, server.app);

  expect((await screen.findByRole("alert")).textContent).toBe(
    "The page filtered by is not in this notebook, or the purge took it."
  );
  await user.click(screen.getByRole("button", { name: "Clear filters" }));

  await waitFor(() => expect(rows()).toHaveLength(4));
});

test("a row read away as the focus is on its link gives the focus to the title", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const { server, write } = logged();
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  act(() => screen.getByRole("link", { name: "Edit · Ada · Oct 5, 2026, 8:03 AM" }).focus());

  server.changesets = server.changesets.filter((changeset) => changeset !== write);
  await readAgain();

  await waitFor(() => expect(rows()).toHaveLength(3));
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "History" }));
});

const pagesEvent = (notebook: string, tree: boolean) => ({
  workspace_id: workspaceJSON.id,
  notebook_id: notebook,
  tree,
  pages: tree ? [] : [{ id: guide.id, revision: 2 }],
});

test("the history is read again as the notebook's pages change, a content written too, at most once in five seconds, and on each connection; not for another notebook's", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const events = eventServer();
  const { server } = logged("admin", { "GET /api/v0/events": events.answer });
  renderApp(historyPath, withEvents(server.app), { eventHandlers });
  await screen.findByRole("list", { name: "Changes" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const reads = () => server.sent.filter((each) => each === "GET changesets");
  events.last().hello();
  await waitFor(() => expect(reads()).toHaveLength(2));

  server.log("elsewhere", { changes: [wrote(guide, 2, 3)] });
  events.last().send("pages", pagesEvent(notebookJSON.id, false));
  await waitFor(() => expect(rows()).toHaveLength(5));
  expect(reads()).toHaveLength(3);

  // Within the interval, read at its end.
  events.last().send("pages", pagesEvent(notebookJSON.id, true));
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  expect(reads()).toHaveLength(3);
  await act(() => vi.advanceTimersByTimeAsync(5_000));
  await waitFor(() => expect(reads()).toHaveLength(4));

  events.last().send("pages", pagesEvent("0199a2b4-0000-7000-8000-0000000000c2", true));
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  expect(reads()).toHaveLength(4);
});

// The negative controls (M8/P3 design section 6).

test("what a choice lists is said once it is read, and said again with a space that does not break when the same; nothing is said of a list shown or read again unasked", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const { server } = logged();
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  const status = screen.getByRole("status");
  await readAgain();
  await waitFor(() => expect(server.sent.filter((each) => each === "GET changesets")).toHaveLength(2));
  expect(status.textContent).toBe("");

  await user.selectOptions(clientField(), "API");
  await waitFor(() => expect(status.textContent).toBe("Listed: 1."));
  await user.selectOptions(clientField(), "Every MCP client");
  await waitFor(() => expect(status.textContent).toBe(`Listed: 1.${String.fromCharCode(0xa0)}`));
  await user.selectOptions(clientField(), "Web");
  await waitFor(() => expect(status.textContent).toBe("Listed: 2."));
});

test("Only this person keeps the other filters; Remove of a page's, those it has not", async () => {
  const user = userEvent.setup();
  const { server } = logged();
  const { router } = renderApp(`${historyPath}?client=mcp%3Aclaude`, server.app);
  await waitFor(() => expect(rows()).toHaveLength(1));

  await user.click(screen.getByRole("button", { name: "Only this person (Bob)" }));

  await waitFor(() => expect(router.state.location.search).toBe(`?actor=${bob.user_id}&client=mcp%3Aclaude`));
  await act(() => router.navigate(`${historyPath}?actor=${userJSON.id}&page=${guide.id}`));
  await waitFor(() => expect(rows()).toHaveLength(2));
  await user.click(screen.getByRole("button", { name: "Remove the filter “Page: Guide”" }));
  await waitFor(() => expect(router.state.location.search).toBe(`?actor=${userJSON.id}`));
});

test("a client the address names that none read has is chosen all the same", async () => {
  const { server } = logged();
  renderApp(`${historyPath}?client=mcp%3Azed`, server.app);

  await screen.findByText("No changes.", { selector: "p" });
  expect(clientField().querySelector("option:checked")?.textContent).toBe("MCP (zed)");
});

test("a page's history whose tree is not read yet says its page by an ellipsis", async () => {
  const { server } = logged();
  server.nodesDown = true;
  renderApp(`${historyPath}?page=${guide.id}`, server.app);

  await waitFor(() => expect(rows()).toHaveLength(2));
  expect(screen.getByText("Page: …")).toBeTruthy();
});

test("Undone leads to the newest revert of a changeset undone more than once", async () => {
  const { server, rename } = logged();
  const again = server.log("again", { at: "2026-10-05T08:05:00Z", kind: "revert", reverts: rename.id });
  rename.revertedBy = [again.id, ...rename.revertedBy];
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });

  expect(screen.getByRole("link", { name: "Undone: Edit · Bob · Oct 5, 2026, 8:02 AM" }).getAttribute("href")).toBe(
    `${historyPath}/${again.id}`
  );
});

test("a revert's link to a changeset not listed is told apart by the end of its id from a row of its name, which is too", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  const first = server.log("first", {
    id: changesetId(11),
    at: "2026-10-05T09:00:05Z",
    client: "api",
    changes: [wrote(guide, 1, 2)],
  });
  server.log("second", { id: changesetId(12), at: "2026-10-05T09:00:40Z", changes: [wrote(guide, 2, 3)] });
  server.log("undo", {
    at: "2026-10-05T09:01:00Z",
    by: bob,
    kind: "revert",
    reverts: first.id,
    changes: [wrote(guide, 3, 4)],
  });
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });

  // The API's first is not listed of the web's.
  await user.selectOptions(clientField(), "Web");

  await waitFor(() => expect(rows()).toHaveLength(2));
  expect(linkNames()).toEqual([
    "Undo change · Bob · Oct 5, 2026, 9:01 AM",
    "Edit · Ada · Oct 5, 2026, 9:00 AM, ID 0d0012",
  ]);
  expect(screen.getByRole("link", { name: "Undid “Edit · Ada · Oct 5, 2026, 9:00 AM, ID 0d0011”" })).toBeTruthy();
});

test("an account or a page in capitals is the id all the same, as the server reads it: read, and named", async () => {
  const { server } = logged();
  const { router } = renderApp(`${historyPath}?actor=${bob.user_id.toUpperCase()}`, server.app);

  await waitFor(() => expect(rows()).toHaveLength(1));
  expect(server.sent).toContain(`GET changesets by ${bob.user_id}`);
  expect(whoField().querySelector("option:checked")?.textContent).toBe("Bob");
  // The list is his: no Only this person.
  expect(screen.queryByRole("button", { name: "Only this person (Bob)" })).toBeNull();

  await act(() => router.navigate(`${historyPath}?page=${guide.id.toUpperCase()}`));
  await waitFor(() => expect(rows()).toHaveLength(2));
  expect(screen.getByText("Page: Guide")).toBeTruthy();
});

// M8/P3B review B-1.
test("a changeset of more nodes than its changes listed says so by an ellipsis, though they list as many items or versions as it counts", async () => {
  const server = pageServer();
  // A rename of Notes that wrote three pages' links: the three writes are listed, by node id; the rename not.
  server.log("rename", {
    changes: [renamed(notes, "Old notes"), wrote(guide, 1, 2), wrote(install, 1, 2), wrote(linux, 1, 2)],
  });
  server.log("write", { changes: [wrote(guide, 2, 3), wrote(install, 2, 3), wrote(linux, 2, 3)] });
  // Three renames listed, and a write of Notes not.
  server.log("renames", {
    changes: [renamed(guide, "G"), renamed(install, "I"), renamed(linux, "L"), wrote(notes, 1, 2)],
  });
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });

  const [renames, write, rename] = rows();
  expect(renames?.textContent).toContain("Renamed “G” to “Guide”, renamed “I” to “Install”, renamed “L” to “Linux”…");
  expect(renames?.textContent).toContain("Nodes: 3, contents: 1");
  expect(rename?.textContent).toContain("Wrote “Guide”, wrote “Install”, wrote “Linux”…");
  expect(rename?.textContent).toContain("Nodes: 1, contents: 3");
  expect(write?.textContent).toContain("Wrote “Guide”, wrote “Install”, wrote “Linux”");
  expect(write?.textContent).not.toContain("…");
});

// M8/P3B review B-4 (M8/P3 design 4.7).
test("a row says what each kind of change did, in Chinese too", async () => {
  const server = pageServer();
  server.log("tidy", { changes: [restored(guide), deleted(install), reordered(linux)] });
  server.log("move", { changes: [moved(notes, guide.id)] });
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });

  const [move, tidy] = rows();
  expect(tidy?.textContent).toContain("Restored “Guide” from the trash, deleted “Install”, reordered “Linux”");
  expect(move?.textContent).toContain("Moved “Notes”");

  act(() => server.app.preferences.setLocale("zh-CN"));
  const list = await screen.findByRole("list", { name: "改动" });
  const [moveZh, tidyZh] = within(list).getAllByRole("listitem");
  expect(tidyZh?.textContent).toContain("从回收站恢复「Guide」、删除「Install」、调整「Linux」的次序");
  expect(moveZh?.textContent).toContain("移动「Notes」");
});

// M8/P3B review B-5.
test("More of a history filtered reads the next of that filter", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  for (let n = 1; n <= 51; n++) {
    server.log(`Bob ${n.toString()}`, { by: bob, message: `Bob ${n.toString()}`, changes: [wrote(guide, n, n + 1)] });
    server.log(`Ada ${n.toString()}`, { message: `Ada ${n.toString()}`, changes: [wrote(notes, n, n + 1)] });
  }
  renderApp(`${historyPath}?actor=${bob.user_id}`, server.app);
  await waitFor(() => expect(rows()).toHaveLength(50));

  await user.click(more());

  await waitFor(() => expect(rows()).toHaveLength(51));
  expect(rows().map((row) => row.querySelector("bdi")?.textContent?.split(" ")[0])).toEqual(
    Array.from({ length: 51 }, () => "Bob")
  );
  expect(server.sent).toContain(`GET changesets by ${bob.user_id} below Bob 2`);
});

// M8/P3B review A-N1.
test("a More that failed says nothing once another filter is chosen, nor once its list is shown again", async () => {
  const user = userEvent.setup();
  const server = loggedMany(51);
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  server.historyDown = true;
  await user.click(more());
  await screen.findByRole("alert");
  server.historyDown = false;

  await user.selectOptions(clientField(), "API");

  await waitFor(() => expect(screen.getByText("No changes.", { selector: "p" })).toBeTruthy());
  expect(screen.queryByRole("alert")).toBeNull();
  await user.selectOptions(clientField(), "Every client");
  await waitFor(() => expect(rows()).toHaveLength(50));
  expect(screen.queryByRole("alert")).toBeNull();
});

// M8/P3B review C-M1.
test("a More read for one filter, another chosen while it is out, adds to the list it was read for: not to the one shown, which is not busy", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  let n = 0;
  const log = (client: string, count: number) => {
    for (let i = 0; i < count; i++) {
      n += 1;
      const at = new Date(Date.parse("2026-10-05T08:00:00Z") + n * 60_000).toISOString();
      const message = `${client} ${n.toString()}`;
      server.log(message, { at, client, message, changes: [wrote(guide, n, n + 1)] });
    }
  };
  log("api", 5);
  log("web", 5);
  log("api", 50);
  renderApp(`${historyPath}?client=api`, server.app);
  await waitFor(() => expect(rows()).toHaveLength(50));
  await user.selectOptions(clientField(), "Every client");
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Listed: 50, and more."));
  const next = gate();
  server.historyWaits.next = next.promise;
  await user.click(more());
  await waitFor(() => expect(more().getAttribute("aria-busy")).toBe("true"));

  await user.selectOptions(clientField(), "API");
  await waitFor(() => expect(rows()[0]?.textContent).toContain("api 60"));
  expect(more().getAttribute("aria-busy")).toBeNull();
  // The API's More reads its own, the other's out.
  server.historyWaits.next = undefined;
  await user.click(more());
  await waitFor(() => expect(rows()).toHaveLength(55));
  expect(server.sent).toContain("GET changesets from api below api 11");
  act(() => next.open());
  await act(async () => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(rows()).toHaveLength(55);
  expect(rows().filter((row) => row.querySelector("bdi")?.textContent?.startsWith("web"))).toEqual([]);
  await user.selectOptions(clientField(), "Every client");
  await waitFor(() => expect(rows()).toHaveLength(60));
});

// M8/P3B review A-N1.
test("a More that fails once another filter is chosen says nothing of it", async () => {
  const user = userEvent.setup();
  const server = loggedMany(51);
  renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  const next = gate();
  server.historyWaits.next = next.promise;
  server.historyDown = true;
  await user.click(more());
  await waitFor(() => expect(more().getAttribute("aria-busy")).toBe("true"));
  server.historyDown = false;
  await user.selectOptions(clientField(), "API");
  await waitFor(() => expect(screen.getByText("No changes.", { selector: "p" })).toBeTruthy());
  server.historyDown = true;

  act(() => next.open());
  await act(async () => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(screen.queryByRole("alert")).toBeNull();
  server.historyDown = false;
});

test("a More read whose list is no longer shown as it ends leaves the focus where it is: not on a row of the list shown", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  server.log("Change 1", { message: "Change 1", changes: [wrote(guide, 1, 2)] });
  for (let n = 2; n <= 51; n++) {
    server.log(`Change ${n.toString()}`, { message: `Change ${n.toString()}`, changes: [wrote(notes, n, n + 1)] });
  }
  const { router } = renderApp(historyPath, server.app);
  await screen.findByRole("list", { name: "Changes" });
  const next = gate();
  server.historyWaits.next = next.promise;
  await user.click(more());
  await waitFor(() => expect(more().getAttribute("aria-busy")).toBe("true"));

  // Guide's history, as Back would show it, lists Change 1, which the More reads: no More there, the focus to the title.
  await act(() => router.navigate(`${historyPath}?page=${guide.id}`));
  await waitFor(() => expect(rows()).toHaveLength(1));
  const heading = screen.getByRole("heading", { level: 1, name: "History" });
  expect(document.activeElement).toBe(heading);
  server.historyWaits.next = undefined;
  act(() => next.open());
  await act(async () => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(document.activeElement).toBe(heading);
  await act(() => router.navigate(historyPath));
  await waitFor(() => expect(rows()).toHaveLength(51));
});

// M8/P3B review B-7.
test("an attachment's history is said by its name", async () => {
  const picture = assetNode(9, "a.png", guide);
  const server = pageServer({ nodes: [guide, install, linux, notes, picture] });
  server.log("upload", { changes: [created(picture)] });
  renderApp(`${historyPath}?page=${picture.id}`, server.app);

  await waitFor(() => expect(rows()).toHaveLength(1));
  expect(screen.getByText("Page: a.png")).toBeTruthy();
});
