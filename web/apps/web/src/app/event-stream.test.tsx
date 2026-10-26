import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditorView } from "@codemirror/view";
import { unstable_serialize, type Cache } from "swr";
import { afterEach, expect, test, vi } from "vitest";

import { eventHandlers, type EventHandler } from "../events/handlers";
import { FakePage } from "../events/testing/fake-page";
import { eventServer, withEvents } from "../test/event-server";
import { json, notebookJSON, problem, workspaceJSON, type Answer } from "../test/fakes";
import type { PageVersionContent } from "../services/page.service";
import { bob, guide, install, linux, notes, pagePath, pageServer, versionJSON } from "../test/page-server";
import { marked } from "../test/page-editor";
import { renderApp } from "../test/render";

// Each event has what it changed read again (M5/P3 design 3.8): the tree,
// a page's reading view whose revision is newer, or whose links lead
// elsewhere (M6/P3 design 6.7), the pages of a tag (M6/P6 design 11), a
// page's edit lock; each connection, all of them.

const pagesEvent = (tree: boolean, pages: { id: string; revision: number }[] | null) => ({
  workspace_id: workspaceJSON.id,
  notebook_id: notebookJSON.id,
  tree,
  pages,
});

const lockEvent = (pageId: string) => ({
  workspace_id: workspaceJSON.id,
  notebook_id: notebookJSON.id,
  page_id: pageId,
  session_id: "0199a2b4-0000-7000-8000-0000000000e1",
});

/** The id in a request's path, /api/v0/pages/{id}… */
const idOf = (request: Request) => new URL(request.url).pathname.split("/")[4] ?? "";

/**
 * open renders Guide over pageServer and its event stream, connected, and
 * waits for what the connection read again: from then on, what is read is
 * the events' doing. A page's next view read can be held (holdView, which
 * returns its release); the locks read are in lockReads, by page id.
 */
async function open(page = new FakePage(), answers: Record<string, Answer> = {}, handlers = eventHandlers) {
  const events = eventServer();
  const workspaces: string[] = [];
  const lockReads: string[] = [];
  const holds = new Map<string, Promise<void>>();
  const server = pageServer({
    answers: {
      ...answers,
      "GET /api/v0/events": events.answer,
      "GET /api/v0/workspaces": () => {
        workspaces.push("GET workspaces");
        return json({ data: [workspaceJSON] });
      },
      // pageServer's views, whose next read of a page can be held: it answers the view as it was asked.
      "GET /api/v0/pages/*/view": async (request) => {
        const node = server.nodes.find((each) => each.id === idOf(request));
        server.sent.push(`GET view ${node?.name}`);
        const view =
          node && (server.views.get(node.id) ?? { html: `<p>${node.name}</p>`, revision: 1, assets_expire_at: null });
        const hold = holds.get(idOf(request));
        holds.delete(idOf(request));
        await hold;
        return view === undefined ? problem(404, "page.not_found") : json(view);
      },
      "GET /api/v0/pages/*/edit-lock": (request) => {
        lockReads.push(idOf(request));
        return server.nodes.some((node) => node.id === idOf(request))
          ? json(server.lockOf(idOf(request)))
          : problem(404, "page.not_found");
      },
    },
  });
  const holdView = (id: string) => {
    let release!: () => void;
    holds.set(id, new Promise<void>((resolve) => (release = resolve)));
    return () => release();
  };
  const view = renderApp(pagePath(guide.id), withEvents(server.app, page), { eventHandlers: handlers });
  expect((await screen.findByRole("article", { name: "Guide" })).innerHTML).toBe("<p>Guide</p>");
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const before = server.sent.length;
  events.last().hello();
  await waitFor(() =>
    expect(server.sent.slice(before)).toEqual(
      expect.arrayContaining([
        "GET nodes",
        "GET view Guide",
        `GET backlinks ${guide.id}`,
        `GET properties ${guide.id}`,
        "GET versions Guide",
        "GET assets Guide",
      ])
    )
  );
  server.sent.length = 0;
  workspaces.length = 0;
  lockReads.length = 0;
  return { ...view, server, events, workspaces, lockReads, holdView };
}

