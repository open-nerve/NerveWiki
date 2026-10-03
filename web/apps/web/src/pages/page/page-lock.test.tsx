import { act, configure, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, beforeAll, expect, test } from "vitest";

import { editorExtensions } from "../../editor/registry";
import { FakePage } from "../../events/testing/fake-page";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, problem, workspaceJSON } from "../../test/fakes";
import { pageEditor } from "../../test/page-editor";
import { ada, bob, guide, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The edit lock on the page (M5/P4 design 3.6–3.9), in StrictMode as the
// app runs, with the app's editor extensions: an edit opens its session
// first; one lost says why and is read-only; a page or notebook gone while
// edited unsaved stays until the edit ends.

beforeAll(() => configure({ reactStrictMode: true }));
afterAll(() => configure({ reactStrictMode: false }));

/** Presses Ctrl+key where the focus is. */
const ctrl = (key: string) => fireEvent.keyDown(document.activeElement ?? document.body, { key, ctrlKey: true });
/** The tab shown again: the edit's session beats. */
const shown = () => act(() => void document.dispatchEvent(new Event("visibilitychange")));

/** Guide shown to Ada over server, with the app's editor extensions, and Edit pressed. */
async function pressEdit(server = pageServer()) {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), server.app, { editorExtensions });
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  return { user, server };
}

