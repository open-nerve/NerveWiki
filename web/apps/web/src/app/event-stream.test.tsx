import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { FakePage } from "../events/testing/fake-page";
import type { EditLock } from "../services/page.service";
import { eventServer, withEvents } from "../test/event-server";
import { json, notebookJSON, workspaceJSON, type Answer } from "../test/fakes";
import { guide, install, linux, notes, pagePath, pageServer } from "../test/page-server";
import { renderApp } from "../test/render";

// Each event has what it changed read again (M5/P3 design 3.8): the tree,
// a page's reading view whose revision is newer, a page's edit lock; each
// connection, all of them.

const bobEditing: EditLock = {
  holder: { user_id: "0199a2b4-0000-7000-8000-000000000002", display_name: "Bob" },
  expires_in: 60,
};

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

/**
 * open renders Guide over pageServer and its event stream, connected, and
 * waits for what the connection read again: from then on, what is read is
 * the events' doing.
 */
async function open(page = new FakePage(), answers: Record<string, Answer> = {}) {
  const events = eventServer();
  const workspaces: string[] = [];
  const server = pageServer({
    answers: {
      ...answers,
      "GET /api/v0/events": events.answer,
      "GET /api/v0/workspaces": () => {
        workspaces.push("GET workspaces");
        return json({ data: [workspaceJSON] });
      },
    },
  });
  const view = renderApp(pagePath(guide.id), withEvents(server.app, page));
  expect((await screen.findByRole("article", { name: "Guide" })).innerHTML).toBe("<p>Guide</p>");
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const before = server.sent.length;
  events.last().hello();
  await waitFor(() =>
    expect(server.sent.slice(before)).toEqual(expect.arrayContaining(["GET nodes", "GET view Guide"]))
  );
  server.sent.length = 0;
  workspaces.length = 0;
  return { ...view, server, events, workspaces };
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

test("an event of too many pages to name reads every reading view of the notebook again", async () => {
  const { server, events } = await open();
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2 });

  events.last().send("pages", pagesEvent(false, null));

  await waitFor(() => expect(screen.getByRole("article", { name: "Guide" }).innerHTML).toBe("<p>Guide, again</p>"));
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

test("an event of a page's lock reads its lock again", async () => {
  const { server, events } = await open();
  server.lock = bobEditing;

  events.last().send("lock", lockEvent(guide.id));

  expect((await screen.findByRole("status")).textContent).toContain("Bob is editing this page.");
});

test("each connection reads again the workspaces, the tree, the reading view and the lock", async () => {
  const { server, events, workspaces } = await open();
  server.nodes = [{ ...guide, name: "Handbook" }, install, linux, notes];
  server.views.set(guide.id, { html: "<p>Guide, again</p>", revision: 2 });
  server.lock = bobEditing;

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

test("the stream ends with the generation", async () => {
  const { app, events } = await open();

  await act(() => app.session.tokens.signOut());

  await waitFor(() => expect(events.last().ended).toBe(true));
});