afterEach(() => vi.useRealTimers());

/** viewReads is what of sent read a reading view: an event reads the right column's too. */
function viewReads(sent: string[]): string[] {
  return sent.filter((each) => each.startsWith("GET view"));
}

/** Presses Ctrl and key on the page. */
function ctrl(key: string) {
  document.dispatchEvent(new KeyboardEvent("keydown", { key, ctrlKey: true, bubbles: true, cancelable: true }));
}

/** Lets what the events asked for go out. */
async function settle(): Promise<void> {
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
}

test("an event whose tree changed reads the tree again, then the attachments shown, not the reading views", async () => {
  const { server, events } = await open();
  server.nodes = [{ ...guide, name: "Handbook" }, install, linux, notes];

  events.last().send("pages", pagesEvent(true, []));

  expect(await screen.findByRole("heading", { level: 1, name: "Handbook" })).toBeTruthy();
  await waitFor(() => expect(server.sent).toEqual(["GET nodes", "GET assets Handbook"]));
});

test("a page's reading view is read again when its revision is newer than the one shown, not when it is the same", async () => {
  const { server, events } = await open();
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2, assets_expire_at: null });

  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 1 }]));
  await settle();
  expect(viewReads(server.sent)).toEqual([]);

  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 2 }]));
  await waitFor(() => expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, again</p>"));
  expect(viewReads(server.sent)).toEqual(["GET view Guide"]);
});

test("a page's reading view is not read again for an older revision than the one a connection read", async () => {
  const { server, events } = await open();
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2, assets_expire_at: null });
  events.last().send("reset", { reason: "expired" });
  await waitFor(() => expect(events.streams).toHaveLength(2));
  events.last().hello();
  await waitFor(() => expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, again</p>"));
  server.sent.length = 0;

  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 1 }]));
  await settle();

  expect(viewReads(server.sent)).toEqual([]);
});

test("an event that comes while a reading view's first read is out reads it again: that read may be the older", async () => {
  const { server, events, router, holdView } = await open();
  const release = holdView(install.id);
  await act(() => router.navigate(pagePath(install.id)));
  await waitFor(() => expect(server.sent).toContain("GET view Install"));
  server.views.set(install.id, { html: "<p>Install, again</p>", revision: 2, assets_expire_at: null });

  events.last().send("pages", pagesEvent(false, [{ id: install.id, revision: 2 }]));
  await settle();
  release();
  await settle();

  expect(screen.getByRole("article", { name: "Install" }).innerHTML).toBe("<p>Install, again</p>");
});

test("an event of too many pages to name reads the notebook's reading views again, once in the refresher's interval", async () => {
  const { server, events } = await open();
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2, assets_expire_at: null });

  events.last().send("pages", { ...pagesEvent(false, null), notebook_id: "0199a2b4-0000-7000-8000-0000000000b2" });
  await settle();
  expect(viewReads(server.sent)).toEqual([]);

  events.last().send("pages", pagesEvent(false, null));
  events.last().send("pages", pagesEvent(false, null));

  await waitFor(() => expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, again</p>"));
  await settle();
  expect(viewReads(server.sent)).toEqual(["GET view Guide"]);
});

test("a hidden tab reads the reading view once it is shown again", async () => {
  const page = new FakePage();
  const { server, events } = await open(page);
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2, assets_expire_at: null });

  page.shown = false;
  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 2 }]));
  await settle();
  expect(server.sent).toEqual([]);

  act(() => page.show(true));
  await waitFor(() => expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, again</p>"));
});

test("an event of a type a later M adds goes to the app's handler of its type, with its data, which can read again what it changed; one of no handler is let be", async () => {
  const later: unknown[] = [];
  const handler: EventHandler = (data, { mutate }) => {
    later.push(data);
    void mutate(["pages", notebookJSON.id]);
  };
  const { server, events } = await open(new FakePage(), {}, new Map([...eventHandlers, ["later", handler]]));

  events.last().send("later", { workspace_id: workspaceJSON.id, page_id: guide.id });
  events.last().send("watch", { workspace_id: workspaceJSON.id });
  await settle();

  expect(later).toEqual([{ workspace_id: workspaceJSON.id, page_id: guide.id }]);
  expect(server.sent).toEqual(["GET nodes"]);
});