test("a page someone else edits keeps its reading view: Edit says who, the focus there, and reads no content", async () => {
  const server = pageServer();
  server.hold(guide.id, bob);
  await pressEdit(server);

  await waitFor(() => expect(document.activeElement?.textContent).toBe("Bob is editing this page.Release lock"));
  expect(screen.getByRole("article", { name: "Guide" })).toBeTruthy();
  expect(screen.queryByRole("textbox", { name: "Page content" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Edit here" })).toBeNull();
  expect(server.sent).toContain("OPEN Guide");
  expect(server.sent).not.toContain("GET content Guide");
});

test("Edit is busy while the session opens: pressed again, or Ctrl+E, it opens one", async () => {
  let opens = 0;
  let open: (() => void) | undefined;
  const { user } = await pressEdit(
    pageServer({
      answers: {
        "POST /api/v0/pages/*/edit-sessions": () => {
          opens++;
          return new Promise((resolve) => {
            open = () => resolve(json({ id: "s1", page_id: guide.id, expires_at: "2026-10-03T08:02:00Z" }, 201));
          });
        },
      },
    })
  );
  const edit = screen.getByRole("button", { name: "Edit" });
  expect(edit.getAttribute("aria-busy")).toBe("true");
  await user.click(edit);
  ctrl("e");
  await act(async () => {});
  expect(opens).toBe(1);

  act(() => open?.());
  await pageEditor();
});

test("a session that opens once the page is left is ended", async () => {
  let open: (() => void) | undefined;
  const server = pageServer({
    answers: {
      "POST /api/v0/pages/*/edit-sessions": () =>
        new Promise((resolve) => {
          open = () => resolve(json({ id: "s1", page_id: guide.id, expires_at: "2026-10-03T08:02:00Z" }, 201));
        }),
    },
  });
  const { user } = await pressEdit(server);
  await waitFor(() => expect(open).toBeDefined());

  await user.click(screen.getByRole("link", { name: "Notes" }));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  act(() => open?.());

  await waitFor(() => expect(server.sent).toContain("END s1"));
});

test("Done goes back to reading once the session's end is answered: the lock read next is not the edit's own", async () => {
  let ended: (() => void) | undefined;
  const server = pageServer({
    answers: {
      "DELETE /api/v0/edit-sessions/*": () =>
        new Promise((resolve) => {
          ended = () => {
            server.sessions.clear();
            resolve(new Response(null, { status: 204 }));
          };
        }),
    },
  });
  const { user } = await pressEdit(server);
  await pageEditor();

  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(ended).toBeDefined());
  await act(async () => {});
  expect(screen.queryByRole("article", { name: "Guide" })).toBeNull();
  act(() => ended?.());

  expect(await screen.findByRole("article", { name: "Guide" })).toBeTruthy();
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(screen.queryByText("You are editing this page elsewhere.")).toBeNull();
});

test("the account editing the page elsewhere may edit here: the edit elsewhere is taken over", async () => {
  const server = pageServer();
  const elsewhere = server.hold(guide.id, ada);
  const { user } = await pressEdit(server);

  await screen.findByText("You are editing this page elsewhere.");
  await user.click(screen.getByRole("button", { name: "Edit here" }));

  const { content } = await pageEditor();
  await waitFor(() => expect(document.activeElement).toBe(content));
  expect(server.sent).toEqual(expect.arrayContaining(["OPEN Guide", "OPEN Guide TAKE", "GET content Guide"]));
  expect(server.sessions.get(elsewhere)?.ended).toEqual({ code: "page.edit_session_taken_over" });
});

test.each<[string, (server: ReturnType<typeof pageServer>) => void, string]>([
  [
    "taken over elsewhere",
    (server) => void server.takeOver(guide.id),
    "You went on editing this page elsewhere: this editor saves no more.",
  ],
  [
    "unlocked by an admin",
    (server) => server.unlock(guide.id, bob),
    "Bob released your edit of this page: this editor saves no more.",
  ],
  [
    "lapsed, the lock taken meanwhile",
    (server) => {
      server.sessions.clear();
      server.hold(guide.id, bob);
    },
    "Your edit lapsed, and Bob is editing this page now: this editor saves no more.",
  ],
  [
    "lapsed, the page gone",
    (server) => {
      server.sessions.clear();
      server.nodes = [notes];
    },
    "This page no longer exists: this editor saves no more.",
  ],
])(
  "an edit %s says why above the editor, the focus there; the editor is read-only, its text kept",
  async (_, lose, why) => {
    const { server } = await pressEdit();
    const { content, type } = await pageEditor();
    type(" more");

    lose(server);
    shown();

    const banner = await screen.findByRole("alert");
    expect(banner.textContent).toContain(why);
    expect(banner.textContent).toContain("Your changes here are not saved: copy them before you leave.");
    await waitFor(() => expect(document.activeElement).toBe(banner));
    expect(content.getAttribute("contenteditable")).toBe("false");
    expect(content.textContent).toBe("Guide more");
    expect(screen.queryByRole("button", { name: "Done" })).toBeNull();
  }
);

test("an edit whose account may no longer edit the page says so", async () => {
  await pressEdit(
    pageServer({ answers: { "POST /api/v0/edit-sessions/*/heartbeat": () => problem(403, "forbidden") } })
  );
  await pageEditor();

  shown();

  expect((await screen.findByRole("alert")).textContent).toContain("You can no longer edit this page.");
});

test("an edit lost saves nothing: Ctrl+S sends nothing; Back to reading asks first while unsaved, Stay keeps the editor", async () => {
  const { user, server } = await pressEdit();
  const { type } = await pageEditor();
  type(" more");
  server.takeOver(guide.id);
  shown();
  await screen.findByRole("alert");

  ctrl("s");
  await user.click(screen.getByRole("button", { name: "Back to reading" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Leave without saving?" });
  await user.click(within(dialog).getByRole("button", { name: "Stay" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();

  ctrl("e");
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Leave without saving?" })).getByRole("button", {
      name: "Leave",
    })
  );
  const edit = await screen.findByRole("button", { name: "Edit" });
  await waitFor(() => expect(document.activeElement).toBe(edit));
  expect(screen.getByRole("article", { name: "Guide" })).toBeTruthy();
  expect(server.sent.filter((line) => line.startsWith("PUT"))).toEqual([]);
});

test("an edit lost with nothing unsaved goes back to reading at once; its session, ended, is not ended again", async () => {
  const { user, server } = await pressEdit();
  await pageEditor();
  server.unlock(guide.id, bob);
  shown();

  await user.click(await screen.findByRole("button", { name: "Back to reading" }));

  expect(await screen.findByRole("article", { name: "Guide" })).toBeTruthy();
  expect(server.sent.filter((line) => line.startsWith("END"))).toEqual([]);
});

/** Guide shown to Ada over server and its event stream, connected; notebooks is Lab's list, which the test changes. */
async function connected() {
  const events = eventServer();
  const lab = { notebooks: [{ ...notebookJSON, role: "admin" as const }] };
  const server = pageServer({
    answers: {
      "GET /api/v0/events": events.answer,
      "GET /api/v0/workspaces": () => json({ data: [workspaceJSON] }),
      "GET /api/v0/workspaces/lab/notebooks": () => json({ data: lab.notebooks }),
    },
  });
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), withEvents(server.app, new FakePage()), { editorExtensions });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  act(() => events.last().hello());
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  return { user, server, events, lab, ...(await pageEditor()) };
}

/** The event of another tab's deletion of Guide: the tree changed, Guide's lock with it. */
function deleted(events: ReturnType<typeof eventServer>) {
  act(() => {
    events
      .last()
      .send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });
    events.last().send("lock", {
      workspace_id: workspaceJSON.id,
      notebook_id: notebookJSON.id,
      page_id: guide.id,
      session_id: "session-1",
    });
  });
}

test("a page deleted while this tab edits it unsaved stays, saying so, until the edit is left; then it is no page", async () => {
  const { user, server, events, type } = await connected();
  type(" more");

  server.nodes = [notes];
  server.sessions.clear();
  deleted(events);

  expect((await screen.findByRole("alert")).textContent).toContain("This page no longer exists");
  expect(screen.getByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Back to reading" }));
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Leave without saving?" })).getByRole("button", {
      name: "Leave",
    })
  );
  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
});

test("a page deleted while this tab edits it with nothing unsaved is no page at once", async () => {
  const { server, events } = await connected();

  server.nodes = [notes];
  server.sessions.clear();
  deleted(events);

  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
});

test("a notebook deleted while this tab edits a page of it unsaved stays until the edit is left", async () => {
  const { user, server, events, lab, type } = await connected();
  type(" more");

  lab.notebooks = [];
  server.nodes = [];
  server.sessions.clear();
  act(() => events.last().send("reset", { reason: "notebooks_deleted" }));
  await waitFor(() => expect(events.streams).toHaveLength(2));
  act(() => events.last().hello());

  expect((await screen.findByRole("alert")).textContent).toContain("This page no longer exists");
  expect(screen.getByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Back to reading" }));
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Leave without saving?" })).getByRole("button", {
      name: "Leave",
    })
  );
  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
});
