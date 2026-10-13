import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { unstable_serialize, type Cache } from "swr";
import { afterEach, expect, test, vi } from "vitest";

import { eventHandlers, type EventHandler } from "../../events/handlers";
import { appLinks } from "../../reading/app-links";
import { taskToggle } from "../../reading/task-toggle";
import { unresolvedLinks } from "../../reading/unresolved-links";
import type { NotebookRole } from "../../services/notebook.service";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, problem, workspaceJSON, type Answer } from "../../test/fakes";
import { assetNode, guide, install, linux, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";
import type { FakeTrashItem } from "../../test/trash-server";

// A trash item's preview (M8 design 4.5; M8/P2 design 4.3, 4.6).

afterEach(() => {
  vi.useRealTimers();
});

const home = `/lab/notebooks/${notebookJSON.id}`;
const trashPath = `${home}/trash`;
const previewPath = (id: string) => `${trashPath}/${id}`;

/** The enhancements of a view these tests look at: the task items', the links'. */
const enhancements = [taskToggle, appLinks, unresolvedLinks];

/**
 * trashed is pageServer with Install in its trash, Linux with it, Install's
 * content task items at revision 3, a link to Notes and one to a page that
 * is not there; and the attachment a.png of Guide. Ada's role in Plans is
 * role, answers answering instead of it.
 */
function trashed(role: NotebookRole = "admin", answers: Record<string, Answer> = {}) {
  const picture = assetNode(20, "a.png", guide);
  const server = pageServer({ role, answers, nodes: [guide, install, linux, notes, picture] });
  server.withTasks(install.id, "- [ ] one\n- [x] two\n", 3);
  server.versionViews.set(`${install.id} 3`, {
    html:
      '<ul><li><input disabled="" type="checkbox" data-task="3"> one</li></ul>' +
      `<p><a data-nw-node="${notes.id}">Notes</a> ` +
      '<a class="nw-unresolved" data-nw-target="Nowhere">Nowhere</a></p>',
    revision: 3,
    assets_expire_at: null,
  });
  const item = server.trashNode(install.id);
  server.trashNode(picture.id);
  return { server, item, picture };
}

test("a page's preview shows where it was and what the trash says of it, and its view as it was deleted, read only, by its revision; the page and its tab are named for it", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  renderApp(previewPath(install.id), server.app, { enhancements });

  expect(await screen.findByRole("heading", { level: 1, name: "Install" })).toBeTruthy();
  await waitFor(() => expect(document.title).toBe("Install · Trash · Plans · Nerve Wiki"));
  const crumbs = screen.getByRole("navigation", { name: "Breadcrumb" });
  expect(
    within(crumbs)
      .getAllByRole("link")
      .map((link) => [link.textContent, link.getAttribute("href")])
  ).toEqual([
    ["Plans", home],
    ["Trash", trashPath],
  ]);
  expect(within(crumbs).getByText("Install").getAttribute("aria-current")).toBe("page");
  const main = screen.getByRole("main");
  expect(main.textContent).toContain("From: Guide");
  expect(main.textContent).toContain("Pages: 2, attachments: 0");
  expect(main.textContent).toContain("Restores under “Guide”");
  expect(main.textContent).toContain(
    "This page is in the trash. Its links lead to the pages there are today: a link to a page deleted with it leads there once both are restored."
  );
  const article = await screen.findByRole("article", { name: "Install" });
  expect(server.sent).toContain("GET version view Install 3");
  await user.click(within(article).getByRole("checkbox"));
  expect(server.sent.filter((each) => each.startsWith("TOGGLE"))).toEqual([]);

  await user.click(within(article).getByRole("button", { name: "Nowhere" }));
  const dialog = await screen.findByRole("alertdialog", { name: "“Nowhere” does not exist" });
  expect(
    within(dialog).getByText("No page has this name. This view is read only: no page is created from its links.")
  ).toBeTruthy();
  expect(server.sent.filter((each) => each.startsWith("GET landing"))).toEqual([]);
  await user.click(within(dialog).getByRole("button", { name: "OK" }));

  // Its links lead to the pages alive.
  await user.click(within(article).getByRole("link", { name: "Notes" }));
  expect(await screen.findByRole("heading", { level: 1, name: "Notes" })).toBeTruthy();
});