test("a mode event reads the workspace's notebooks again, through the app's handlers", async () => {
  let sent: string[] = [];
  const { server, events, workspaces, lockReads } = await open(new FakePage(), {
    "GET /api/v0/workspaces/lab/notebooks": () => {
      sent.push("GET notebooks");
      return json({ data: [notebookJSON] });
    },
  });
  await settle();
  // The route is called during open too; record alongside page reads once the connection has settled.
  sent = server.sent;
  sent.length = 0;
  const stream = events.last();

  stream.send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });
  await settle();

  expect(server.sent).toEqual(["GET notebooks"]);
  expect(workspaces).toEqual([]);
  expect(lockReads).toEqual([]);
  expect(events.streams).toEqual([stream]);
});

const linksEvent = (pages: string[] | null, notebook = notebookJSON.id) => ({
  workspace_id: workspaceJSON.id,
  notebook_id: notebook,
  pages,
  targets: [],
});

test("a links event reads again the reading view of a page it names, at the revision shown, not those of pages not shown", async () => {
  const { server, events } = await open();
  server.views.set(guide.id, { html: "<p>Guide, its links again</p>", revision: 1, assets_expire_at: null });

  events.last().send("links", linksEvent([install.id]));
  await settle();
  expect(viewReads(server.sent)).toEqual([]);

  events.last().send("links", linksEvent([guide.id, install.id]));
  await waitFor(() =>
    expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, its links again</p>")
  );
  expect(viewReads(server.sent)).toEqual(["GET view Guide"]);
});

test("a links event of too many pages to name reads the notebook's reading views again, not another notebook's", async () => {
  const { server, events } = await open();
  server.views.set(guide.id, { html: "<p>Guide, its links again</p>", revision: 1, assets_expire_at: null });

  events.last().send("links", linksEvent(null, "0199a2b4-0000-7000-8000-0000000000b2"));
  await settle();
  expect(viewReads(server.sent)).toEqual([]);

  events.last().send("links", linksEvent(null));
  await waitFor(() =>
    expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, its links again</p>")
  );
  expect(viewReads(server.sent)).toEqual(["GET view Guide"]);
});

test("a links event reads again the backlinks of the pages whose backlinks changed, and the properties of the pages it names, of those shown", async () => {
  const { server, events } = await open();
  server.backlinks.set(guide.id, [{ data: [{ id: notes.id, count: 1, contexts: ["[[Guide]]"] }], next_cursor: null }]);
  server.properties.set(guide.id, {
    valid: true,
    properties: [{ key: "status", value: "draft" }],
    links: [],
    assets_expire_at: null,
  });

  events.last().send("links", { ...linksEvent([install.id]), targets: [install.id] });
  await settle();
  expect(server.sent).toEqual([]);

  events.last().send("links", { ...linksEvent([]), targets: [guide.id] });
  const panel = screen.getByRole("complementary", { name: "About this page" });
  expect(await within(panel).findByRole("link", { name: "Notes" })).toBeTruthy();
  expect(server.sent).toEqual([`GET backlinks ${guide.id}`]);

  server.sent.length = 0;
  events.last().send("links", linksEvent([guide.id]));
  expect(await within(panel).findByText("draft")).toBeTruthy();
  await settle();
  expect(server.sent.toSorted()).toEqual([`GET properties ${guide.id}`, "GET view Guide"]);
});

test("a links event of too many pages to name reads again every backlinks and properties shown of the notebook, not another notebook's", async () => {
  const { server, events } = await open();
  server.backlinks.set(guide.id, [{ data: [{ id: notes.id, count: 1, contexts: [] }], next_cursor: null }]);
  server.properties.set(guide.id, {
    valid: true,
    properties: [{ key: "status", value: "draft" }],
    links: [],
    assets_expire_at: null,
  });

  events.last().send("links", { ...linksEvent(null, "0199a2b4-0000-7000-8000-0000000000b2"), targets: null });
  await settle();
  expect(server.sent).toEqual([]);

  events.last().send("links", { ...linksEvent([]), targets: null });
  const panel = screen.getByRole("complementary", { name: "About this page" });
  expect(await within(panel).findByRole("link", { name: "Notes" })).toBeTruthy();
  events.last().send("links", { ...linksEvent(null), targets: [] });
  expect(await within(panel).findByText("draft")).toBeTruthy();
  await settle();
  expect(server.sent.filter((each) => !each.startsWith("GET view")).toSorted()).toEqual([
    `GET backlinks ${guide.id}`,
    `GET properties ${guide.id}`,
  ]);
});

