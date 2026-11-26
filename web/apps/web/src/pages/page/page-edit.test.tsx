import { EditorView } from "@codemirror/view";
import { act, cleanup, configure, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { afterAll, afterEach, beforeAll, expect, test, vi } from "vitest";

import { routes } from "../../app/routes";
import { autosavePause } from "../../editor/autosave";
import { editorExtensions } from "../../editor/registry";
import { useStore } from "../../stores/context";
import type { RootStore } from "../../stores/root.store";
import { json, problem, workspaceJSON } from "../../test/fakes";
import { guide, pagePath, pageServer } from "../../test/page-server";
import { pageEditor } from "../../test/page-editor";
import { renderApp } from "../../test/render";

// A page's edit mode (M4/P6 design 3.6, 3.7), in StrictMode as the app
// runs: React runs a new component's effects twice there. jsdom's platform
// is not macOS: Mod is Ctrl.

beforeAll(() => configure({ reactStrictMode: true }));
afterAll(() => configure({ reactStrictMode: false }));
afterEach(() => vi.useRealTimers());

/** Presses Ctrl+key where the focus is; tells whether the browser's own action was let be. */
const ctrl = (key: string) => fireEvent.keyDown(document.activeElement ?? document.body, { key, ctrlKey: true });

const status = () => screen.getByRole("status");
/** Closes the tab, as far as the page sees it; tells whether it let the tab close. */
const unload = () => window.dispatchEvent(new Event("beforeunload", { cancelable: true }));
const tree = () => screen.getByRole("navigation", { name: "Pages of Plans" });

/** Guide's page shown to Ada, its server, and the editor once Edit is pressed. */
async function editing(server = pageServer()) {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  return { user, server, ...(await pageEditor()) };
}

test("Edit opens the editor on the content, the focus in it, a session open; the reading view is gone", async () => {
  const { server, content, view } = await editing();

  expect(view.state.doc.toString()).toBe("Guide\n");
  expect(document.activeElement).toBe(content);
  expect(server.sent).toEqual(expect.arrayContaining(["GET content Guide", "OPEN Guide"]));
  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  expect(screen.queryByRole("article")).toBeNull();
});

test("Ctrl+E opens it too; Ctrl+S saves in the session and says so; Ctrl+E saves the rest, ends the session and shows the reading view read again, the focus on Edit", async () => {
  const server = pageServer();
  renderApp(pagePath(guide.id), server.app);
  await screen.findByRole("button", { name: "Edit" });
  expect(ctrl("e")).toBe(false);
  const { type } = await pageEditor();

  type("one");
  await waitFor(() => expect(status().textContent).toBe("Unsaved changes"));
  expect(ctrl("s")).toBe(false);
  await waitFor(() => expect(status().textContent).toBe("Saved."));
  type(" two");
  const viewsRead = server.sent.filter((line) => line === "GET view Guide").length;
  ctrl("e");

  const edit = await screen.findByRole("button", { name: "Edit" });
  // The reading view was read before the editor went: it shows with Edit, no loading in between.
  expect(screen.getByRole("article")).toBeTruthy();
  expect(server.sent.filter((line) => line === "GET view Guide").length).toBeGreaterThan(viewsRead);
  await waitFor(() => expect(document.activeElement).toBe(edit));
  const puts = server.sent.filter((line) => line.startsWith("PUT"));
  expect(puts).toEqual(['PUT Guide "Guide\\none" on 1 in session-1', 'PUT Guide "Guide\\none two" on 2 in session-1']);
  expect(server.sent).toContain("END session-1");
});

test("Done with nothing unsaved sends no content; a save that fails keeps the editor and says why", async () => {
  const server = pageServer({ answers: { "PUT /api/v0/pages/*/content": () => problem(422, "validation_failed") } });
  const { user, type } = await editing(server);

  type("!");
  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(status().textContent).toBe("Some values are not valid."));
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
});

