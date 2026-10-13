import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { unstable_serialize, type Cache } from "swr";
import { expect, test } from "vitest";

import { eventHandlers, type EventHandler } from "../../events/handlers";
import { appLinks } from "../../reading/app-links";
import { taskToggle } from "../../reading/task-toggle";
import { unresolvedLinks } from "../../reading/unresolved-links";
import type { NotebookRole } from "../../services/notebook.service";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, workspaceJSON, type Answer } from "../../test/fakes";
import {
  assetNode,
  guide,
  install,
  linux,
  notes,
  pagePath,
  pageServer,
  type FakeTrashItem,
} from "../../test/page-server";
import { renderApp } from "../../test/render";

// A trash item's preview (M8 design 4.5; M8/P2 design 4.3, 4.6).

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

  expect(await screen.findByRole("heading", { level: 1, name: "Install" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(previewPath(install.id));
  expect(router.state.historyAction).toBe("REPLACE");
});

test("a node no longer in the trash says so, with a way to it the tree has; a page to itself", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  server.restoreElsewhere(install.id);
  renderApp(previewPath(install.id), server.app);

  expect(await screen.findByText("This is no longer in the trash.")).toBeTruthy();
  expect(screen.getByRole("link", { name: "Back to the trash" }).getAttribute("href")).toBe(trashPath);
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

  await user.click(await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Trash" });
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

test("a restore from the preview refused says why above its view, which stays; the item is read again", async () => {
  const user = userEvent.setup();
  const { server, item } = trashed();
  renderApp(previewPath(install.id), server.app);
  const restore = await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" });
  item.history = 400_001;
  const before = server.sent.length;

  await user.click(restore);

  expect((await screen.findByRole("alert")).textContent).toBe(
    "“Install” is too large to restore here. Pages: 2, attachments: 0."
  );
  await waitFor(() =>
    expect(server.sent.slice(before)).toEqual(["RESTORE Install", "GET nodes", "GET trash item Install"])
  );
  expect(document.activeElement).toBe(restore);
  expect(screen.getByRole("article", { name: "Install" })).toBeTruthy();
});

test("a restore from the preview of an item restored meanwhile says it is no longer in the trash", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  renderApp(previewPath(install.id), server.app);
  const restore = await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" });
  server.restoreElsewhere(install.id);

  await user.click(restore);

  expect(await screen.findByText("This is no longer in the trash.")).toBeTruthy();
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
