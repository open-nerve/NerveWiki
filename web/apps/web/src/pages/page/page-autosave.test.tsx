import { EditorView } from "@codemirror/view";
import { act, configure, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, test, vi } from "vitest";

import { autosavePause } from "../../editor/autosave";
import { idleLimit } from "../../editor/idle-exit";
import { editorExtensions } from "../../editor/registry";
import { pageEditor } from "../../test/page-editor";
import { bob, guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// Autosave and idle exit on the page (M5/P5 design 3.4–3.7), in StrictMode
// as the app runs, with the app's editor extensions: the registry's last
// hop for both.

beforeAll(() => configure({ reactStrictMode: true }));
afterAll(() => configure({ reactStrictMode: false }));
afterEach(() => void vi.useRealTimers());

const idleLeft = "Editing ended after 30 minutes without input.";

/** Presses Ctrl+key where the focus is. */
const ctrl = (key: string) => fireEvent.keyDown(document.activeElement ?? document.body, { key, ctrlKey: true });
/** Time goes by for ms, the timers due run. */
const rest = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const puts = (sent: readonly string[]) => sent.filter((line) => line.startsWith("PUT"));

/** Guide shown to Ada over server with the app's editor extensions, on timers the test moves, and edited. */
async function editing(server = pageServer()) {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  renderApp(pagePath(guide.id), server.app, { editorExtensions });
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  return { user, server, ...(await pageEditor()) };
}

test("an edit that rests 2 seconds is saved, once, no key pressed", async () => {
  const { server, type } = await editing();

  type(" one");
  await rest(autosavePause - 500);
  expect(puts(server.sent)).toEqual([]);
  await rest(500);
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Saved."));
  expect(puts(server.sent)).toEqual(['PUT Guide "Guide\\n one" on 1 in session-1']);
  await rest(autosavePause * 3);
  expect(puts(server.sent)).toHaveLength(1);
});

test("while a composition goes on nothing is saved, however long it rests; once it ends, the word is", async () => {
  const { server, type, view } = await editing();
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);

  type("ni");
  await rest(autosavePause * 3);
  expect(puts(server.sent)).toEqual([]);
  composing.mockReturnValue(false);
  view?.contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await rest(100);
  await waitFor(() => expect(puts(server.sent)).toEqual(['PUT Guide "Guide\\nni" on 1 in session-1']));
});

test("Ctrl+S saves at once; the rest that follows sends nothing more", async () => {
  const { server, type } = await editing();

  type(" one");
  ctrl("s");
  await waitFor(() => expect(puts(server.sent)).toHaveLength(1));
  await rest(autosavePause * 2);
  expect(puts(server.sent)).toHaveLength(1);
});

test("with a conflict open, autosave sends nothing and leaves the focus where it is", async () => {
  const { server, type, content } = await editing();
  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 2 });
  type(" mine");
  ctrl("s");
  const region = await screen.findByRole("region", { name: "This page changed while you edited it" });
  await waitFor(() => expect(document.activeElement).toBe(within(region).getByRole("heading")));

  content.focus();
  type(" more");
  await rest(autosavePause * 2);
  expect(puts(server.sent)).toHaveLength(1);
  expect(document.activeElement).toBe(content);
});

test("an edit without input for 30 minutes is saved and left: the reading view says so, the focus on Edit, the session ended; Edit pressed again, it says so no more", async () => {
  const { user, server, type } = await editing();

  type(" one");
  await rest(idleLimit - 1_000);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  await rest(1_000);
  expect(await screen.findByText(idleLeft)).toBe(screen.getByRole("status"));
  const edit = screen.getByRole("button", { name: "Edit" });
  await waitFor(() => expect(document.activeElement).toBe(edit));
  expect(puts(server.sent)).toEqual(['PUT Guide "Guide\\n one" on 1 in session-1']);
  expect(server.sent).toContain("END session-1");

  // Refused, the reading view stays: what it said of the edit before goes.
  server.hold(guide.id, bob);
  await user.click(edit);
  expect(await screen.findByText("Bob is editing this page.")).toBeTruthy();
  expect(screen.queryByText(idleLeft)).toBeNull();
});

test.each([
  ["unsaved", " one", "Guide one"],
  ["with nothing unsaved", "", "Guide"],
])(
  "an edit whose session is lost, %s, is not left for being idle: its banner and its text stay",
  async (_, typed, text) => {
    const { server, type } = await editing();
    type(typed);
    server.takeOver(guide.id);
    act(() => void document.dispatchEvent(new Event("visibilitychange")));
    await screen.findByRole("alert");

    await rest(idleLimit * 2);
    expect(screen.getByRole("textbox", { name: "Page content" }).textContent).toBe(text);
    expect(screen.getByRole("alert").textContent).toContain("You went on editing this page elsewhere");
    expect(screen.queryByText(idleLeft)).toBeNull();
  }
);

test("an edit idle while a composition goes on is left once it ends, with the word", async () => {
  const { server, type, view } = await editing();
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  type("ni");

  await rest(idleLimit + autosavePause);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(puts(server.sent)).toEqual([]);
  composing.mockReturnValue(false);
  view?.contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await rest(100);
  expect(await screen.findByText(idleLeft)).toBeTruthy();
  expect(puts(server.sent)).toEqual(['PUT Guide "Guide\\nni" on 1 in session-1']);
});

test("an edit whose save fails is not left for being idle; 30 minutes later it is tried again, and left", async () => {
  const { server, type } = await editing();
  server.writesDown = true;

  type(" one");
  await rest(idleLimit);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(puts(server.sent).length).toBeGreaterThanOrEqual(2);
  server.writesDown = false;
  await rest(idleLimit);
  expect(await screen.findByText(idleLeft)).toBeTruthy();
  expect(puts(server.sent).at(-1)).toBe('PUT Guide "Guide\\n one" on 1 in session-1');
});