test("a pages event reads again the properties and the versions of the pages written that are shown, of every one for too many to name", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const { server, events } = await open();
  server.properties.set(guide.id, {
    valid: true,
    properties: [{ key: "status", value: "draft" }],
    links: [],
    assets_expire_at: null,
  });

  events.last().send("pages", pagesEvent(false, [{ id: install.id, revision: 2 }]));
  await settle();
  expect(server.sent).toEqual([]);

  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 1 }]));
  const panel = screen.getByRole("complementary", { name: "About this page" });
  expect(await within(panel).findByText("draft")).toBeTruthy();
  await waitFor(() => expect(server.sent.toSorted()).toEqual([`GET properties ${guide.id}`, "GET versions Guide"]));

  server.properties.set(guide.id, {
    valid: true,
    properties: [{ key: "status", value: "done" }],
    links: [],
    assets_expire_at: null,
  });
  events.last().send("pages", pagesEvent(false, null));
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  expect(await within(panel).findByText("done")).toBeTruthy();
});

/** onTag opens the tag t's pages, Guide's, after open, and waits until they show: what is read from then on is the events' doing. */
async function onTag(opened: Awaited<ReturnType<typeof open>>) {
  const { server, router } = opened;
  server.tags.set("t", [guide.id]);
  await act(() => router.navigate(`/lab/notebooks/${notebookJSON.id}/tags/t`));
  await screen.findByRole("list", { name: "Pages tagged #t" });
  await settle();
  server.sent.length = 0;
}

test("an event of the notebook's pages written, or its tree, reads the pages of its tags shown again, once in the refresher's interval", async () => {
  const opened = await open();
  const { server, events } = opened;
  await onTag(opened);
  server.tags.set("t", [guide.id, notes.id]);

  events.last().send("pages", { ...pagesEvent(true, null), notebook_id: "0199a2b4-0000-7000-8000-0000000000b2" });
  events.last().send("pages", pagesEvent(false, []));
  await settle();
  expect(server.sent).toEqual([]);

  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 2 }]));
  events.last().send("pages", pagesEvent(true, []));
  const list = screen.getByRole("list", { name: "Pages tagged #t" });
  await waitFor(() =>
    expect(
      within(list)
        .getAllByRole("link")
        .map((link) => link.textContent)
    ).toEqual(["Guide", "Notes"])
  );
  await settle();
  expect(server.sent).toEqual(["GET tag t", "GET nodes"]);
});

test("an event of the tree alone, and one of too many pages to name, each read the pages of the notebook's tags again", async () => {
  for (const event of [pagesEvent(true, []), pagesEvent(false, null)]) {
    // oxlint-disable-next-line no-await-in-loop -- one event, in a refresher of its own, after another
    const opened = await open();
    // oxlint-disable-next-line no-await-in-loop -- as above
    await onTag(opened);
    opened.events.last().send("pages", event);
    // oxlint-disable-next-line no-await-in-loop -- as above
    await waitFor(() => expect(opened.server.sent).toContain("GET tag t"));
    opened.unmount();
  }
});

test("a links event of too many pages to name reads the pages of the notebook's tags again; one that names its pages does not", async () => {
  const opened = await open();
  const { server, events } = opened;
  await onTag(opened);

  events.last().send("links", linksEvent([guide.id]));
  events.last().send("links", linksEvent(null, "0199a2b4-0000-7000-8000-0000000000b2"));
  await settle();
  expect(server.sent).toEqual([]);

  events.last().send("links", linksEvent(null));
  await waitFor(() => expect(server.sent).toEqual(["GET tag t"]));
});