test("an attachment's preview shows its type and size, to download once restored: no address to it", async () => {
  const { server, picture } = trashed();
  renderApp(previewPath(picture.id), server.app);

  expect(await screen.findByRole("heading", { level: 1, name: "a.png" })).toBeTruthy();
  const main = screen.getByRole("main");
  expect(main.textContent).toContain("Image · 1 KB");
  expect(main.textContent).toContain("It can be downloaded once restored.");
  expect(main.querySelector("a[href*='/content']")).toBeNull();
  expect(server.sent.filter((each) => each.startsWith("GET asset"))).toEqual([]);
});

test.each([
  ["a page whose content the purge took", "Install"],
  ["an attachment whose file the purge took", "a.png"],
])("%s cannot be previewed, and its view is not read", async (_case, name) => {
  const { server, item, picture } = trashed();
  item.purged = true;
  (server.trash.get(picture.id) as FakeTrashItem).purged = true;
  renderApp(previewPath(name === "Install" ? install.id : picture.id), server.app);

  expect(await screen.findByText("This cannot be previewed.")).toBeTruthy();
  expect(server.sent.filter((each) => each.startsWith("GET version view"))).toEqual([]);
});

test("an item of another notebook is no page of this one's", async () => {
  const { server } = trashed("admin", {
    "GET /api/v0/nodes/*/trash-item": () =>
      json({ ...trashItemJSON, notebook_id: "0199a2b4-0000-7000-8000-0000000000c2" }),
  });
  renderApp(previewPath(install.id), server.app);

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(server.sent.filter((each) => each.startsWith("GET version view"))).toEqual([]);
});

/** trashItemJSON is an item as the server answers it, of Plans. */
const trashItemJSON = {
  id: install.id,
  notebook_id: notebookJSON.id,
  kind: "page",
  name: "Install",
  ancestors: [],
  deleted_at: "2026-10-04T08:01:00Z",
  deleted_by: { user_id: notebookJSON.id, display_name: "Ada" },
  restorable_until: "2026-12-03T07:01:00Z",
  pages: 1,
  assets: 0,
  restore_to: { parent_id: null, reason: "original" },
  revision: 1,
  asset: null,
};

test("a node deleted with a page above it goes to its item's root's preview, the address replaced", async () => {
  const { server } = trashed();
  const { router } = renderApp(previewPath(linux.id), server.app);

  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  expect(router.state.location.pathname).toBe(previewPath(install.id));
  expect(router.state.historyAction).toBe("REPLACE");
  // It says why, its title taking the focus (M8/P2B review A-M1).
  expect(screen.getByRole("alert").textContent).toBe(
    "What you opened was deleted again with a page above it: it is part of this item now, and is restored with it."
  );
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

// M8/P2B review B-3 (v0.1 design 13.2, item 7).
test("an item that could not be read says why, and is read again on Try again", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  server.trashDown = true;
  renderApp(previewPath(install.id), server.app);

  const retry = await screen.findByRole("button", { name: "Try again" });
  expect(screen.getByRole("alert").textContent).not.toBe("");
  server.trashDown = false;
  await user.click(retry);

  expect(await screen.findByRole("heading", { level: 1, name: "Install" })).toBeTruthy();
  expect(screen.queryByRole("alert")).toBeNull();
});

// M8/P2B review A-N1, A-N3.
test("an address of no node's id is no page, and reads nothing", async () => {
  const { server } = trashed();
  renderApp(`${trashPath}/install`, server.app);

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(server.sent.filter((each) => each.startsWith("GET trash item"))).toEqual([]);
});

test("a node of another notebook no longer in the trash is no page of this one's, once the tree read has it not", async () => {
  const other = "0199a2b4-0000-7000-8000-000000000777";
  const { server } = trashed("admin", {
    [`GET /api/v0/nodes/${other}/trash-item`]: () => problem(409, "page.not_in_trash"),
  });
  renderApp(previewPath(other), server.app);

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
});

test("a node no longer in the trash says so, with a way to it the tree has; a page to itself", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  server.restoreElsewhere(install.id);
  renderApp(previewPath(install.id), server.app);

  expect(await screen.findByText("This is no longer in the trash.")).toBeTruthy();
  expect(screen.getByRole("link", { name: "Back to the trash" }).getAttribute("href")).toBe(trashPath);
  await waitFor(() => expect(document.title).toBe("Trash · Plans · Nerve Wiki"));
  expect(screen.queryByRole("alert")).toBeNull();
  // Opened so, it takes no focus: nothing it showed left with it.
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(document.activeElement).toBe(document.body);
  await user.click(await screen.findByRole("link", { name: "Open it" }));
  expect(await screen.findByRole("heading", { level: 1, name: "Install" })).toBe(document.activeElement);
});

