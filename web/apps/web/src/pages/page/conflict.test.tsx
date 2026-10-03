import { EditorView } from "@codemirror/view";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A save refused because the page changed meanwhile (M4/P6 design 3.8).
// jsdom's platform is not macOS: Mod is Ctrl.

afterEach(() => {
  vi.doUnmock("../../editor/conflict-view");
});

const ctrl = (key: string) => fireEvent.keyDown(document.activeElement ?? document.body, { key, ctrlKey: true });
const conflictTitle = "This page changed while you edited it";

/** The editor once it shows. */
async function editorView() {
  const content = await screen.findByRole("textbox", { name: "Page content" });
  const element = content.closest<HTMLElement>(".cm-editor");
  const view = element === null ? null : EditorView.findFromDOM(element);
  if (view === null) {
    throw new Error("no editor");
  }
  return { content, view };
}

/**
 * Guide edited, "mine" typed and saved after another wrote "theirs" as
 * revision 2: the conflict is shown.
 */
async function inConflict() {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(pagePath(guide.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  const { content, view } = await editorView();
  server.contents.set(guide.id, { content: "Guide\ntheirs\r\n", revision: 2 });
  view.dispatch({ changes: { from: view.state.doc.length, insert: "mine" } });
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Unsaved changes"));
  ctrl("s");
  const region = await screen.findByRole("region", { name: conflictTitle });
  return { user, server, content, view, region };
}

const puts = (sent: readonly string[]) => sent.filter((line) => line.startsWith("PUT"));

test("the conflict shows above the editor, its heading focused, with what the user's text changes", async () => {
  const { region, server } = await inConflict();

  await waitFor(() =>
    expect(document.activeElement).toBe(within(region).getByRole("heading", { name: conflictTitle }))
  );
  const diff = await within(region).findByRole("textbox", { name: "Your text against the page as it is now" });
  expect(diff.textContent).toContain("mine");
  expect(region.textContent).toContain("theirs");
  expect(server.sent).toContain("GET content Guide");
  expect(puts(server.sent)).toEqual(['PUT Guide "Guide\\nmine" on 1 in session-1']);
});

test("meanwhile Ctrl+S, Save, Ctrl+E and Done send nothing and bring the focus back to the conflict", async () => {
  const { user, server, content, region } = await inConflict();
  const heading = within(region).getByRole("heading", { name: conflictTitle });

  content.focus();
  ctrl("s");
  expect(document.activeElement).toBe(heading);
  content.focus();
  ctrl("e");
  await waitFor(() => expect(document.activeElement).toBe(heading));
  await user.click(screen.getByRole("button", { name: "Save" }));
  await user.click(screen.getByRole("button", { name: "Done" }));
  expect(document.activeElement).toBe(heading);
  expect(puts(server.sent)).toHaveLength(1);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
});

test("Keep mine saves the user's text on the revision the conflict read, the focus back in the editor", async () => {
  const { user, server, content, region } = await inConflict();

  await user.click(within(region).getByRole("button", { name: "Keep mine" }));
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Saved."));
  expect(puts(server.sent).at(-1)).toBe('PUT Guide "Guide\\nmine" on 2 in session-1');
  expect(screen.queryByRole("region", { name: conflictTitle })).toBeNull();
  expect(document.activeElement).toBe(content);
});

test("Discard mine edits the page as it is now, its line breaks as written; the next save goes on its revision", async () => {
  const { user, server, content, view, region } = await inConflict();

  await user.click(within(region).getByRole("button", { name: "Discard mine" }));
  expect(view.state.doc.toString()).toBe("Guide\ntheirs\n");
  expect(screen.queryByRole("region", { name: conflictTitle })).toBeNull();
  expect(document.activeElement).toBe(content);
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe(""));

  view.dispatch({ changes: { from: view.state.doc.length, insert: "more" } });
  ctrl("s");
  await waitFor(() => expect(puts(server.sent).at(-1)).toBe('PUT Guide "Guide\\ntheirs\\r\\nmore" on 2 in session-1'));
});

test("differences that cannot load say so; the buttons still work, and Try again shows them", async () => {
  vi.doMock("../../editor/conflict-view", () => {
    throw new Error("offline");
  });
  const { user, region } = await inConflict();

  await within(region).findByText("The differences could not be shown. You can still keep your text or discard it.");
  expect(within(region).getByRole("button", { name: "Keep mine" })).toBeTruthy();
  vi.doUnmock("../../editor/conflict-view");
  await user.click(within(region).getByRole("button", { name: "Try again" }));
  await within(region).findByRole("textbox", { name: "Your text against the page as it is now" });
});