test("an event of a page's lock reads its lock again", async () => {
  const { server, events } = await open();
  server.hold(guide.id, bob, 60);

  events.last().send("lock", lockEvent(guide.id));

  expect((await screen.findByRole("status")).textContent).toContain("Bob is editing this page.");
});

test("a lock event reads the tree first: a page deleted while edited leaves before its lock would be read", async () => {
  const { server, events, lockReads } = await open();
  server.nodes = [notes];

  events.last().send("lock", lockEvent(guide.id));

  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
  await settle();
  expect(lockReads).toEqual([]);
});

test("each connection reads again the workspaces, the tree, the reading view, the lock and the attachments", async () => {
  const { server, events, workspaces } = await open();
  server.nodes = [{ ...guide, name: "Handbook" }, install, linux, notes];
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2, assets_expire_at: null });
  server.hold(guide.id, bob, 60);

  events.last().send("reset", { reason: "expired" });
  await waitFor(() => expect(events.streams).toHaveLength(2));
  events.last().hello();

  expect(await screen.findByRole("heading", { level: 1, name: "Handbook" })).toBeTruthy();
  await waitFor(() => expect(screen.getByRole("article", { name: "Handbook" }).innerHTML).toBe("<p>Guide, again</p>"));
  expect((await screen.findByRole("status")).textContent).toContain("Bob is editing this page.");
  expect(workspaces).toEqual(["GET workspaces"]);
  await waitFor(() => expect(server.sent).toContain("GET assets Handbook"));
});

test("each connection reads again the pages of a tag shown, with the tree", async () => {
  const opened = await open();
  const { server, events } = opened;
  await onTag(opened);

  events.last().send("reset", { reason: "expired" });
  await waitFor(() => expect(events.streams).toHaveLength(2));
  events.last().hello();

  await waitFor(() => expect(server.sent).toEqual(expect.arrayContaining(["GET nodes", "GET tag t"])));
  await settle();
  expect(server.sent.toSorted()).toEqual(["GET nodes", "GET tag t"]);
});

test("a connection reads from the outside in: a notebook no longer seen leaves the page before its tree is read", async () => {
  let seen = true;
  const { server, events } = await open(new FakePage(), {
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: seen ? [{ ...notebookJSON, role: "admin" }] : [] }),
  });
  seen = false;
  server.nodesDown = true;

  events.last().send("reset", { reason: "access" });
  await waitFor(() => expect(events.streams).toHaveLength(2));
  events.last().hello();

  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
  await settle();
  expect(server.sent).toEqual([]);
});

test("an event whose tree lost the page shown reads none of its attachments: the page leaves first", async () => {
  const { server, events } = await open();
  server.nodes = [notes];

  events.last().send("pages", pagesEvent(true, []));

  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
  await settle();
  expect(server.sent).toEqual(["GET nodes"]);
});

test("a connection reads the right column after the tree: a page deleted meanwhile leaves before its backlinks and properties would be read", async () => {
  const { server, events } = await open();
  server.nodes = [notes];

  events.last().send("reset", { reason: "expired" });
  await waitFor(() => expect(events.streams).toHaveLength(2));
  events.last().hello();

  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
  await settle();
  expect(server.sent).toEqual(["GET nodes"]);
});

test("the tab editing the page does not read its reading view again", async () => {
  const user = userEvent.setup();
  const { server, events } = await open();
  await user.click(screen.getByRole("button", { name: "Edit" }));
  await screen.findByRole("textbox", { name: "Page content" });
  server.sent.length = 0;

  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 2 }]));
  await settle();

  expect(server.sent.filter((sent) => sent.startsWith("GET view"))).toEqual([]);
});

test("the tab that edited the page reads it once back, not again for the events of its own saves", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const { server, events } = await open();
  ctrl("e");
  const content = await screen.findByRole("textbox", { name: "Page content" });
  await act(async () => {});
  const view = EditorView.findFromDOM(content.closest<HTMLElement>(".cm-editor") ?? content);
  const save = async (text: string, revision: number) => {
    view?.dispatch({ changes: { from: view.state.doc.length, insert: text }, userEvent: "input.type" });
    ctrl("s");
    await screen.findByText("Saved.");
    events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision }]));
    await settle();
  };
  await save("one", 2);
  await save(" two", 3);

  ctrl("e");
  await screen.findByRole("button", { name: "Edit" });
  await settle();
  const reads = server.sent.filter((sent) => sent === "GET view Guide").length;
  await act(() => vi.advanceTimersByTimeAsync(6_000));

  expect(server.sent.filter((sent) => sent === "GET view Guide")).toHaveLength(reads);
});