test("an attachment no longer in the trash leads to where it is, its row taking the focus", async () => {
  const user = userEvent.setup();
  const { server, picture } = trashed();
  server.restoreElsewhere(picture.id);
  renderApp(previewPath(picture.id), server.app);

  await user.click(await screen.findByRole("link", { name: "Open it" }));
  expect(await screen.findByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
  await waitFor(() => expect(document.activeElement?.getAttribute("data-attachment")).toBe(picture.id));
});

test("an item past its time says so, and leads back to the trash; a node of no item is no page", async () => {
  const { server, item } = trashed();
  item.expired = true;
  const view = renderApp(previewPath(install.id), server.app);

  expect(await screen.findByText("This can no longer be restored: its time in the trash is over.")).toBeTruthy();
  expect(screen.getByRole("link", { name: "Back to the trash" }).getAttribute("href")).toBe(trashPath);
  await waitFor(() => expect(document.title).toBe("Trash · Plans · Nerve Wiki"));
  view.unmount();

  renderApp(previewPath("0199a2b4-0000-7000-8000-000000000999"), server.app);
  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
});

test("a reader previews an item, without Restore", async () => {
  const { server } = trashed("reader");
  renderApp(previewPath(install.id), server.app);

  await screen.findByRole("heading", { level: 1, name: "Install" });
  expect(screen.queryByRole("button", { name: /^Restore/ })).toBeNull();
});

test("a restore from the preview goes back to the trash, which says where the item went, its title taking the focus, described by it", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  const { router } = renderApp(previewPath(install.id), server.app);
  const restore = await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" });
  const before = server.sent.length;

  await user.click(restore);

  const heading = await screen.findByRole("heading", { level: 1, name: "Trash" });
  // The item read again after the answer, as after every answer (M8/P2 design 4.2).
  expect(server.sent.slice(before, before + 3)).toEqual(["RESTORE Install", "GET nodes", "GET trash item Install"]);
  expect(router.state.location.pathname).toBe(trashPath);
  await waitFor(() => expect(document.activeElement).toBe(heading));
  const status = screen.getByRole("status");
  expect(status.textContent).toBe("Restored “Install” under “Guide”. Open");
  expect(heading.getAttribute("aria-describedby")).toBe(status.id);
  expect(within(status).getByRole("link", { name: "Open" }).getAttribute("href")).toBe(pagePath(install.id));
  await waitFor(() =>
    expect(within(screen.getByRole("list", { name: "Trash" })).getAllByRole("listitem")).toHaveLength(1)
  );
});

// M8/P2B review A-I1, C-I1: the restore's own event comes with its answer, before the tree read after it.
test("a restore from the preview whose own event reads the item again, no longer in the trash, before the restore's tree read answers keeps the item shown, then goes back to the trash, which says where it went", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const tree: { waits?: Promise<void> } = {};
  const holder: { server?: ReturnType<typeof pageServer> } = {};
  const { server } = trashed("admin", {
    "GET /api/v0/events": events.answer,
    [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: async () => {
      holder.server?.sent.push("GET nodes");
      await tree.waits;
      return json({ data: holder.server?.nodes ?? [] });
    },
  });
  holder.server = server;
  const { router } = renderApp(previewPath(install.id), withEvents(server.app), { eventHandlers });
  await screen.findByRole("article", { name: "Install" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
  let open!: () => void;
  tree.waits = new Promise<void>((resolve) => {
    open = resolve;
  });
  const itemReads = () => server.sent.filter((each) => each === "GET trash item Install").length;

  await user.click(screen.getByRole("button", { name: "Restore “Install” (From: Guide)" }));
  await waitFor(() => expect(server.sent.slice(-2)).toEqual(["RESTORE Install", "GET nodes"]));
  const before = itemReads();
  events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });
  await waitFor(() => expect(itemReads()).toBe(before + 1));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(screen.getByRole("article", { name: "Install" })).toBeTruthy();
  expect(screen.queryByText("This is no longer in the trash.")).toBeNull();
  tree.waits = undefined;
  act(() => open());

  const heading = await screen.findByRole("heading", { level: 1, name: "Trash" });
  expect(router.state.location.pathname).toBe(trashPath);
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(screen.getByRole("status").textContent).toBe("Restored “Install” under “Guide”. Open");
});

