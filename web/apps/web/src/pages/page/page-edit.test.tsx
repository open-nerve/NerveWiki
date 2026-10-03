import { EditorView } from "@codemirror/view";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { problem } from "../../test/fakes";
import { guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A page's edit mode (M4/P6 design 3.6, 3.7). jsdom's platform is not
// macOS: Mod is Ctrl.

afterEach(() => vi.useRealTimers());

/** Presses Ctrl+key where the focus is; tells whether the browser's own action was let be. */
const ctrl = (key: string) => fireEvent.keyDown(document.activeElement ?? document.body, { key, ctrlKey: true });

const status = () => screen.getByRole("status");
/** Closes the tab, as far as the page sees it; tells whether it let the tab close. */
const unload = () => window.dispatchEvent(new Event("beforeunload", { cancelable: true }));
const tree = () => screen.getByRole("navigation", { name: "Pages of Plans" });

/** The editor once it shows, and a way to type at its end. */
async function editor() {
  const content = await screen.findByRole("textbox", { name: "Page content" });
  const element = content.closest<HTMLElement>(".cm-editor");
  const view = element === null ? null : EditorView.findFromDOM(element);
  if (view === null) {
    throw new Error("no editor");
  }
  const type = (text: string) =>
    view.dispatch({ changes: { from: view.state.doc.length, insert: text }, userEvent: "input.type" });
  return { content, view, type };
}

/** Guide's page shown to Ada, its server, and the editor once Edit is pressed. */
async function editing(server = pageServer()) {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  return { user, server, ...(await editor()) };
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
  const { type } = await editor();

  type("one");
  await waitFor(() => expect(status().textContent).toBe("Unsaved changes"));
  expect(ctrl("s")).toBe(false);
  await waitFor(() => expect(status().textContent).toBe("Saved."));
  type(" two");
  const viewsRead = server.sent.filter((line) => line === "GET view Guide").length;
  ctrl("e");

  const edit = await screen.findByRole("button", { name: "Edit" });
  await waitFor(() => expect(document.activeElement).toBe(edit));
  const puts = server.sent.filter((line) => line.startsWith("PUT"));
  expect(puts).toEqual(['PUT Guide "Guide\\none" on 1 in session-1', 'PUT Guide "Guide\\none two" on 2 in session-1']);
  expect(server.sent).toContain("END session-1");
  expect(server.sent.filter((line) => line === "GET view Guide").length).toBeGreaterThan(viewsRead);
  expect(screen.getByRole("article")).toBeTruthy();
});

test("Done with nothing unsaved sends no content; a save that fails keeps the editor and says why", async () => {
  const server = pageServer({ answers: { "PUT /api/v0/pages/*/content": () => problem(422, "validation_failed") } });
  const { user, type } = await editing(server);

  type("!");
  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(status().textContent).toBe("Some values are not valid."));
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
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
  expect((await editor()).view.state.doc.toString()).toBe("Guide\ndraft");

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

  await user.click(within(tree()).getByRole("link", { name: "Notes" }));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  expect(screen.queryByRole("alertdialog")).toBeNull();
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