/** versionPath is the address of Guide's version that holds revision. */
const versionPath = (revision: number) => `${pagePath(guide.id)}/versions/${revision.toString()}`;

/**
 * openVersion renders Guide's version that holds revision, of versions,
 * over pageServer and its event stream, connected, and waits for what the
 * connection read again: from then on, what is read is the events' doing.
 */
async function openVersion(revision: number, versions: PageVersionContent[]) {
  const events = eventServer();
  const server = pageServer({ answers: { "GET /api/v0/events": events.answer } });
  server.withVersions(guide.id, versions);
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
  const view = renderApp(versionPath(revision), withEvents(server.app), { eventHandlers: handlers });
  await screen.findByRole("article", { name: `Version ${revision.toString()}` });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const before = server.sent.length;
  events.last().hello();
  await waitFor(() =>
    expect(server.sent.slice(before)).toEqual(
      expect.arrayContaining([
        `GET version Guide ${revision.toString()}`,
        `GET version view Guide ${revision.toString()}`,
        "GET versions Guide",
      ])
    )
  );
  await settle();
  server.sent.length = 0;
  /** versionsCached are the keys of the versions' contents and views SWR's cache holds data of. */
  const versionsCached = async () => {
    events.last().send("probe", {});
    await settle();
    const cache = probed.cache;
    return cache === undefined
      ? []
      : [...cache.keys()].filter((key) => /^@"page-version(-view)?",/.test(key) && cache.get(key)?.data !== undefined);
  };
  return { ...view, server, events, versionsCached };
}

/** versionArticle is the view of Guide's version 1. */
const versionArticle = () => screen.getByRole("article", { name: "Version 1" });

/** versionReads is what of sent read a version, its view, or the page's versions. */
function versionReads(sent: string[]): string[] {
  return sent.filter((each) => each.startsWith("GET version"));
}

test("a pages event reads the version shown again while it is being edited, its address following the revision the session's saves move it to", async () => {
  const first = versionJSON(1, null, "Guide\n");
  const edited = versionJSON(3, 1, "Guide\nmore\n", { by: bob, editing: true });
  const { server, events, router, versionsCached } = await openVersion(3, [first, edited]);
  expect(screen.getByText("Editing")).toBeTruthy();

  server.withVersions(guide.id, [first, { ...edited, revision: 5, content: "Guide\nmore and more\n" }]);
  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 5 }]));

  expect(await screen.findByRole("heading", { level: 1, name: "Version 5" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(versionPath(5));
  await waitFor(() =>
    expect(screen.getByRole("article", { name: "Version 5" }).textContent).toBe("Guide\nmore and more")
  );
  await settle();
  // The view is read once, at the revision moved to; what was read at the address's revision leaves the cache.
  expect(versionReads(server.sent).toSorted()).toEqual([
    "GET version Guide 3",
    "GET version view Guide 5",
    "GET versions Guide",
  ]);
  expect((await versionsCached()).toSorted()).toEqual([
    unstable_serialize(["page-version", notebookJSON.id, guide.id, 5]),
    unstable_serialize(["page-version-view", notebookJSON.id, guide.id, 5]),
  ]);
});

test("a save of the version shown, its tab comparing with the version before it, keeps that one: read once, it is not dropped", async () => {
  const user = userEvent.setup();
  const first = versionJSON(1, null, "Guide\n");
  const edited = versionJSON(3, 1, "Guide\nmore\n", { by: bob, editing: true });
  const { server, events } = await openVersion(3, [first, edited]);
  await user.click(screen.getByRole("tab", { name: "Compare with the previous" }));
  await screen.findByRole("textbox", { name: "Changes from version 1 to version 3" });
  server.sent.length = 0;

  server.withVersions(guide.id, [first, { ...edited, revision: 5, content: "Guide\nmore and more\n" }]);
  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 5 }]));

  expect(await screen.findByRole("textbox", { name: "Changes from version 1 to version 5" })).toBeTruthy();
  await settle();
  expect(versionReads(server.sent).toSorted()).toEqual(["GET version Guide 3", "GET versions Guide"]);
});