test("Done reads the reading view again before it goes back: the view shows with Edit, no loading between", async () => {
  let hold = false;
  let answer: ((view: Response) => void) | undefined;
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/view": () =>
        hold
          ? new Promise<Response>((resolve) => (answer = resolve))
          : json({ html: "<p>Guide</p>", revision: 1, assets_expire_at: null }),
    },
  });
  const { user, type } = await editing(server);
  type(" more");
  hold = true;

  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(answer).toBeDefined());
  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  answer?.(json({ html: "<p>Guide, more</p>", revision: 2, assets_expire_at: null }));

  await screen.findByRole("button", { name: "Edit" });
  expect(screen.getByRole("article").innerHTML).toBe("<p>Guide, more</p>");
});

test("Done with nothing unsaved goes back at once", async () => {
  const { user, server } = await editing();

  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByRole("button", { name: "Edit" });
  expect(server.sent.some((line) => line.startsWith("PUT"))).toBe(false);
});

test("a reader has no Edit, and Ctrl+E is the browser's", async () => {
  const server = pageServer({ role: "reader" });
  renderApp(pagePath(guide.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  await screen.findByRole("article");

  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  expect(ctrl("e")).toBe(true);
  expect(screen.queryByRole("textbox", { name: "Page content" })).toBeNull();
  expect(server.sent).not.toContain("GET content Guide");
});

test("in the reading view Ctrl+S is the browser's; over a dialog Ctrl+E is the dialog's", async () => {
  const server = pageServer();
  renderApp(pagePath(guide.id), server.app);
  await screen.findByRole("button", { name: "Edit" });

  expect(ctrl("s")).toBe(true);
  ctrl("o");
  await screen.findByRole("dialog", { name: "Go to a page" });
  ctrl("e");
  await new Promise((resolve) => setTimeout(resolve, 50));
  // The dialog hides the page from the accessibility tree: the editor is looked for among all.
  expect(screen.queryByRole("textbox", { name: "Page content", hidden: true })).toBeNull();
  expect(server.sent).not.toContain("GET content Guide");
});

test("editing, over a dialog Ctrl+S and Ctrl+E are the dialog's", async () => {
  const { server, type } = await editing();
  type("draft");

  ctrl("o");
  await screen.findByRole("dialog", { name: "Go to a page" });
  ctrl("s");
  ctrl("e");
  await new Promise((resolve) => setTimeout(resolve, 50));
  expect(server.sent.some((line) => line.startsWith("PUT"))).toBe(false);
  expect(screen.queryByRole("textbox", { name: "Page content", hidden: true })).not.toBeNull();
});

test("unsaved, going to another page asks first: Stay keeps the edit, Leave goes and ends the session", async () => {
  const { user, server, type } = await editing();
  type("draft");
  await waitFor(() => expect(status().textContent).toBe("Unsaved changes"));

  await user.click(within(tree()).getByRole("link", { name: "Notes" }));
  const ask = await screen.findByRole("alertdialog", { name: "Leave without saving?" });
  await user.click(within(ask).getByRole("button", { name: "Stay" }));
  expect(screen.getByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
  const { content, view } = await pageEditor();
  expect(view.state.doc.toString()).toBe("Guide\ndraft");
  await waitFor(() => expect(document.activeElement).toBe(content));

  await user.click(within(tree()).getByRole("link", { name: "Notes" }));
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Leave without saving?" })).getByRole("button", {
      name: "Leave",
    })
  );
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  expect(server.sent).toContain("END session-1");
  expect(server.sent.some((line) => line.startsWith("PUT"))).toBe(false);
});

test("saved, going to another page asks nothing", async () => {
  const { user, type } = await editing();
  type("draft");
  ctrl("s");
  await waitFor(() => expect(status().textContent).toBe("Saved."));

  // Not even for a moment: a dialog that comes and goes is watched for as it is added.
  const asked = vi.fn();
  const watch = new MutationObserver((records) => {
    for (const added of records.flatMap((record) => [...record.addedNodes])) {
      if (
        added instanceof Element &&
        (added.matches('[role="alertdialog"]') || added.querySelector('[role="alertdialog"]'))
      ) {
        asked();
      }
    }
  });
  watch.observe(document.body, { childList: true, subtree: true });
  await user.click(within(tree()).getByRole("link", { name: "Notes" }));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  watch.disconnect();
  expect(asked).not.toHaveBeenCalled();
});