test("a restore from the preview refused says why above its view, which stays; the item is read again", async () => {
  const user = userEvent.setup();
  const { server, item } = trashed();
  renderApp(previewPath(install.id), server.app);
  const restore = await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" });
  item.history = 400_001;
  const before = server.sent.length;

  await user.click(restore);

  expect((await screen.findByRole("alert")).textContent).toBe(
    "“Install” is too large to restore here: it holds more pages, attachments, content, links or versions than one restore brings back. Pages: 2, attachments: 0."
  );
  await waitFor(() =>
    expect(server.sent.slice(before)).toEqual(["RESTORE Install", "GET nodes", "GET trash item Install"])
  );
  expect(document.activeElement).toBe(restore);
  expect(screen.getByRole("article", { name: "Install" })).toBeTruthy();
  // The refusal describes the title.
  expect(screen.getByRole("heading", { level: 1, name: "Install" }).getAttribute("aria-describedby")).toBe(
    screen.getByRole("alert").id
  );
});

test("a restore from the preview refused, the role taken away meanwhile, says why: a reader by now, the Restore goes, the focus to the title", async () => {
  const user = userEvent.setup();
  let role: NotebookRole = "admin";
  const { server } = trashed("admin", {
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [{ ...notebookJSON, role }] }),
    "POST /api/v0/nodes/*/restore": () => problem(403, "forbidden"),
  });
  renderApp(previewPath(install.id), server.app);
  const restore = await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" });
  role = "reader";

  await user.click(restore);

  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(screen.queryByRole("button", { name: /^Restore/ })).toBeNull());
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Install" }));
});

// M8/P2B review A-M1: what the restore was refused for is said where the item was, its title taking the focus.
test.each([
  {
    meanwhile: "restored elsewhere",
    change: (server: ReturnType<typeof trashed>) => void server.server.restoreElsewhere(install.id),
    refusal: "“Install” changed meanwhile: the trash shows it as it is now.",
    text: "This is no longer in the trash.",
  },
  {
    meanwhile: "past its time",
    change: (server: ReturnType<typeof trashed>) => {
      server.item.expired = true;
    },
    refusal: "“Install” can no longer be restored: its time in the trash is over.",
    text: "This can no longer be restored: its time in the trash is over.",
  },
])(
  "a restore from the preview of an item $meanwhile says why, and what became of it, the trash's title taking the focus, described by the refusal",
  async ({ change, refusal, text }) => {
    const user = userEvent.setup();
    const trash = trashed();
    renderApp(previewPath(install.id), trash.server.app);
    const restore = await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" });
    change(trash);

    await user.click(restore);

    expect(await screen.findByText(text)).toBeTruthy();
    const alert = screen.getByRole("alert");
    expect(alert.textContent).toBe(refusal);
    const heading = screen.getByRole("heading", { level: 1, name: "Trash" });
    await waitFor(() => expect(document.activeElement).toBe(heading));
    expect(heading.getAttribute("aria-describedby")).toBe(alert.id);
    await waitFor(() => expect(document.title).toBe("Trash · Plans · Nerve Wiki"));
  }
);

test("a restore from the preview of an item deleted again with a page above it goes to that item's preview, which says why, its title taking the focus", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  const { router } = renderApp(previewPath(install.id), server.app);
  const restore = await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" });
  server.restoreElsewhere(install.id);
  server.trashNode(guide.id);

  await user.click(restore);

  const heading = await screen.findByRole("heading", { level: 1, name: "Guide" });
  expect(router.state.location.pathname).toBe(previewPath(guide.id));
  const alert = screen.getByRole("alert");
  expect(alert.textContent).toBe(
    "What you opened was deleted again with a page above it: it is part of this item now, and is restored with it."
  );
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(heading.getAttribute("aria-describedby")).toBe(alert.id);
});