test("a pages event reads no version shown that is not being edited: it does not change; the page's versions it reads", async () => {
  const { server, events } = await openVersion(1, [
    versionJSON(1, null, "Guide\n"),
    versionJSON(3, 1, "Guide\nmore\n", { by: bob, editing: true }),
  ]);

  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 4 }]));
  await waitFor(() => expect(server.sent).toContain("GET versions Guide"));
  await settle();

  expect(versionReads(server.sent)).toEqual(["GET versions Guide"]);
});

test("a lock event reads the page's versions again at once, and the content of its version shown being edited, not its view; its session over, neither is marked editing", async () => {
  const user = userEvent.setup();
  const first = versionJSON(1, null, "Guide\n");
  const edited = versionJSON(3, 1, "Guide\nmore\n", { by: bob, editing: true });
  const { server, events } = await openVersion(3, [first, edited]);

  events.last().send("lock", lockEvent(notes.id));
  await settle();
  expect(versionReads(server.sent)).toEqual([]);

  // Its view shows: the revision does not move, and the view stays.
  events.last().send("lock", lockEvent(guide.id));
  await waitFor(() => expect(versionReads(server.sent)).toHaveLength(2));
  await settle();
  expect(versionReads(server.sent).toSorted()).toEqual(["GET version Guide 3", "GET versions Guide"]);

  // The version before it shows, which is not being edited.
  await user.click(screen.getByRole("tab", { name: "Compare with the previous" }));
  await screen.findByRole("textbox", { name: "Changes from version 1 to version 3" });
  server.sent.length = 0;
  server.withVersions(guide.id, [first, { ...edited, editing: false }]);
  events.last().send("lock", lockEvent(guide.id));

  await waitFor(() => expect(screen.queryByText("Editing")).toBeNull());
  await settle();
  expect(versionReads(server.sent).toSorted()).toEqual(["GET version Guide 3", "GET versions Guide"]);
});

test("a links event reads again the views of the versions shown of the pages it names; a tree changed, every one's; another page's, none", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const { server, events } = await openVersion(1, [versionJSON(1, null, "Guide\n"), versionJSON(2, 1, "Guide 2\n")]);

  events
    .last()
    .send("links", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, pages: [notes.id], targets: [] });
  await settle();
  expect(versionReads(server.sent)).toEqual([]);

  server.versionViews.set(`${guide.id} 1`, { html: "<p>Linked</p>", revision: 1, assets_expire_at: null });
  events
    .last()
    .send("links", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, pages: [guide.id], targets: [] });
  await waitFor(() => expect(versionArticle().innerHTML).toBe("<p>Linked</p>"));

  server.versionViews.set(`${guide.id} 1`, { html: "<p>Renamed</p>", revision: 1, assets_expire_at: null });
  events.last().send("pages", pagesEvent(true, []));
  await waitFor(() => expect(versionArticle().innerHTML).toBe("<p>Renamed</p>"));
  expect(versionReads(server.sent)).toEqual(["GET version view Guide 1", "GET version view Guide 1"]);
});

test("a links event of a write that changed the aliases reads again every version's view shown of its notebook, and only those; one that kept them, or an ordinary save, none it does not name", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const { server, events } = await openVersion(1, [versionJSON(1, null, "Guide\n"), versionJSON(2, 1, "Guide 2\n")]);
  const of = { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id };

  events.last().send("pages", pagesEvent(false, [{ id: notes.id, revision: 2 }]));
  events.last().send("links", { ...of, pages: [notes.id], targets: [guide.id], aliases: false });
  events.last().send("links", {
    ...of,
    notebook_id: "0199a2b4-0000-7000-8000-0000000000b2",
    pages: [],
    targets: [],
    aliases: true,
  });
  await settle();
  expect(versionReads(server.sent).filter((each) => each.startsWith("GET version view"))).toEqual([]);

  server.sent.length = 0;
  server.versionViews.set(`${guide.id} 1`, { html: "<p>Unresolved now</p>", revision: 1, assets_expire_at: null });
  events.last().send("links", { ...of, pages: [], targets: [], aliases: true });
  await waitFor(() => expect(versionArticle().innerHTML).toBe("<p>Unresolved now</p>"));
  await settle();
  expect(server.sent).toEqual(["GET version view Guide 1"]);
});

