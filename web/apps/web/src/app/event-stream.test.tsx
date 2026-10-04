import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditorView } from "@codemirror/view";
import { afterEach, expect, test, vi } from "vitest";

import { eventHandlers, type EventHandler } from "../events/handlers";
import { FakePage } from "../events/testing/fake-page";
import { eventServer, withEvents } from "../test/event-server";
import { json, notebookJSON, problem, workspaceJSON, type Answer } from "../test/fakes";
import { bob, guide, install, linux, notes, pagePath, pageServer } from "../test/page-server";
import { renderApp } from "../test/render";

// Each event has what it changed read again (M5/P3 design 3.8): the tree,
// a page's reading view whose revision is newer, a page's edit lock; each
// connection, all of them.

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
        const view = node && (server.views.get(node.id) ?? { html: `<p>${node.name}</p>`, revision: 1 });
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
    expect(server.sent.slice(before)).toEqual(expect.arrayContaining(["GET nodes", "GET view Guide"]))
  );
  server.sent.length = 0;
  workspaces.length = 0;
  lockReads.length = 0;
  return { ...view, server, events, workspaces, lockReads, holdView };
}

afterEach(() => vi.useRealTimers());

/** Presses Ctrl and key on the page. */
function ctrl(key: string) {
  document.dispatchEvent(new KeyboardEvent("keydown", { key, ctrlKey: true, bubbles: true, cancelable: true }));
}

/** Lets what the events asked for go out. */
async function settle(): Promise<void> {
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
}

test("an event whose tree changed reads the tree again, not the reading views", async () => {
  const { server, events } = await open();
  server.nodes = [{ ...guide, name: "Handbook" }, install, linux, notes];

  events.last().send("pages", pagesEvent(true, []));

  expect(await screen.findByRole("heading", { level: 1, name: "Handbook" })).toBeTruthy();
  expect(server.sent).toEqual(["GET nodes"]);
});

test("a page's reading view is read again when its revision is newer than the one shown, not when it is the same", async () => {
  const { server, events } = await open();
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2 });

  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 1 }]));
  await settle();
  expect(server.sent).toEqual([]);

  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 2 }]));
  await waitFor(() => expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, again</p>"));
  expect(server.sent).toEqual(["GET view Guide"]);
});

test("a page's reading view is not read again for an older revision than the one a connection read", async () => {
  const { server, events } = await open();
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2 });
  events.last().send("reset", { reason: "expired" });
  await waitFor(() => expect(events.streams).toHaveLength(2));
  events.last().hello();
  await waitFor(() => expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, again</p>"));
  server.sent.length = 0;

  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 1 }]));
  await settle();

  expect(server.sent).toEqual([]);
});

test("an event that comes while a reading view's first read is out reads it again: that read may be the older", async () => {
  const { server, events, router, holdView } = await open();
  const release = holdView(install.id);
  await act(() => router.navigate(pagePath(install.id)));
  await waitFor(() => expect(server.sent).toContain("GET view Install"));
  server.views.set(install.id, { html: "<p>Install, again</p>", revision: 2 });

  events.last().send("pages", pagesEvent(false, [{ id: install.id, revision: 2 }]));
  await settle();
  release();
  await settle();

  expect(screen.getByRole("article", { name: "Install" }).innerHTML).toBe("<p>Install, again</p>");
});

test("an event of too many pages to name reads the notebook's reading views again, once in the refresher's interval", async () => {
  const { server, events } = await open();
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2 });

  events.last().send("pages", { ...pagesEvent(false, null), notebook_id: "0199a2b4-0000-7000-8000-0000000000b2" });
  await settle();
  expect(server.sent).toEqual([]);

  events.last().send("pages", pagesEvent(false, null));
  events.last().send("pages", pagesEvent(false, null));

  await waitFor(() => expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, again</p>"));
  await settle();
  expect(server.sent).toEqual(["GET view Guide"]);
});

test("a hidden tab reads the reading view once it is shown again", async () => {
  const page = new FakePage();
  const { server, events } = await open(page);
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2 });

  page.shown = false;
  events.last().send("pages", pagesEvent(false, [{ id: guide.id, revision: 2 }]));
  await settle();
  expect(server.sent).toEqual([]);

  act(() => page.show(true));
  await waitFor(() => expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, again</p>"));
});

test("an event of a type a later M adds goes to the app's handler of its type, with its data, which can read again what it changed; one of no handler is let be", async () => {
  const links: unknown[] = [];
  const handler: EventHandler = (data, { mutate }) => {
    links.push(data);
    void mutate(["pages", notebookJSON.id]);
  };
  const { server, events } = await open(new FakePage(), {}, new Map([...eventHandlers, ["links", handler]]));

  events.last().send("links", { workspace_id: workspaceJSON.id, page_id: guide.id });
  events.last().send("modes", { workspace_id: workspaceJSON.id });
  await settle();

  expect(links).toEqual([{ workspace_id: workspaceJSON.id, page_id: guide.id }]);
  expect(server.sent).toEqual(["GET nodes"]);
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

test("each connection reads again the workspaces, the tree, the reading view and the lock", async () => {
  const { server, events, workspaces } = await open();
  server.nodes = [{ ...guide, name: "Handbook" }, install, linux, notes];
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2 });
  server.hold(guide.id, bob, 60);

  events.last().send("reset", { reason: "expired" });
  await waitFor(() => expect(events.streams).toHaveLength(2));
  events.last().hello();

  expect(await screen.findByRole("heading", { level: 1, name: "Handbook" })).toBeTruthy();
  await waitFor(() => expect(screen.getByRole("article", { name: "Handbook" }).innerHTML).toBe("<p>Guide, again</p>"));
  expect((await screen.findByRole("status")).textContent).toContain("Bob is editing this page.");
  expect(workspaces).toEqual(["GET workspaces"]);
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