test("a refusal from the preview is not said once the item is gone for another reason: restored elsewhere since", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const { server, item } = trashed("admin", { "GET /api/v0/events": events.answer });
  renderApp(previewPath(install.id), withEvents(server.app), { eventHandlers });
  const restore = await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  item.history = 400_001;
  await user.click(restore);
  expect(await screen.findByRole("alert")).toBeTruthy();

  server.restoreElsewhere(install.id);
  events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });

  expect(await screen.findByText("This is no longer in the trash.")).toBeTruthy();
  expect(screen.queryByRole("alert")).toBeNull();
});

test("a node no longer in the trash, the tree not read, says so, without a way to it", async () => {
  const { server } = trashed();
  server.restoreElsewhere(install.id);
  server.nodesDown = true;
  renderApp(previewPath(install.id), server.app);

  expect(await screen.findByText("This is no longer in the trash.")).toBeTruthy();
  await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
  expect(screen.queryByRole("heading", { name: "Page not found" })).toBeNull();
  expect(screen.queryByRole("link", { name: "Open it" })).toBeNull();
});

// M8/P2B fix check 1, N1 (K1): made a member by others, the redirect leaves the focus where the reader has it.
test("a node made a member of another item by others while its preview shows goes to that item's preview, which says why, the focus left where the reader has it", async () => {
  const events = eventServer();
  const { server } = trashed("admin", { "GET /api/v0/events": events.answer });
  const { router } = renderApp(previewPath(install.id), withEvents(server.app), { eventHandlers });
  await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  const elsewhere = within(screen.getByRole("navigation", { name: "Pages of Plans" })).getByRole("link", {
    name: "Notes",
  });
  act(() => elsewhere.focus());

  // Others restore Install, then delete Guide with it.
  server.restoreElsewhere(install.id);
  server.trashNode(guide.id);
  events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });

  await screen.findByRole("heading", { level: 1, name: "Guide" });
  expect(router.state.location.pathname).toBe(previewPath(guide.id));
  expect(screen.getByRole("alert").textContent).toBe(
    "What you opened was deleted again with a page above it: it is part of this item now, and is restored with it."
  );
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(document.activeElement).toBe(elsewhere);
});

// M8/P2B fix check 1, N2 (K5b): the item read again as a network would, after the refusal is said.
test("a refusal from the preview that takes the item away is one alert, said once as what shows below it changes", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  renderApp(previewPath(install.id), server.app);
  const restore = await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" });
  server.restoreElsewhere(install.id);
  let open!: () => void;
  server.trashWaits.item = new Promise<void>((resolve) => {
    open = resolve;
  });
  const inserted: string[] = [];
  const watching = new MutationObserver((records) => {
    for (const node of records.flatMap((record) => [...record.addedNodes])) {
      if (node instanceof HTMLElement) {
        const alerts = node.matches("[role=alert]") ? [node] : [...node.querySelectorAll("[role=alert]")];
        inserted.push(...alerts.map((alert) => alert.textContent ?? ""));
      }
    }
  });
  watching.observe(document.body, { childList: true, subtree: true });

  await user.click(restore);
  await waitFor(() => expect(inserted).toHaveLength(1));
  server.trashWaits.item = undefined;
  act(() => open());

  expect(await screen.findByText("This is no longer in the trash.")).toBeTruthy();
  watching.disconnect();
  expect(inserted).toEqual(["“Install” changed meanwhile: the trash shows it as it is now."]);
  const heading = screen.getByRole("heading", { level: 1, name: "Trash" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(heading.getAttribute("aria-describedby")).toBe(screen.getByRole("alert").id);
});

// M8/P2B fix check 1: what replaces the item takes the focus from nothing alone.
test("an item gone again, its preview shown again since, the reader elsewhere, leaves the focus where the reader has it", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const events = eventServer();
  const { server } = trashed("admin", { "GET /api/v0/events": events.answer });
  renderApp(previewPath(install.id), withEvents(server.app), { eventHandlers });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  const tree = { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] };
  // Restored elsewhere as the reader is on its Restore: what replaces it takes the focus.
  act(() => (screen.getByRole("button", { name: "Restore “Install” (From: Guide)" }) as HTMLElement).focus());
  server.restoreElsewhere(install.id);
  events.last().send("pages", tree);
  const gone = await screen.findByRole("heading", { level: 1, name: "Trash" });
  await waitFor(() => expect(document.activeElement).toBe(gone));

  // Deleted again, then restored again, the reader in the tree meanwhile.
  // Past the refresher's interval each time: the event reads the item at once.
  server.trashNode(install.id);
  await act(() => vi.advanceTimersByTimeAsync(5_100));
  events.last().send("pages", tree);
  await screen.findByRole("heading", { level: 1, name: "Install" });
  const elsewhere = within(screen.getByRole("navigation", { name: "Pages of Plans" })).getByRole("link", {
    name: "Notes",
  });
  act(() => elsewhere.focus());
  server.restoreElsewhere(install.id);
  await act(() => vi.advanceTimersByTimeAsync(5_100));
  events.last().send("pages", tree);

  expect(await screen.findByText("This is no longer in the trash.")).toBeTruthy();
  await act(() => vi.advanceTimersByTimeAsync(50));
  expect(document.activeElement).toBe(elsewhere);
});

