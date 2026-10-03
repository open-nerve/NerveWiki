import { EditorView } from "@codemirror/view";
import { act, configure, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, onTestFinished, test, vi } from "vitest";

import { problem } from "../../test/fakes";
import { guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A save refused because the page changed meanwhile (M4/P6 design 3.8),
// in StrictMode as the app runs. jsdom's platform is not macOS: Mod is
// Ctrl.

beforeAll(() => configure({ reactStrictMode: true }));
afterAll(() => configure({ reactStrictMode: false }));

afterEach(() => {
  vi.doUnmock("../../editor/conflict-view");
});

const ctrl = (key: string) => fireEvent.keyDown(document.activeElement ?? document.body, { key, ctrlKey: true });
const conflictTitle = "This page changed while you edited it";

/** The editor once it shows: StrictMode makes it again as the first one's effects run, which are run first. */
async function editorView() {
  await screen.findByRole("textbox", { name: "Page content" });
  await act(async () => {});
  const content = screen.getByRole("textbox", { name: "Page content" });
  const element = content.closest<HTMLElement>(".cm-editor");
  const view = element === null ? null : EditorView.findFromDOM(element);
  if (view === null) {
    throw new Error("no editor");
  }
  return { content, view };
}

/**
 * Guide edited, "mine" typed and saved after another wrote theirs as
 * revision 2: the conflict is shown.
 */
async function inConflict(server = pageServer(), theirs = "Guide\ntheirs\r\n") {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  const { content, view } = await editorView();
  server.contents.set(guide.id, { content: theirs, revision: 2 });
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
  // A module whose mock throws fails once: loaded again, as StrictMode does, it is there. Not here.
  configure({ reactStrictMode: false });
  onTestFinished(() => configure({ reactStrictMode: true }));
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

test("Keep mine that fails says why, the focus in the editor", async () => {
  let writes = 0;
  const server = pageServer({
    answers: {
      "PUT /api/v0/pages/*/content": () =>
        ++writes === 1 ? problem(409, "page.revision_mismatch") : problem(500, "internal_error"),
    },
  });
  const { user, content, region } = await inConflict(server);

  await user.click(within(region).getByRole("button", { name: "Keep mine" }));
  await waitFor(() =>
    expect(screen.getByRole("status").textContent).toBe("Something went wrong on the server. Try again.")
  );
  expect(screen.queryByRole("region", { name: conflictTitle })).toBeNull();
  expect(document.activeElement).toBe(content);
});

test("the unchanged stretches are folded; Show unchanged lines, which the keyboard reaches, shows them all", async () => {
  const lines = Array.from({ length: 12 }, (_, i) => `line ${i.toString()}`).join("\n");
  const server = pageServer();
  server.contents.set(guide.id, { content: `Guide\n${lines}\n`, revision: 1 });
  const { user, region } = await inConflict(server, `Guide\ntheirs\n${lines}\n`);
  await within(region).findByRole("textbox", { name: "Your text against the page as it is now" });
  expect(region.querySelector(".cm-collapsedLines")).not.toBeNull();

  const show = within(region).getByRole("button", { name: "Show unchanged lines" });
  expect(show.getAttribute("aria-pressed")).toBe("false");
  await user.click(show);
  expect(show.getAttribute("aria-pressed")).toBe("true");
  await waitFor(() => expect(region.querySelector(".cm-collapsedLines")).toBeNull());
});

test("Stay, while the conflict is shown, takes the focus back to the conflict", async () => {
  const { user, region } = await inConflict();

  await user.click(
    within(screen.getByRole("navigation", { name: "Pages of Plans" })).getByRole("link", { name: "Notes" })
  );
  const ask = await screen.findByRole("alertdialog", { name: "Leave without saving?" });
  await user.click(within(ask).getByRole("button", { name: "Stay" }));
  await waitFor(() =>
    expect(document.activeElement).toBe(within(region).getByRole("heading", { name: conflictTitle }))
  );
});
