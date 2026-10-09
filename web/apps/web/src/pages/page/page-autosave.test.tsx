import { EditorView } from "@codemirror/view";
import { act, configure, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, test, vi } from "vitest";

import { autosavePause } from "../../editor/autosave";
import { idleLimit } from "../../editor/idle-exit";
import { editorExtensions, type EditorControls, type EditorExtension } from "../../editor/registry";
import { pageEditor } from "../../test/page-editor";
import { problem } from "../../test/fakes";
import { bob, guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// Autosave and idle exit on the page (M5/P5 design 3.4–3.7), in StrictMode
// as the app runs, with the app's editor extensions: the registry's last
// hop for both.

beforeAll(() => configure({ reactStrictMode: true }));
afterAll(() => configure({ reactStrictMode: false }));
afterEach(() => void vi.useRealTimers());

const idleLeft = "Editing ended after 30 minutes without input.";
const conflictTitle = "This page changed while you edited it";
/** The tab shown again: the edit's session beats. */
const shown = () => act(() => void document.dispatchEvent(new Event("visibilitychange")));

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

test("a conflict autosave runs into leaves the focus in the editor, the status saying so", async () => {
  const { server, type, content } = await editing();
  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 2 });

  type(" mine");
  await rest(autosavePause);
  expect(await screen.findByRole("region", { name: conflictTitle })).toBeTruthy();
  await rest(100);
  expect(document.activeElement).toBe(content);
  expect(screen.getByRole("status").textContent).toBe("This page changed while you edited it: see below.");
});

test("a conflict Ctrl+S runs into takes the focus to its heading, an autosave merged behind it or not; Keep mine's again too", async () => {
  let answer: ((response: Response) => void) | undefined;
  const server = pageServer({
    answers: { "PUT /api/v0/pages/*/content": () => new Promise<Response>((resolve) => (answer = resolve)) },
  });
  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 1 });
  const { user, type } = await editing(server);
  const sent = async () => {
    await waitFor(() => expect(answer).toBeDefined());
    const answering = answer;
    answer = undefined;
    // Autosave's 2 seconds pass while the save is out: its save goes behind it.
    await rest(autosavePause + 100);
    act(() => answering?.(problem(409, "page.revision_mismatch")));
  };

  type(" mine");
  ctrl("s");
  await sent();
  const region = await screen.findByRole("region", { name: conflictTitle });
  await waitFor(() => expect(document.activeElement).toBe(within(region).getByRole("heading")));

  type(" more");
  await user.click(within(region).getByRole("button", { name: "Keep mine" }));
  await sent();
  await waitFor(() =>
    expect(document.activeElement).toBe(
      within(screen.getByRole("region", { name: conflictTitle })).getByRole("heading")
    )
  );
});

test("with a conflict open, the idle exit leaves nothing and moves no focus", async () => {
  const { server, type, content } = await editing();
  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 2 });
  type(" mine");
  ctrl("s");
  const region = await screen.findByRole("region", { name: conflictTitle });
  await waitFor(() => expect(document.activeElement).toBe(within(region).getByRole("heading")));

  content.focus();
  await rest(idleLimit * 2);
  expect(document.activeElement).toBe(content);
  expect(puts(server.sent)).toHaveLength(1);
  expect(screen.queryByText(idleLeft)).toBeNull();
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
  // Edit, where the focus is, is described by why the edit ended.
  expect(document.getElementById(edit.getAttribute("aria-describedby") ?? "")?.textContent).toBe(idleLeft);
  expect(puts(server.sent)).toEqual(['PUT Guide "Guide\\n one" on 1 in session-1']);
  expect(server.sent).toContain("END session-1");

  // Refused, the reading view stays: what it said of the edit before goes.
  server.hold(guide.id, bob);
  await user.click(edit);
  expect(await screen.findByText("Bob is editing this page.")).toBeTruthy();
  expect(screen.queryByText(idleLeft)).toBeNull();
  expect(edit.hasAttribute("aria-describedby")).toBe(false);
});

test("input meanwhile starts the 30 minutes again", async () => {
  const { type } = await editing();

  await rest(idleLimit - 10 * 60_000);
  type(" one");
  await rest(idleLimit - 1_000);
  expect(screen.queryByText(idleLeft)).toBeNull();
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  await rest(1_000);
  expect(await screen.findByText(idleLeft)).toBeTruthy();
});