test("unsaved, a change of the query or the hash alone asks nothing: the edit stays", async () => {
  const user = userEvent.setup();
  const { router } = renderApp(pagePath(guide.id), pageServer().app);
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  const { type } = await pageEditor();
  type(" more");
  await waitFor(() => expect(status().textContent).toBe("Unsaved changes"));

  await act(() => router.navigate(`${pagePath(guide.id)}?tab=2#part`));

  expect([router.state.location.search, router.state.location.hash]).toEqual(["?tab=2", "#part"]);
  expect(screen.queryByRole("alertdialog")).toBeNull();
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
});

test("closing the tab is stopped only while the edit is unsaved", async () => {
  const { type } = await editing();

  expect(unload()).toBe(true);
  type("draft");
  await waitFor(() => expect(unload()).toBe(false));
  ctrl("s");
  await waitFor(() => expect(status().textContent).toBe("Saved."));
  expect(unload()).toBe(true);
});

test("Ctrl+S while the input method composes waits for the composition's end, then saves the text it ends on", async () => {
  const { server, content, type } = await editing();
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  type("ni");

  expect(ctrl("s")).toBe(false);
  await new Promise((resolve) => setTimeout(resolve, 100));
  expect(server.sent.some((line) => line.startsWith("PUT"))).toBe(false);
  composing.mockReturnValue(false);
  type("你好");
  content.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await waitFor(() => expect(server.sent).toContain('PUT Guide "Guide\\nni你好" on 1 in session-1'));
});

/** A write's answer that waits until answer is called with the answer to give. */
function held() {
  const write: { answer?: (response: Response) => void } = {};
  const route = () => new Promise<Response>((resolve) => (write.answer = resolve));
  return { write, route };
}

const savedGuide = () =>
  json({
    ...guide,
    ancestors: [],
    revision: 2,
    byte_size: 6,
    content_updated_at: guide.updated_at,
    content_updated_by: "",
  });

test("while Done saves, the content is held as it is; a save that fails gives it back, the focus in the editor", async () => {
  const { write, route } = held();
  const { user, view, content, type } = await editing(
    pageServer({ answers: { "PUT /api/v0/pages/*/content": route } })
  );
  type("one");

  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(write.answer).toBeDefined());
  expect(view.state.readOnly).toBe(true);
  expect(content.getAttribute("contenteditable")).toBe("false");
  write.answer?.(problem(500, "internal_error"));
  await waitFor(() => expect(status().textContent).not.toBe("Saving…"));
  expect(view.state.readOnly).toBe(false);
  expect(content.getAttribute("contenteditable")).toBe("true");
  await waitFor(() => expect(document.activeElement).toBe(content));
});

test("Save and Done pressed while the input method composes wait for the composition's end", async () => {
  const { user, server, content, type } = await editing();
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  type("ni");

  await user.click(screen.getByRole("button", { name: "Save" }));
  await user.click(screen.getByRole("button", { name: "Done" }));
  await new Promise((resolve) => setTimeout(resolve, 100));
  expect(server.sent.some((line) => line.startsWith("PUT"))).toBe(false);
  composing.mockReturnValue(false);
  type("你好");
  content.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await screen.findByRole("button", { name: "Edit" });
  expect(server.sent.filter((line) => line.startsWith("PUT"))).toEqual([
    'PUT Guide "Guide\\nni你好" on 1 in session-1',
  ]);
});

test("a save that goes through while leaving is asked about lets the move go on", async () => {
  const { write, route } = held();
  const { user, type } = await editing(pageServer({ answers: { "PUT /api/v0/pages/*/content": route } }));
  type("draft");
  ctrl("s");
  await waitFor(() => expect(write.answer).toBeDefined());

  await user.click(within(tree()).getByRole("link", { name: "Notes" }));
  await screen.findByRole("alertdialog", { name: "Leave without saving?" });
  write.answer?.(savedGuide());
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

test("Ctrl+E held down leaves once", async () => {
  const { server, type } = await editing();
  type("!");

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true, repeat: true });
  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true, repeat: true });
  const edit = await screen.findByRole("button", { name: "Edit" });
  expect(server.sent.filter((line) => line.startsWith("PUT"))).toHaveLength(1);
  await waitFor(() => expect(document.activeElement).toBe(edit));
  expect(fireEvent.keyDown(edit, { key: "e", ctrlKey: true, repeat: true })).toBe(false);
  await new Promise((resolve) => setTimeout(resolve, 50));
  expect(screen.queryByRole("textbox", { name: "Page content" })).toBeNull();
  expect(server.sent.filter((line) => line === "GET content Guide")).toHaveLength(1);
});

