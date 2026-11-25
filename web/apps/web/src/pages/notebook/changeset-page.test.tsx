import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { arrived } from "../../app/arrival";
import { docFactsClass, headActionsClass, noteClass } from "../../app/page-classes";
import { eventHandlers } from "../../events/handlers";
import type { NotebookRole } from "../../services/notebook.service";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, workspaceJSON, type Answer } from "../../test/fakes";
import { changesetId, created, deleted, moved, renamed, reordered, restored, wrote } from "../../test/history-server";
import { assetNode, bob, guide, install, linux, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { readAgain } from "../../test/page-panel";
import { renderApp } from "../../test/render";
import { cn } from "../../lib/cn";
import { docTitleClass, headCrumbsClass, headRowClass } from "../page/page-layout";

// A changeset's page (M8 design 4.7, 4.13; M8/P3 design 4.3).

const home = `/lab/notebooks/${notebookJSON.id}`;

afterEach(() => {
  vi.useRealTimers();
});

const changesetPath = (id: string) => `${home}/history/${id}`;

const picture = assetNode(5, "a.png", guide);
const ideas = pageNode(6, "Ideas");
const old = pageNode(7, "Old");
const draft = pageNode(8, "Draft", notes);
/** The id of a page the purge took, which Ideas was under. */
const purged = "0199a2b4-0000-7000-8000-000000000999";

/**
 * reworked is pageServer with Ada's rework in its history, from the CLI,
 * with a message, written again five minutes after it was made: it
 * created Install and a.png, moved Linux from under Notes and Ideas from
 * under a page the purge took, renamed Notes, reordered Guide, deleted Old
 * and wrote Draft, both in the trash now.
 */
function reworked(role: NotebookRole = "admin", answers: Record<string, Answer> = {}) {
  const server = pageServer({ role, answers, nodes: [guide, install, linux, notes, picture, ideas, old, draft] });
  const changes = [
    created(install),
    moved(linux, notes.id),
    renamed(notes, "Old notes"),
    reordered(guide),
    created(picture),
    moved(ideas, purged),
    deleted(old),
    wrote(draft, 1, 2),
  ];
  server.trashNode(old.id);
  server.trashNode(draft.id);
  const rework = server.log("rework", {
    at: "2026-10-05T08:01:00Z",
    updatedAt: "2026-10-05T08:06:00Z",
    client: "cli",
    message: "Tidy <up>",
    changes,
  });
  return { server, rework };
}

/** nodes are the nodes the changeset's page lists, each one's row, the list named list. */
const nodes = (list = "Nodes of this change") =>
  within(screen.getByRole("list", { name: list })).getAllByRole("listitem");

/** links are the links of the row of the node named name, each by its name and where it leads. */
const links = (name: string) =>
  within(rowOf(name))
    .getAllByRole("link")
    .map((link) => [link.getAttribute("aria-label") ?? link.textContent, link.getAttribute("href")]);

/** rowOf is the row of the node named name, of the list named list. */
const rowOf = (name: string, list?: string) =>
  nodes(list).find((row) => row.textContent?.startsWith(name)) as HTMLElement;

// Arrived at, the changeset not read yet (the settings' Back, a job's row of S2): its title, drawn once the changeset
// is read, takes the focus then, not the body (M13/P3/S1, decision 835; review A's M1).
test("a changeset arrived at before it is read gives its title the focus once it is", async () => {
  const { server, rework } = reworked();
  const { router } = renderApp(home, server.app);
  await screen.findByRole("heading", { level: 1, name: "Plans" });

  await act(() => router.navigate(changesetPath(rework.id), { state: arrived }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Edit · Ada" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

test("a changeset's page says what it was and who made it, from where, when it was made and last written, why, and its counts, under the history; its tab is named for it", async () => {
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);

  const heading = await screen.findByRole("heading", { level: 1, name: "Edit · Ada" });
  await waitFor(() => expect(document.title).toBe("Edit · Ada · History · Plans · Nerve Wiki"));
  const crumbs = screen.getByRole("navigation", { name: "Breadcrumb" });
  expect(
    within(crumbs)
      .getAllByRole("link")
      .map((link) => [link.textContent, link.getAttribute("href")])
  ).toEqual([
    ["Plans", home],
    ["History", `${home}/history`],
  ]);
  expect(within(crumbs).getByText("Edit · Ada · Oct 5, 2026, 8:01 AM").getAttribute("aria-current")).toBe("page");
  const about = heading.nextElementSibling as HTMLElement;
  expect(about.textContent).toContain("Client: CLI");
  expect(about.textContent).toContain("Made Oct 5, 2026, 8:01 AM · Last written Oct 5, 2026, 8:06 AM");
  expect(about.querySelector("bdi")?.textContent).toBe("Tidy <up>");
  expect(about.textContent).toContain("Nodes: 7, contents: 2");
  expect(about.textContent).not.toContain("Undone");
  expect(screen.getByRole("button", { name: "Undo change" })).toBeTruthy();
  expect(screen.queryByText(/undoing it again/)).toBeNull();
  // The history in the left column leads to the history, which this page is under, not.
  const places = screen.getByRole("navigation", { name: "More of Plans" });
  expect(within(places).getByRole("link", { name: "History" }).getAttribute("aria-current")).toBeNull();
  expect(server.sent).toEqual(expect.arrayContaining(["GET changeset rework", "GET changes rework"]));
});

test("a changeset's nodes say what it did to each, by node id: each alive leads to it, a page to itself, an attachment to its page; one in the trash to its preview; each to its history; a content written to its version while its page is alive", async () => {
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);
  await screen.findByRole("list", { name: "Nodes of this change" });

  expect(nodes().map((row) => row.textContent)).toEqual([
    "GuideHistory of this nodeReordered",
    "InstallHistory of this nodeCreatedWrote its content",
    "LinuxHistory of this nodeMoved from “Notes” to “Install”",
    "NotesHistory of this nodeRenamed: “Old notes” → “Notes”",
    "a.pngHistory of this nodeCreated",
    "IdeasHistory of this nodeMoved from (purged) to the notebook's top level",
    "OldIn the trashHistory of this nodeDeleted",
    "DraftIn the trashHistory of this nodeWrote its content",
  ]);
  const historyOf = (id: string) => `${home}/history?page=${id}`;
  expect(links("Install")).toEqual([
    ["Install", pagePath(install.id)],
    ["History of this node: “Install”", historyOf(install.id)],
    ["Wrote its content: “Install”, version 1", `${pagePath(install.id)}/versions/1`],
  ]);
  expect(links("a.png")).toEqual([
    ["a.png", pagePath(guide.id)],
    ["History of this node: “a.png”", historyOf(picture.id)],
  ]);
  expect(links("Old")).toEqual([
    ["In the trash: “Old”", `${home}/trash/${old.id}`],
    ["History of this node: “Old”", historyOf(old.id)],
  ]);
  // A content of a page not alive leads nowhere: a version's page takes only the tree's pages.
  expect(links("Draft")).toEqual([
    ["In the trash: “Draft”", `${home}/trash/${draft.id}`],
    ["History of this node: “Draft”", historyOf(draft.id)],
  ]);
});

test("an attachment's link opens its page, its row taking the focus", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);
  await screen.findByRole("list", { name: "Nodes of this change" });

  await user.click(within(rowOf("a.png")).getByRole("link", { name: "a.png" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
  await waitFor(() => expect(document.activeElement?.textContent).toBe("a.png (opens in a new tab)"));
  expect(document.activeElement?.getAttribute("data-attachment")).toBe(picture.id);
});

test("a changeset's page speaks Chinese too", async () => {
  const { server, rework } = reworked();
  server.app.preferences.setLocale("zh-CN");
  renderApp(changesetPath(rework.id), server.app);

  expect(await screen.findByRole("heading", { level: 1, name: "编辑 · Ada" })).toBeTruthy();
  await screen.findByRole("list", { name: "这次改动的节点" });
  const list = "这次改动的节点";
  expect(rowOf("Linux", list).textContent).toContain("从「Notes」移到「Install」");
  expect(rowOf("Ideas", list).textContent).toContain("从（已清理）移到笔记本的顶层");
  expect(rowOf("Notes", list).textContent).toContain("改名：「Old notes」→「Notes」");
  expect(within(rowOf("Old", list)).getByRole("link", { name: "在回收站里：「Old」" }).textContent).toBe("在回收站里");
});

test("nodes of one name are told apart by the end of their ids, which their rows show, one in the trash too", async () => {
  const readme = pageNode(9, "Readme", guide);
  const other = pageNode(10, "Readme", notes);
  const server = pageServer({ nodes: [guide, install, linux, notes, readme, other] });
  const twice = server.log("twice", { changes: [created(readme), created(other), created(install)] });
  server.trashNode(other.id);
  renderApp(changesetPath(twice.id), server.app);
  await screen.findByRole("list", { name: "Nodes of this change" });

  const [first, second, third] = nodes() as HTMLElement[];
  expect(within(first as HTMLElement).getByRole("link", { name: "Install" })).toBeTruthy();
  expect(
    within(second as HTMLElement)
      .getAllByRole("link")
      .map((link) => link.getAttribute("aria-label"))
  ).toEqual([
    "Readme (ID 000109)",
    "History of this node: “Readme (ID 000109)”",
    "Wrote its content: “Readme (ID 000109)”, version 1",
  ]);
  expect(second?.textContent).toContain("ID 000109");
  expect(
    within(third as HTMLElement)
      .getAllByRole("link")
      .map((link) => link.getAttribute("aria-label"))
  ).toEqual(["In the trash: “Readme (ID 000110)”", "History of this node: “Readme (ID 000110)”"]);
  expect(third?.textContent).toContain("ID 000110");
});

// M8/P3B review B-4 (M8/P3 design 4.7).
test("a node restored from the trash says so, in Chinese too", async () => {
  const server = pageServer();
  const restore = server.log("restore", { changes: [restored(notes)] });
  renderApp(changesetPath(restore.id), server.app);
  await screen.findByRole("list", { name: "Nodes of this change" });

  expect(nodes().map((row) => row.textContent)).toEqual(["NotesHistory of this nodeRestored from the trash"]);
  act(() => server.app.preferences.setLocale("zh-CN"));
  await waitFor(() => expect(rowOf("Notes", "这次改动的节点").textContent).toContain("从回收站恢复"));
});

// M8/P3B review A-M4.
test("an attachment leads to its page as the tree has it now: one at the notebook's top level to the notebook's home", async () => {
  const photo = assetNode(9, "photo.png");
  const server = pageServer({ nodes: [guide, install, linux, notes, photo] });
  // The change moved it from Notes to Guide; another moved it to the top level since.
  const move = server.log("move", { changes: [moved({ ...photo, parent_id: guide.id }, notes.id)] });
  renderApp(changesetPath(move.id), server.app);
  await screen.findByRole("navigation", { name: "Pages of Plans" });
  await screen.findByRole("list", { name: "Nodes of this change" });

  expect(links("photo.png")[0]).toEqual(["photo.png", home]);
});

test("an attachment leads to where the change left it while the tree is not read", async () => {
  const photo = assetNode(9, "photo.png", guide);
  const server = pageServer({ nodes: [guide, install, linux, notes, photo] });
  server.nodesDown = true;
  const move = server.log("move", { changes: [moved(photo, notes.id)] });
  renderApp(changesetPath(move.id), server.app);
  await screen.findByRole("list", { name: "Nodes of this change" });

  expect(links("photo.png")[0]).toEqual(["photo.png", pagePath(guide.id)]);
});

test("a revert says what it undid, leading there, or that the purge took it; what undid a changeset, newest first, each leading there, and that undoing it again waits for those", async () => {
  const { server, rework } = reworked();
  const undo = server.log("undo", { kind: "revert", reverts: rework.id, at: "2026-10-05T08:10:00Z" });
  const again = server.log("again", { kind: "revert", reverts: rework.id, by: bob, at: "2026-10-05T08:20:00Z" });
  const orphan = server.log("orphan", { kind: "revert", reverts: changesetId(9999) });
  rework.revertedBy = [again.id, undo.id];
  const { router } = renderApp(changesetPath(rework.id), server.app);

  const heading = await screen.findByRole("heading", { level: 1, name: "Edit · Ada" });
  const about = heading.nextElementSibling as HTMLElement;
  expect(about.textContent).toContain("Undone by:");
  expect(
    within(about)
      .getAllByRole("link")
      .map((link) => [link.textContent, link.getAttribute("href")])
  ).toEqual([
    [`Undo change, ID ${again.id.slice(-6)}`, changesetPath(again.id)],
    [`Undo change, ID ${undo.id.slice(-6)}`, changesetPath(undo.id)],
  ]);
  expect(screen.getByText("Undone before: undoing it again succeeds only once that undo is undone.").className).toBe(
    noteClass
  );
  expect(screen.getByRole("button", { name: "Undo change" })).toBeTruthy();

  await act(() => router.navigate(changesetPath(undo.id)));
  const reverting = await screen.findByRole("heading", { level: 1, name: "Undo change · Ada" });
  expect(
    within(reverting.nextElementSibling as HTMLElement)
      .getByRole("link", { name: "Undid “Edit · Ada · Oct 5, 2026, 8:01 AM”" })
      .getAttribute("href")
  ).toBe(changesetPath(rework.id));
  // Never written again: when it was made alone.
  expect(reverting.nextElementSibling?.textContent).toContain("Made Oct 5, 2026, 8:10 AM");
  expect(reverting.nextElementSibling?.textContent).not.toContain("Last written");

  await act(() => router.navigate(changesetPath(orphan.id)));
  await screen.findByText("Undid a change the purge took");
  expect(screen.queryByRole("link", { name: /^Undid/ })).toBeNull();
});

test("a changeset the purge trimmed says it can no longer be undone, and has no Undo change", async () => {
  const { server, rework } = reworked();
  rework.trimmed = true;
  renderApp(changesetPath(rework.id), server.app);

  await screen.findByRole("heading", { level: 1, name: "Edit · Ada" });
  expect(
    screen.getByText(
      "Some or all of what this change touched has been purged from the trash: it can no longer be undone."
    )
  ).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Undo change" })).toBeNull();
  expect(screen.queryByText("Editors and admins can undo this change.")).toBeNull();
});

test("a reader sees the changeset, and who can undo it, without Undo change", async () => {
  const { server, rework } = reworked("reader");
  renderApp(changesetPath(rework.id), server.app);

  await screen.findByRole("heading", { level: 1, name: "Edit · Ada" });
  expect(screen.getByText("Editors and admins can undo this change.")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Undo change" })).toBeNull();
  await screen.findByRole("list", { name: "Nodes of this change" });
  expect(nodes()).toHaveLength(8);
});

test("More reads the next hundred nodes after those read, and leaves once the changeset is read whole, the focus on the first it added; read again, the list is as long as it was", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const many = Array.from({ length: 101 }, (_, n) => pageNode(200 + n, `Page ${(n + 1).toString()}`));
  const server = pageServer({ nodes: many });
  const big = server.log("big", { changes: many.map((node) => created(node)) });
  renderApp(changesetPath(big.id), server.app);
  await screen.findByRole("list", { name: "Nodes of this change" });
  expect(nodes()).toHaveLength(100);

  await user.click(screen.getByRole("button", { name: "More nodes of this change" }));

  await waitFor(() => expect(nodes()).toHaveLength(101));
  expect(server.sent.filter((each) => each.startsWith("GET changes "))).toEqual([
    "GET changes big",
    "GET changes big after Page 100",
  ]);
  expect(screen.queryByRole("button", { name: "More nodes of this change" })).toBeNull();
  expect(document.activeElement).toBe(within(nodes()[100] as HTMLElement).getByRole("link", { name: "Page 101" }));

  // Read again, as many pages as were read.
  server.sent.length = 0;
  await readAgain();
  await waitFor(() => expect(server.sent.filter((each) => each.startsWith("GET changes "))).toHaveLength(2));
  expect(nodes()).toHaveLength(101);
});

test("an address of no changeset's id is no page, and is not read", async () => {
  const { server } = reworked();
  renderApp(`${home}/history/rework`, server.app);

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(server.sent.filter((each) => each.startsWith("GET change"))).toEqual([]);
});

test("a changeset not found, or of another notebook, is no page", async () => {
  const asked: string[] = [];
  const { server, rework } = reworked("admin", {
    [`GET /api/v0/changesets/${changesetId(9998)}`]: () => {
      asked.push("GET changeset of another notebook");
      return json({
        id: changesetId(9998),
        notebook_id: "0199a2b4-0000-7000-8000-0000000000c2",
        kind: "edit",
        client: "web",
        message: null,
        created_by: { user_id: bob.user_id, display_name: "Bob" },
        created_at: "2026-10-05T08:00:00Z",
        updated_at: "2026-10-05T08:00:00Z",
        reverts: null,
        reverted_by: [],
        trimmed: false,
        ingest_count: 0,
        ingest_sources: [],
        node_count: 0,
        content_count: 0,
        changes: [],
        renames: false,
        moves: false,
      });
    },
  });
  const { router } = renderApp(changesetPath(changesetId(9997)), server.app);
  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();

  await act(() => router.navigate(changesetPath(changesetId(9998))));
  await waitFor(() => expect(asked).toEqual(["GET changeset of another notebook"]));
  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(screen.queryByRole("heading", { level: 1, name: "Edit · Bob" })).toBeNull();

  await act(() => router.navigate(changesetPath(rework.id)));
  expect(await screen.findByRole("heading", { level: 1, name: "Edit · Ada" })).toBeTruthy();
});

test("a changeset that could not be read says why, and is read again on Try again", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  server.historyDown = true;
  renderApp(changesetPath(rework.id), server.app);

  await user.click(await screen.findByRole("button", { name: "Try again" }));
  server.historyDown = false;
  await user.click(await screen.findByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Edit · Ada" })).toBeTruthy();
});

const pagesEvent = (tree: boolean) => ({
  workspace_id: workspaceJSON.id,
  notebook_id: notebookJSON.id,
  tree,
  pages: tree ? [] : [{ id: guide.id, revision: 2 }],
});

test("a changeset and its nodes are read again as the notebook's pages change, and on each connection", async () => {
  const events = eventServer();
  const { server, rework } = reworked("admin", { "GET /api/v0/events": events.answer });
  renderApp(changesetPath(rework.id), withEvents(server.app), { eventHandlers });
  await screen.findByRole("list", { name: "Nodes of this change" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const reads = () => server.sent.filter((each) => each === "GET changeset rework" || each === "GET changes rework");
  events.last().hello();
  await waitFor(() => expect(reads()).toHaveLength(4));

  // Undone elsewhere: a content written is a change too.
  const undo = server.log("undo", { kind: "revert", reverts: rework.id, by: bob });
  rework.revertedBy = [undo.id];
  events.last().send("pages", pagesEvent(false));

  await screen.findByText("Undone by:");
  await waitFor(() => expect(reads()).toHaveLength(6));
});

// The negative controls (M8/P3 design section 6).

test("a content's link leads to the version the change wrote, not the one it wrote on", async () => {
  const server = pageServer();
  const write = server.log("write", { changes: [wrote(guide, 3, 4)] });
  renderApp(changesetPath(write.id), server.app);
  await screen.findByRole("list", { name: "Nodes of this change" });

  expect(
    within(rowOf("Guide")).getByRole("link", { name: "Wrote its content: “Guide”, version 4" }).getAttribute("href")
  ).toBe(`${pagePath(guide.id)}/versions/4`);
});

test("a node moved and renamed at once says both", async () => {
  const server = pageServer();
  const both = server.log("both", {
    changes: [{ ...moved(linux, notes.id), before: { parent: notes.id, name: "Old linux" } }],
  });
  renderApp(changesetPath(both.id), server.app);
  await screen.findByRole("list", { name: "Nodes of this change" });

  expect(rowOf("Linux").textContent).toBe(
    "LinuxHistory of this nodeMoved from “Notes” to “Install”Renamed: “Old linux” → “Linux”"
  );
});

test("a node read away as the focus is on its link gives the focus to the title", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);
  await screen.findByRole("list", { name: "Nodes of this change" });
  act(() => within(rowOf("Old")).getByRole("link", { name: "In the trash: “Old”" }).focus());

  // Purged from the trash: the node and its change are gone.
  server.trash.delete(old.id);
  await readAgain();

  await waitFor(() => expect(nodes()).toHaveLength(7));
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Edit · Ada" }));
});

// A name of another direction does not turn the line around it (M12/P2/S3; M5-polish 32.4).

/** bdis are the texts of the bdis in element, in order. */
const bdis = (element: Element) => [...element.querySelectorAll("bdi")].map((bdi) => bdi.textContent);

const manual = pageNode(21, "מדריך");
const setup = pageNode(22, "התקנה");

test("a changeset's page writes its MCP client's name, what it undid and its nodes' names where they were renamed each in a bdi", async () => {
  const server = pageServer({ nodes: [guide, install, linux, notes, setup] });
  const david = { user_id: "0199a2b4-0000-7000-8000-000000000003", display_name: "דוד" };
  const rtl = server.log("rtl", {
    at: "2026-10-05T08:02:00Z",
    by: david,
    client: "mcp:עוזר",
    changes: [renamed({ ...setup, name: "מדריך" }, "התקנה")],
  });
  const undo = server.log("undo", {
    at: "2026-10-05T08:04:00Z",
    kind: "revert",
    client: "mcp:עוזר",
    reverts: rtl.id,
    changes: [renamed(setup, "מדריך")],
  });
  rtl.revertedBy = [undo.id];
  renderApp(changesetPath(undo.id), server.app);

  const heading = await screen.findByRole("heading", { level: 1, name: "Undo change · Ada" });
  await screen.findByRole("list", { name: "Nodes of this change" });
  const client = (heading.nextElementSibling as HTMLElement).querySelector("p") as HTMLElement;
  expect(bdis(client)).toEqual(["עוזר"]);
  expect(client.textContent).toBe("Client: MCP (עוזר)");
  const undid = screen.getByRole("link", { name: /^Undid/ });
  expect(bdis(undid)).toEqual(["דוד"]);
  expect(undid.textContent).toBe("Undid “Edit · דוד · Oct 5, 2026, 8:02 AM”");
  const said = [...rowOf("התקנה").querySelectorAll("span")].find((span) =>
    span.textContent?.startsWith("Renamed:")
  ) as HTMLElement;
  expect(bdis(said)).toEqual(["מדריך", "התקנה"]);
  expect(said.textContent).toBe("Renamed: “מדריך” → “התקנה”");
});

test("a node moved between two pages of another direction names where it was and where it is in that order, each in a bdi", async () => {
  const quota = pageNode(23, "Quota", setup);
  const server = pageServer({ nodes: [guide, install, linux, notes, manual, setup, quota] });
  const move = server.log("move", { changes: [moved(quota, manual.id)] });
  renderApp(changesetPath(move.id), server.app);

  await screen.findByRole("list", { name: "Nodes of this change" });
  const said = [...rowOf("Quota").querySelectorAll("span")].find((span) =>
    span.textContent?.startsWith("Moved")
  ) as HTMLElement;
  expect([...said.querySelectorAll("bdi")].map((bdi) => bdi.textContent)).toEqual(["מדריך", "התקנה"]);
  expect(said.textContent).toBe("Moved from “מדריך” to “התקנה”");
});

// M13/P5/S2 (decision 929): the changeset's page is headed as a page is: its trail in the head's row, Undo change on
// the row's right, before the title in the document; its title the page's 32px one, its facts right under it, one grey.
test("a changeset's page is headed as a page is: Undo change in the head's row after the trail, before the title, a small pill; the dialog closed, it has the focus back", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);
  const heading = await screen.findByRole("heading", { level: 1, name: "Edit · Ada" });
  const undo = screen.getByRole("button", { name: "Undo change" });
  const trail = screen.getByRole("navigation", { name: "Breadcrumb" });

  expect(trail.compareDocumentPosition(undo)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  expect(undo.compareDocumentPosition(heading)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  // The trail and Undo change share the head's row; the title, in the column, has its facts right under it.
  expect(trail.className).toBe(headCrumbsClass);
  expect(trail.parentElement?.className).toBe(headRowClass);
  expect(undo.parentElement?.className).toBe(headActionsClass);
  expect(trail.parentElement?.contains(undo)).toBe(true);
  expect(heading.className).toBe(docTitleClass);
  expect((heading.nextElementSibling as HTMLElement).className).toBe(cn(docFactsClass, "space-y-1"));
  // Its message is content, the body's colour, not the facts' grey (review B's P5B-R1-M3).
  expect(heading.nextElementSibling?.querySelector("bdi")?.parentElement?.className).toBe(
    "text-sm break-words text-foreground"
  );
  expect(heading.closest(".max-w-180")).not.toBeNull();
  expect([...undo.classList]).toEqual(expect.arrayContaining(["h-7.5", "rounded-full"]));

  await user.click(undo);
  const dialog = await screen.findByRole("alertdialog", { name: "Undo this change?" });
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(undo);
});

test("a reader's note and a writer's Undone before are notes under the facts", async () => {
  const { server, rework } = reworked("reader");
  renderApp(changesetPath(rework.id), server.app);
  const heading = await screen.findByRole("heading", { level: 1, name: "Edit · Ada" });
  const note = screen.getByText("Editors and admins can undo this change.");

  expect(heading.nextElementSibling?.compareDocumentPosition(note)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  expect(note.className).toBe(noteClass);
  expect(screen.queryByRole("button", { name: "Undo change" })).toBeNull();
});

// M13/P5/S2 (decisions 946, 951): on its way, not found, its address no changeset's, the changeset's column is where
// the read page's is, below an empty row laid out as its head's row: the trail alone, and a writer's group.
test.each([
  ["a writer", "admin"],
  ["a reader", "reader"],
] as const)(
  "a changeset on its way lays its head's row out empty for %s, as the changeset read lays it",
  async (_, role) => {
    const { server, rework } = reworked(role);
    const held = gateOf();
    server.historyWaits.changeset = held.promise;
    renderApp(changesetPath(rework.id), server.app);
    const loading = await waitFor(() => {
      const found = within(screen.getByRole("main")).getByText("Loading…", { selector: "output" });
      expect(found.closest(".max-w-180")).not.toBeNull();
      return found;
    });
    const empty = (loading.closest(".max-w-180")?.previousElementSibling ?? null) as HTMLElement | null;
    const shape = {
      hidden: empty?.getAttribute("aria-hidden"),
      row: empty?.className,
      text: empty?.textContent,
      blocks: [...(empty?.children ?? [])].map((block) => block.className),
    };

    act(() => held.open());
    const trail = await screen.findByRole("navigation", { name: "Breadcrumb" });
    const row = trail.parentElement as HTMLElement;
    expect(shape).toEqual({
      hidden: "true",
      row: row.className,
      text: "",
      blocks: [
        cn(trail.className, "h-[1lh] text-compact"),
        ...[...row.children].slice(1).map((block) => block.className),
      ],
    });
    expect(row.children).toHaveLength(role === "admin" ? 2 : 1);
  }
);

/** gateOf is a wait a test opens: what waits for its promise goes on once open is called. */
function gateOf() {
  let open!: () => void;
  const promise = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { promise, open };
}

// M13/P5/S2 (decision 940): the links of the changeset's page to elsewhere arrive there, where the title takes the
// focus: not the body.
type Made = ReturnType<typeof reworked>;
/** A link of the page's: the changeset its page shows, what is done there before it shows, the link. */
type Leading = [
  string,
  (made: Made) => string,
  ((user: ReturnType<typeof userEvent.setup>) => Promise<void>) | undefined,
  () => HTMLElement,
];

/** undone has made's rework undone by Ada: the undo's id. */
function undone({ server, rework }: Made): string {
  const undo = server.log("undo", { kind: "revert", reverts: rework.id, at: "2026-10-05T08:10:00Z" });
  rework.revertedBy = [undo.id];
  return undo.id;
}

test.each<Leading>([
  [
    "See the undo",
    ({ rework }) => rework.id,
    async (user) => {
      await user.click(await screen.findByRole("button", { name: "Undo change" }));
      const dialog = await screen.findByRole("alertdialog", { name: "Undo this change?" });
      await user.click(within(dialog).getByRole("button", { name: "Undo" }));
      await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Undone. See the undo"));
    },
    () => within(screen.getByRole("status")).getByRole("link", { name: "See the undo" }),
  ],
  ["Undid “…”", (made) => undone(made), undefined, () => screen.getByRole("link", { name: /^Undid / })],
  [
    "Undo change, ID …, after Undone by:",
    (made) => {
      undone(made);
      return made.rework.id;
    },
    undefined,
    () => screen.getByRole("link", { name: /^Undo change, ID / }),
  ],
  [
    "In the trash",
    ({ rework }) => rework.id,
    undefined,
    () => within(rowOf("Old")).getByRole("link", { name: "In the trash: “Old”" }),
  ],
  [
    "History of this node",
    ({ rework }) => rework.id,
    undefined,
    () => within(rowOf("Install")).getByRole("link", { name: "History of this node: “Install”" }),
  ],
  [
    "a content's version",
    ({ rework }) => rework.id,
    undefined,
    () => within(rowOf("Install")).getByRole("link", { name: "Wrote its content: “Install”, version 1" }),
  ],
])("%s leads elsewhere arrived at, the title there taking the focus", async (_, shown, before, linkOf) => {
  const user = userEvent.setup();
  const made = reworked();
  const { router } = renderApp(changesetPath(shown(made)), made.server.app);
  await screen.findByRole("list", { name: "Nodes of this change" });
  await before?.(user);

  await user.click(linkOf());

  await waitFor(() => expect(router.state.location.state).toEqual(arrived));
  await waitFor(() =>
    expect(document.activeElement).toBe(within(screen.getByRole("main")).getAllByRole("heading", { level: 1 })[0])
  );
});