// M8/P2B review B-9.
test("a restore from the preview pressed again says nothing of the refusal before while it is out", async () => {
  const user = userEvent.setup();
  const { server, item } = trashed();
  renderApp(previewPath(install.id), server.app);
  const restore = await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" });
  item.history = 400_001;
  await user.click(restore);
  expect(await screen.findByRole("alert")).toBeTruthy();
  item.history = 2;
  server.restoresHeld = true;

  await user.click(restore);

  await waitFor(() => expect(server.sent.filter((each) => each === "RESTORE Install")).toHaveLength(2));
  expect(screen.queryByRole("alert")).toBeNull();
  act(() => server.release());
  expect(await screen.findByRole("heading", { level: 1, name: "Trash" })).toBeTruthy();
});

test("the preview left, its view leaves SWR's cache: read again as it shows again, not kept for each item visited", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const { server } = trashed("admin", { "GET /api/v0/events": events.answer });
  // An event of its own hands the test SWR's cache, as the handlers have it.
  const probed: { cache?: Cache } = {};
  const handlers = new Map<string, EventHandler>([
    ...eventHandlers,
    [
      "probe",
      (_data, { cache }) => {
        probed.cache = cache;
      },
    ],
  ]);
  renderApp(previewPath(install.id), withEvents(server.app), { eventHandlers: handlers });
  await screen.findByRole("article", { name: "Install" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  const key = unstable_serialize(["page-version-view", notebookJSON.id, install.id, 3]);
  const cached = async () => {
    events.last().send("probe", {});
    await new Promise((resolve) => setTimeout(resolve, 50));
    return probed.cache?.get(key)?.data !== undefined;
  };
  expect(await cached()).toBe(true);

  await user.click(within(screen.getByRole("navigation", { name: "Breadcrumb" })).getByRole("link", { name: "Trash" }));
  await screen.findByRole("heading", { level: 1, name: "Trash" });
  await waitFor(async () => expect(await cached()).toBe(false));
  const before = server.sent.length;

  await user.click(screen.getByRole("link", { name: "Install (From: Guide)" }));
  await screen.findByRole("article", { name: "Install" });
  expect(server.sent.slice(before)).toContain("GET version view Install 3");
});

test("the item and its view are read again as the tree changes, and on each connection", async () => {
  const events = eventServer();
  const { server } = trashed("admin", { "GET /api/v0/events": events.answer });
  renderApp(previewPath(install.id), withEvents(server.app), { eventHandlers });
  await screen.findByRole("article", { name: "Install" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const reads = () =>
    server.sent.filter((each) => each === "GET trash item Install" || each === "GET version view Install 3");
  events.last().hello();
  await waitFor(() => expect(reads()).toHaveLength(4));

  events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });

  await waitFor(() => expect(reads()).toHaveLength(6));
});