test("Edit takes the focus to the page's title until the editor takes it: there it is when the content cannot be read, and Done goes back to reading", async () => {
  const user = userEvent.setup();
  const server = pageServer({
    answers: { "GET /api/v0/pages/*/content": () => Promise.reject(new TypeError("offline")) },
  });
  renderApp(pagePath(guide.id), server.app);

  await user.click(await screen.findByRole("button", { name: "Edit" }));
  await screen.findByRole("button", { name: "Try again" });
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Guide" }));

  // Done goes back to the reading view, the session ended.
  await user.click(screen.getByRole("button", { name: "Done" }));
  expect(await screen.findByRole("button", { name: "Edit" })).toBeTruthy();
  expect(screen.getByRole("article").textContent).toBe("Guide");
  await waitFor(() => expect(server.sent).toContain("END session-1"));
});

test("an edit whose content cannot be read, left with Ctrl+E from the page's title, gives the focus back to Edit", async () => {
  const user = userEvent.setup();
  const server = pageServer({
    answers: { "GET /api/v0/pages/*/content": () => Promise.reject(new TypeError("offline")) },
  });
  renderApp(pagePath(guide.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  await screen.findByRole("button", { name: "Try again" });
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Guide" }));

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  const edit = await screen.findByRole("button", { name: "Edit" });
  await waitFor(() => expect(document.activeElement).toBe(edit));
});

test.each([
  {
    what: "a busy server",
    answers: { "PUT /api/v0/pages/*/content": () => problem(503, "server_busy", {}, { "Retry-After": "30" }) },
    says: "The server is busy: the page is saved again in a moment.",
  },
  {
    what: "a network too slow",
    answers: { "PUT /api/v0/pages/*/content": () => problem(400, "bad_request") },
    says: "The network was too slow to send the page in time. Save again.",
  },
  {
    what: "a page too large",
    answers: {
      "PUT /api/v0/pages/*/content": () =>
        problem(422, "validation_failed", { errors: [{ field: "content", code: "too_long", message: "" }] }),
    },
    says: "The page is over 5 MiB: shorten it to save it.",
  },
  {
    what: "a NUL character",
    answers: {
      "PUT /api/v0/pages/*/content": () =>
        problem(422, "validation_failed", { errors: [{ field: "content", code: "invalid_format", message: "" }] }),
    },
    says: "The page has a NUL character, which cannot be saved: take it out to save it.",
  },
])("the status says what a save came to: $what", async ({ answers, says }) => {
  const { type } = await editing(pageServer({ answers }));
  type("!");

  ctrl("s");
  await waitFor(() => expect(status().textContent).toBe(says));
});

test("a page saved and left for another shows its reading view read again when it is back", async () => {
  const { user, server, type } = await editing();
  type("edited");
  ctrl("s");
  await waitFor(() => expect(status().textContent).toBe("Saved."));
  server.views.set(guide.id, { html: "<p>Guide edited</p>", revision: 2, assets_expire_at: null });

  await user.click(within(tree()).getByRole("link", { name: "Notes" }));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await user.click(within(tree()).getByRole("link", { name: "Guide" }));
  expect((await screen.findByRole("article")).textContent).toBe("Guide edited");
});

test("Done pressed again while the edit is left does nothing more: a save that fails keeps what is typed after it", async () => {
  const { write, route } = held();
  let writes = 0;
  const server = pageServer({
    answers: { "PUT /api/v0/pages/*/content": () => (++writes === 1 ? route() : savedGuide()) },
  });
  const { user, content, type } = await editing(server);
  type("one");

  await user.click(screen.getByRole("button", { name: "Done" }));
  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(write.answer).toBeDefined());
  write.answer?.(problem(500, "internal_error"));
  await waitFor(() => expect(document.activeElement).toBe(content));
  type(" two");
  await new Promise((resolve) => setTimeout(resolve, 50));
  expect(writes).toBe(1);
  expect(status().textContent).toBe("Something went wrong on the server. Try again.");
  expect(screen.getByRole("textbox", { name: "Page content" })).toBe(content);
});

test("a change made while the edit is left keeps it: the edit leaves only with nothing unsaved", async () => {
  const { write, route } = held();
  const { user, view, type } = await editing(pageServer({ answers: { "PUT /api/v0/pages/*/content": route } }));
  type("one");

  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(write.answer).toBeDefined());
  // An extension changes the content as code does, read-only or not.
  type(" two");
  write.answer?.(savedGuide());
  await waitFor(() => expect(status().textContent).toBe("Unsaved changes"));
  expect(view.state.readOnly).toBe(false);
  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
});

