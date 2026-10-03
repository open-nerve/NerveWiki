import { EditorView } from "@codemirror/view";
import { act, configure, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, test, vi } from "vitest";

import { json, problem } from "../../test/fakes";
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
        hold ? new Promise<Response>((resolve) => (answer = resolve)) : json({ html: "<p>Guide</p>", revision: 1 }),
    },
  });
  const { user, type } = await editing(server);
  type(" more");
  hold = true;

  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(answer).toBeDefined());
  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  answer?.(json({ html: "<p>Guide, more</p>", revision: 2 }));

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
  server.views.set(guide.id, { html: "<p>Guide edited</p>", revision: 2 });

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