/** compared is what the comparison of the page now with Guide's version 1 marks. */
const compared = () => marked(screen.getByRole("textbox", { name: "Changes from the page now to version 1" }));

test("a pages event reads again the page as it is now a version is compared with: of the page written newer, or of every page", async () => {
  const user = userEvent.setup();
  const { server, events } = await openVersion(1, [versionJSON(1, null, "Guide\n"), versionJSON(2, 1, "Guide 2\n")]);
  await user.click(screen.getByRole("tab", { name: "Compare with the current" }));
  await waitFor(() => expect(compared().removed).toEqual(["Guide 2"]));
  server.sent.length = 0;

  server.contents.set(guide.id, { content: "Guide 3\n", revision: 3 });
  events.last().send("pages", pagesEvent(false, [{ id: notes.id, revision: 9 }]));
  await settle();
  expect(server.sent).not.toContain("GET content Guide");
  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 3 }]));
  await waitFor(() => expect(compared().removed).toEqual(["Guide 3"]));

  server.contents.set(guide.id, { content: "Guide 4\n", revision: 4 });
  events.last().send("pages", pagesEvent(false, null));
  await waitFor(() => expect(compared().removed).toEqual(["Guide 4"]));
  expect(server.sent.filter((each) => each === "GET content Guide")).toHaveLength(2);
});

test("each connection reads again the version shown, the page's versions, and the page as it is now it is compared with", async () => {
  const user = userEvent.setup();
  const { server, events } = await openVersion(1, [versionJSON(1, null, "Guide\n"), versionJSON(2, 1, "Guide 2\n")]);
  await user.click(screen.getByRole("tab", { name: "Compare with the current" }));
  await waitFor(() => expect(server.sent).toContain("GET content Guide"));
  server.sent.length = 0;

  events.last().send("reset", { reason: "expired" });
  await waitFor(() => expect(events.streams).toHaveLength(2));
  events.last().hello();

  await waitFor(() =>
    expect(server.sent).toEqual(
      expect.arrayContaining(["GET version Guide 1", "GET versions Guide", "GET content Guide"])
    )
  );
});

/**
 * unmountedOnTree has the app unmounted as its tree is read: the refresh
 * that read it is between its steps. What the refresh asks of the cache
 * after that is in rejections, unhandled.
 */
async function unmountedOnTree(ask: (opened: Awaited<ReturnType<typeof open>>) => void) {
  const rejections: unknown[] = [];
  const record = (reason: unknown) => void rejections.push(reason);
  process.on("unhandledRejection", record);
  try {
    const opened = await open();
    const push = opened.server.sent.push.bind(opened.server.sent);
    opened.server.sent.push = (...lines: string[]) => {
      if (lines.includes("GET nodes")) {
        opened.unmount();
      }
      return push(...lines);
    };
    ask(opened);
    await waitFor(() => expect(opened.server.sent).toContain("GET nodes"));
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(opened.lockReads).toEqual([]);
    expect(rejections).toEqual([]);
  } finally {
    process.off("unhandledRejection", record);
  }
}

test("a lock's refresh still going on when the stream stops reads no more: its cache went with the generation", async () => {
  await unmountedOnTree(({ events }) => events.last().send("lock", lockEvent(guide.id)));
});

test("a connection's refresh still going on when the stream stops reads no more", async () => {
  await unmountedOnTree(({ events }) => {
    events.last().send("reset", { reason: "expired" });
    void waitFor(() => expect(events.streams).toHaveLength(2)).then(() => events.last().hello());
  });
});

test("the stream ends with the generation", async () => {
  const { app, events } = await open();

  await act(() => app.session.tokens.signOut());

  await waitFor(() => expect(events.last().ended).toBe(true));
});