test("a page left while its edit is being left is not read again for it", async () => {
  const { write, route } = held();
  const { user, server, type } = await editing(pageServer({ answers: { "PUT /api/v0/pages/*/content": route } }));
  type("one");
  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(write.answer).toBeDefined());
  const viewsRead = server.sent.filter((line) => line === "GET view Guide").length;

  await user.click(within(tree()).getByRole("link", { name: "Notes" }));
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Leave without saving?" })).getByRole("button", {
      name: "Leave",
    })
  );
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  write.answer?.(savedGuide());
  await new Promise((resolve) => setTimeout(resolve, 50));
  expect(server.sent.filter((line) => line === "GET view Guide")).toHaveLength(viewsRead);
});

/** Guide's page shown with the app's routes, the notebooks read again, as another tab's change of the role is read. */
function renderReread(server: ReturnType<typeof pageServer>) {
  let store: RootStore | undefined;
  function CaptureStore() {
    const current = useStore();
    useEffect(() => {
      store = current;
    }, [current]);
    return <Outlet />;
  }
  renderApp(pagePath(guide.id), server.app, { routes: [{ Component: CaptureStore, children: routes }] });
  return () =>
    act(async () => {
      await store?.notebooksOf(workspaceJSON)?.load();
    });
}

// M12/P2/S1, 18.6: back from the edit, the focus goes to Edit, or, with no Edit, to the page's title; handled as the
// reading view shows, so that an Edit shown later does not take it from the reader.
test("Done, the account made a reader meanwhile, gives the focus to the page's title; made a writer again, Edit does not take it", async () => {
  const user = userEvent.setup();
  const server = pageServer({ role: "editor" });
  const reread = renderReread(server);
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  await pageEditor();

  server.notebook = { ...server.notebook, role: "reader" };
  await reread();
  await user.click(screen.getByRole("button", { name: "Done" }));
  const title = await screen.findByRole("heading", { level: 1, name: "Guide" });
  await screen.findByRole("article");
  await waitFor(() => expect(document.activeElement).toBe(title));
  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();

  server.notebook = { ...server.notebook, role: "editor" };
  await reread();
  await screen.findByRole("button", { name: "Edit" });
  await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
  expect(document.activeElement).toBe(title);
});

// The edit's status and buttons in the page's head (M13/P4/S1, decisions 866, 867, 894): the status in the box of who
// edited the page last, Save before the right column's toggle, Done after it; a note under the title.

/** head is the page's head's right group, its box first: the right column's toggle's. */
function head(): { group: HTMLElement; box: Element | null; toggle: HTMLElement } {
  const toggle = screen.getByRole("button", { name: "About this page" });
  const group = toggle.parentElement;
  if (group === null) {
    throw new Error("the toggle is in no group");
  }
  return { group, box: group.firstElementChild, toggle };
}

/** before tells whether a comes before b in the document. */
const before = (a: Node, b: Node) => (a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING) !== 0;