test("Try again starts the 30 minutes of an unread edit again", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer({
    answers: { "GET /api/v0/pages/*/content": () => Promise.reject(new TypeError("offline")) },
  });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  renderApp(pagePath(guide.id), server.app, { editorExtensions });
  await user.click(await screen.findByRole("button", { name: "Edit" }));

  await rest(idleLimit - 60_000);
  await user.click(await screen.findByRole("button", { name: "Try again" }));
  expect(await screen.findByRole("button", { name: "Try again" })).toBeTruthy();
  await rest(2 * 60_000);
  expect(screen.queryByText(idleLeft)).toBeNull();
  await rest(idleLimit);
  expect(await screen.findByText(idleLeft)).toBeTruthy();
});

test("an edit whose content cannot be read is left all the same after 30 minutes, its session ended", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer({
    answers: { "GET /api/v0/pages/*/content": () => Promise.reject(new TypeError("offline")) },
  });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  renderApp(pagePath(guide.id), server.app, { editorExtensions });
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  expect(await screen.findByRole("button", { name: "Try again" })).toBeTruthy();

  await rest(idleLimit);
  expect(await screen.findByText(idleLeft)).toBeTruthy();
  expect(server.sent).toContain("END session-1");
  // From the page's title, where Edit put it, the focus goes back to Edit, which says why the edit ended.
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "Edit" })));
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

test("an idle exit waiting for a composition stays once it ends, changing nothing (only the user ends one), the word saved; 30 minutes on, it leaves", async () => {
  const { server, type, view } = await editing();
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  type("ni");

  await rest(idleLimit + autosavePause);
  expect(puts(server.sent)).toEqual([]);
  composing.mockReturnValue(false);
  view?.contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await rest(100);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(screen.queryByText(idleLeft)).toBeNull();
  expect(puts(server.sent)).toEqual(['PUT Guide "Guide\\nni" on 1 in session-1']);
  await rest(idleLimit);
  expect(await screen.findByText(idleLeft)).toBeTruthy();
});

test("an idle exit waiting for a composition stays once the user comes back in it; 30 minutes on, it leaves", async () => {
  const { server, type, view } = await editing();
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  type("ni");
  await rest(idleLimit + 1_000);

  type("hao");
  composing.mockReturnValue(false);
  view?.contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await rest(autosavePause);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(screen.queryByText(idleLeft)).toBeNull();
  expect(puts(server.sent)).toEqual(['PUT Guide "Guide\\nnihao" on 1 in session-1']);
  await rest(idleLimit);
  expect(await screen.findByText(idleLeft)).toBeTruthy();
});

test("an idle exit waiting for a composition stays when the session is lost meanwhile: the banner keeps the focus", async () => {
  const { server, type, view } = await editing();
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  type("ni");
  await rest(idleLimit + 1_000);

  server.takeOver(guide.id);
  shown();
  const banner = await screen.findByRole("alert");
  await waitFor(() => expect(document.activeElement).toBe(banner));
  composing.mockReturnValue(false);
  view?.contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await rest(100);
  expect(document.activeElement).toBe(banner);
  expect(screen.queryByText(idleLeft)).toBeNull();
});

test("an idle leave waiting for a composition settles once the editor goes first", async () => {
  let kept: EditorControls | undefined;
  const keeping: EditorExtension = {
    name: "keeping",
    extension: (_, controls) => {
      kept = controls;
      return [];
    },
  };
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  renderApp(pagePath(guide.id), pageServer().app, { editorExtensions: [...editorExtensions, keeping] });
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  const { type } = await pageEditor();
  vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  type("ni");

  const left = kept?.leave("idle");
  await user.click(screen.getByRole("link", { name: "Notes" }));
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Leave without saving?" })).getByRole("button", {
      name: "Leave",
    })
  );
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await expect(left).resolves.toBeUndefined();
});

test("an edit whose save fails is not left for being idle; 30 minutes later it is tried again, and left", async () => {
  const { server, type } = await editing();
  server.writesDown = true;

  type(" one");
  await rest(autosavePause);
  // The user pressed Save, to no avail: the idle exit that stays leaves the focus there.
  const saveButton = screen.getByRole("button", { name: "Save" });
  saveButton.focus();
  const tried = puts(server.sent).length;
  await rest(idleLimit);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(puts(server.sent).length).toBe(tried + 1);
  expect(document.activeElement).toBe(saveButton);
  server.writesDown = false;
  await rest(idleLimit);
  expect(await screen.findByText(idleLeft)).toBeTruthy();
  expect(server.contents.get(guide.id)?.content).toBe("Guide\n one");
});