test("editing, the page's head has the edit's status in the box of who edited the page, Save before the column's toggle, Done after it; reading, that box says who edited it", async () => {
  const server = pageServer();
  renderApp(pagePath(guide.id), server.app);
  await screen.findByText(
    (_, element) => element?.tagName === "P" && (element.textContent ?? "").startsWith("Edited by ")
  );
  const reading = head();
  expect(reading.box?.textContent).toMatch(/^Edited by Ada /);
  expect(reading.box?.className).toContain("text-[12.5px]");
  expect(reading.box?.className).toContain("text-faint");

  await userEvent.setup().click(screen.getByRole("button", { name: "Edit" }));
  const { type } = await pageEditor();
  const { group, box, toggle } = head();
  const output = status();
  expect(box?.contains(output)).toBe(true);
  expect(box).toBe(reading.box);
  const save = within(group).getByRole("button", { name: "Save" });
  const done = within(group).getByRole("button", { name: "Done" });
  expect([before(output, save), before(save, toggle), before(toggle, done)]).toEqual([true, true, true]);
  expect(output.className).not.toContain("text-destructive");

  type("!");
  await waitFor(() => expect(output.textContent).toBe("Unsaved changes"));
  expect(output.hasAttribute("data-saved")).toBe(false);
  ctrl("s");
  await waitFor(() => expect(output.textContent).toBe("Saved."));
  expect(output.getAttribute("data-saved")).toBe("true");
});

test("a save that fails says why in the head's box, in the destructive colour", async () => {
  const server = pageServer({ answers: { "PUT /api/v0/pages/*/content": () => problem(422, "validation_failed") } });
  const { type } = await editing(server);

  type("!");
  ctrl("s");
  await waitFor(() => expect(status().textContent).toBe("Some values are not valid."));
  expect(head().box?.contains(status())).toBe(true);
  expect(status().className).toContain("text-destructive");
});

test("editing, a note under the title says others only read the page meanwhile; in Chinese too", async () => {
  const { server } = await editing();
  const title = screen.getByRole("heading", { level: 1, name: "Guide" });
  const note = screen.getByText("You are editing; others can only read this page meanwhile.");
  expect(note.tagName).toBe("P");
  expect(before(title, note)).toBe(true);
  expect(note.closest("output, [role=status], [role=alert]")).toBeNull();
  cleanup();

  const chinese = pageServer();
  chinese.app.preferences.setLocale("zh-CN");
  renderApp(pagePath(guide.id), chinese.app);
  await userEvent.setup().click(await screen.findByRole("button", { name: "编辑" }));
  expect(await screen.findByText("你正在编辑，别人此时只能阅读这一页")).toBeTruthy();
  expect([server.sent, chinese.sent].map((sent) => sent.includes("OPEN Guide"))).toEqual([true, true]);
});

test("an edit lost has no note under the title: its banner says why", async () => {
  const { server, type } = await editing();
  expect(screen.getByText("You are editing; others can only read this page meanwhile.")).toBeTruthy();
  type(" more");
  server.takeOver(guide.id);
  // The page shown again beats the session, which answers that it was taken over.
  act(() => void document.dispatchEvent(new Event("visibilitychange")));

  await screen.findByRole("alert");
  expect(screen.queryByText("You are editing; others can only read this page meanwhile.")).toBeNull();
});

test("Done in the page's head, its leave's save meeting a conflict: the conflict's heading takes the focus", async () => {
  const { user, server, type } = await editing();
  type("mine");
  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 2 });

  await user.click(within(head().group).getByRole("button", { name: "Done" }));
  const region = await screen.findByRole("region", { name: "This page changed while you edited it" });
  await waitFor(() =>
    expect(document.activeElement).toBe(
      within(region).getByRole("heading", { name: "This page changed while you edited it" })
    )
  );
});

// Save shown only when it is needed (M13/P4/S3, decisions 877, 885, 894, 895, 896): with something unsaved, or as long
// as the focus stays in the head's right group once it showed; otherwise invisible, its room kept. jsdom reads no
// Tailwind: the cells read its class, and find it in either case.

/** Guide edited with the app's editor extensions, autosave's among them, on timers the test moves. */
async function autosaving(server = pageServer()) {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  renderApp(pagePath(guide.id), server.app, { editorExtensions });
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  return { user, server, ...(await pageEditor()) };
}

/** autosaved has autosave's pause go by, what was typed saved. */
async function autosaved() {
  await act(() => vi.advanceTimersByTimeAsync(autosavePause));
  await waitFor(() => expect(status().textContent).toBe("Saved."));
}

/** The head's Save, Done and the right column's toggle. */
const save = () => within(head().group).getByRole("button", { name: "Save" });
const done = () => within(head().group).getByRole("button", { name: "Done" });
const toggle = () => within(head().group).getByRole("button", { name: "About this page" });

/** hidden tells whether Save is invisible: there, its room kept, not to be focused. */
const hidden = (button: HTMLElement) => button.classList.contains("invisible");

test("Save is always there, invisible while nothing is unsaved: typed in, it shows; a save that fails keeps it, which tries again", async () => {
  // The writes the server refuses, which it does not count as sent.
  let puts = 0;
  const server = pageServer({
    answers: {
      "PUT /api/v0/pages/*/content": () => {
        puts++;
        return problem(500, "internal_error");
      },
    },
  });
  const { user, type } = await editing(server);
  expect(hidden(save())).toBe(true);

  type("!");
  await waitFor(() => expect(status().textContent).toBe("Unsaved changes"));
  expect(hidden(save())).toBe(false);
  await user.click(save());
  await waitFor(() => expect(status().className).toContain("text-destructive"));
  expect(puts).toBe(1);
  expect(hidden(save())).toBe(false);
  await user.click(save());
  await waitFor(() => expect(puts).toBe(2));
});

test("Save holding the focus as autosave saves stays, pressed then doing nothing; the focus gone, it hides (decision 885)", async () => {
  const { user, server, type, content } = await autosaving();
  type("!");
  await waitFor(() => expect(hidden(save())).toBe(false));
  act(() => save().focus());

  await autosaved();
  expect(hidden(save())).toBe(false);
  expect(document.activeElement).toBe(save());
  const puts = server.sent.filter((line) => line.startsWith("PUT")).length;
  await user.click(save());
  await act(() => vi.advanceTimersByTimeAsync(100));
  expect(server.sent.filter((line) => line.startsWith("PUT"))).toHaveLength(puts);

  act(() => content.focus());
  expect(hidden(save())).toBe(true);
});

test("typed in, the focus on Done as autosave saves: Save stays, the keys back from Done finding it (decision 895, review A's R1-M2)", async () => {
  const { user, type } = await autosaving();
  type("!");
  act(() => done().focus());

  await autosaved();
  expect(hidden(save())).toBe(false);
  // Through the toggle, which jsdom, reading no CSS, has in the order too.
  await user.tab({ shift: true });
  await user.tab({ shift: true });
  expect(document.activeElement).toBe(save());
  expect(hidden(save())).toBe(false);
});

test("nothing unsaved, the focus going into the head's group shows no Save: as the edit opens, and once autosave saved (review A's R2-M1)", async () => {
  const { type, content } = await autosaving();
  act(() => done().focus());
  expect(hidden(save())).toBe(true);
  act(() => toggle().focus());
  expect(hidden(save())).toBe(true);

  act(() => content.focus());
  type("!");
  await autosaved();
  expect(hidden(save())).toBe(true);
  act(() => done().focus());
  expect(hidden(save())).toBe(true);
  act(() => toggle().focus());
  expect(hidden(save())).toBe(true);
});

test("something unsaved coming as the focus is in the group, as an upload's link goes in while Done waits: Save shows, and stays once saved until the focus goes", async () => {
  const { view, content } = await autosaving();
  act(() => done().focus());
  expect(hidden(save())).toBe(true);

  act(() => view.dispatch({ changes: { from: view.state.doc.length, insert: "![a](/a.png)" } }));
  await waitFor(() => expect(hidden(save())).toBe(false));
  expect(document.activeElement).toBe(done());
  await autosaved();
  expect(hidden(save())).toBe(false);
  act(() => content.focus());
  expect(hidden(save())).toBe(true);
});

test("the window losing the focus as Save holds it keeps it: the focus leaves for nothing, the element still active (section 1 (g))", async () => {
  const { type } = await autosaving();
  type("!");
  await waitFor(() => expect(hidden(save())).toBe(false));
  act(() => save().focus());
  await autosaved();

  fireEvent.focusOut(save(), { relatedTarget: null });
  expect(document.activeElement).toBe(save());
  expect(hidden(save())).toBe(false);
});
