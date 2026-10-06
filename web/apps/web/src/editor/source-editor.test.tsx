import { EditorView, keymap } from "@codemirror/view";
import { act as reactAct, render, screen } from "@testing-library/react";
import { createRef, StrictMode, Suspense, type ReactNode } from "react";
import { afterEach, expect, test, vi } from "vitest";

import { I18nProvider } from "../i18n/i18n";
import type { Locale } from "../i18n/locale";
import { lockReadOnly } from "./lock-read-only";
import {
  EditorClosed,
  EditorExtensions,
  type EditorContext,
  type EditorControls,
  type EditorExtension,
} from "./registry";
import { SourceEditor, type SourceEditorHandle } from "./source-editor";

afterEach(() => vi.useRealTimers());

const context: EditorContext = { workspace: "lab", notebook: "n1", page: "p1", role: "editor" };

/** The edit's session as the controls tell it, which the test loses; following counts its listeners. */
function sessionState() {
  let lost = false;
  const listeners = new Set<() => void>();
  return {
    session: () => ({ lost }),
    onSessionChange: (listener: () => void) => {
      listeners.add(listener);
      return () => void listeners.delete(listener);
    },
    lose: () => {
      lost = true;
      for (const listener of listeners) {
        listener();
      }
    },
    following: () => listeners.size,
  };
}

/** The editor on content, its handle, its session, and what it told onChange, save and leave. */
function editor(content: string, extensions: readonly EditorExtension[] = [], wrap = (node: ReactNode) => node) {
  const handle = createRef<SourceEditorHandle>();
  const onChange = vi.fn();
  const save = vi.fn(() => Promise.resolve());
  const leave = vi.fn((_reason: "idle") => Promise.resolve());
  const session = sessionState();
  const tree = (locale: Locale) =>
    wrap(
      <I18nProvider locale={locale}>
        <EditorExtensions value={extensions}>
          <SourceEditor
            ref={handle}
            content={content}
            context={context}
            controls={{
              save,
              saving: () => false,
              session: session.session,
              onSessionChange: session.onSessionChange,
              leave,
            }}
            onChange={onChange}
          />
        </EditorExtensions>
      </I18nProvider>
    );
  const { container, rerender, unmount } = render(tree("en"));
  const view = () => {
    const element = container.querySelector<HTMLElement>(".cm-editor");
    const found = element === null ? null : EditorView.findFromDOM(element);
    if (found === null) {
      throw new Error("no editor");
    }
    return found;
  };
  const type = (text: string, at = view().state.doc.length) => view().dispatch({ changes: { from: at, insert: text } });
  return {
    container,
    handle: () => handle.current as SourceEditorHandle,
    view,
    type,
    onChange,
    save,
    leave,
    session,
    unmount,
    speak: (locale: Locale) => rerender(tree(locale)),
  };
}

/** An extension that keeps the controls it is given in controls. */
function keeping() {
  const kept: { controls?: EditorControls } = {};
  const extension: EditorExtension = {
    name: "keeping",
    extension: (_, controls) => {
      kept.controls = controls;
      return [];
    },
  };
  return { kept, extension };
}

const editable = (view: EditorView) =>
  !view.state.readOnly && view.contentDOM.getAttribute("contenteditable") === "true";

test("under StrictMode one editor is in the page; its content is labelled", () => {
  const { container, view } = editor("# Title\n", [], (node) => <StrictMode>{node}</StrictMode>);
  expect(container.querySelectorAll(".cm-editor")).toHaveLength(1);
  expect(view().contentDOM.getAttribute("aria-label")).toBe("Page content");
});

test("gives back the content with its line breaks and byte order mark; counts the changes", () => {
  const bom = String.fromCodePoint(0xfeff);
  const { handle, type, onChange } = editor(`${bom}a\r\nb\r\n`);

  type("!", 1);
  type("?", 4);
  expect(handle().text()).toBe(`${bom}a!\r\nb?\r\n`);
  expect(onChange.mock.calls).toEqual([[1], [2]]);
  expect(handle().version()).toBe(2);
});

test("a content loaded is a new state: undo does not reach the one before", () => {
  const { handle, view, type } = editor("old\n");
  type("edit");

  handle().load("new\r\n");
  expect(handle().text()).toBe("new\r\n");
  expect(view().state.doc.toString()).toBe("new\n");
  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "z", ctrlKey: true, bubbles: true }));
  expect(handle().text()).toBe("new\r\n");
});

test("what waits on a composition runs at once when there is none", () => {
  const { handle } = editor("text");
  const act = vi.fn();

  handle().whenComposed(act);
  expect(act).toHaveBeenCalledOnce();
});

test("the registered extensions are in; their controls reach the latest save", async () => {
  const saving: EditorExtension = {
    name: "save on change",
    extension: (_, controls) => EditorView.updateListener.of((update) => void (update.docChanged && controls.save())),
  };
  const { type, save } = editor("text", [saving]);

  type("!");
  await Promise.resolve();
  expect(save).toHaveBeenCalledOnce();
});

test("an extension that loads what builds it is waited for, the editor suspended, then composed with the others", async () => {
  let go!: () => void;
  const saving: EditorExtension = {
    name: "save on change, loaded",
    load: () =>
      new Promise((resolve) => {
        go = () =>
          resolve((_, controls) =>
            EditorView.updateListener.of((update) => void (update.docChanged && controls.save()))
          );
      }),
  };
  const save = vi.fn(() => Promise.resolve());
  const session = sessionState();
  const { container } = await reactAct(async () =>
    render(
      <I18nProvider locale="en">
        <EditorExtensions value={[lockReadOnly, saving]}>
          <Suspense fallback={<p>loading</p>}>
            <SourceEditor
              content="text"
              context={context}
              controls={{ save, saving: () => false, ...session, leave: () => Promise.resolve() }}
              onChange={() => undefined}
            />
          </Suspense>
        </EditorExtensions>
      </I18nProvider>
    )
  );
  expect(screen.getByText("loading")).toBeTruthy();
  expect(container.querySelector(".cm-editor")).toBeNull();

  await reactAct(async () => go());
  const element = container.querySelector<HTMLElement>(".cm-editor");
  const view = element === null ? null : EditorView.findFromDOM(element);
  view?.dispatch({ changes: { from: 4, insert: "!" } });
  await Promise.resolve();
  expect(save).toHaveBeenCalledOnce();
});

test("Mod+B is the editor's", () => {
  const { view, handle } = editor("word");
  view().dispatch({ selection: { anchor: 0, head: 4 } });

  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "b", ctrlKey: true, bubbles: true }));
  expect(handle().text()).toBe("**word**");
});

test("the registered extensions come after the editor's own: a key both bind is the editor's", () => {
  const run = vi.fn(() => true);
  const { view, handle } = editor("word", [{ name: "bold", extension: () => keymap.of([{ key: "Mod-b", run }]) }]);
  view().dispatch({ selection: { anchor: 0, head: 4 } });

  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "b", ctrlKey: true, bubbles: true }));
  expect(handle().text()).toBe("**word**");
  expect(run).not.toHaveBeenCalled();
});

test("an extension may set the content read-only as it is built; a content loaded keeps it; the edit's hold is apart", () => {
  const errors = vi.spyOn(console, "error");
  let built = 0;
  const locking: EditorExtension = {
    name: "locking",
    extension: (_, controls) => {
      if (built++ === 0) {
        controls.setReadOnly(true);
      }
      return [];
    },
  };
  const { kept, extension } = keeping();
  const { view, handle } = editor("text", [locking, extension]);

  expect(errors).not.toHaveBeenCalled();
  expect(editable(view())).toBe(false);
  handle().load("new");
  expect(editable(view())).toBe(false);
  kept.controls?.setReadOnly(false);
  expect(editable(view())).toBe(true);

  handle().hold(true);
  expect(editable(view())).toBe(false);
  // Read-only, the content stays focusable: the focus in it stays.
  expect(view().contentDOM.getAttribute("tabindex")).toBe("-1");
  kept.controls?.setReadOnly(true);
  kept.controls?.setReadOnly(false);
  expect(editable(view())).toBe(false);
  handle().hold(false);
  expect(editable(view())).toBe(true);
  expect(view().contentDOM.hasAttribute("tabindex")).toBe(false);
});

test("the edit's session reaches the extensions; what one follows of it is let go as a content loads, and as the editor goes", () => {
  const seen: boolean[] = [];
  const following: EditorExtension = {
    name: "following",
    extension: (_, controls) => {
      controls.onSessionChange(() => seen.push(controls.session().lost));
      return [];
    },
  };
  const { handle, session, unmount } = editor("text", [following]);

  expect(session.following()).toBe(1);
  handle().load("new");
  expect(session.following()).toBe(1);
  session.lose();
  expect(seen).toEqual([true]);
  unmount();
  expect(session.following()).toBe(0);
});

test("an extension hears of each change of the content, not of a content loaded; as a content loads and as the editor goes, it hears no more and is told", () => {
  const heard: string[] = [];
  let built = 0;
  const listening: EditorExtension = {
    name: "listening",
    extension: (_, controls) => {
      const state = ++built;
      controls.onChange(() => heard.push(`change ${state.toString()}`));
      controls.onClose(() => heard.push(`close ${state.toString()}`));
      return [];
    },
  };
  const { handle, view, type, unmount } = editor("text", [listening]);

  type("!");
  type("?");
  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "z", ctrlKey: true, bubbles: true }));
  expect(handle().text()).toBe("text");
  handle().load("new");
  type("!");
  unmount();
  expect(heard).toEqual(["change 1", "change 1", "change 1", "close 1", "change 2", "close 2"]);
});

test("under StrictMode the editor made first, and gone at once, tells its extensions it closed", () => {
  const built: number[] = [];
  const closed: number[] = [];
  const listening: EditorExtension = {
    name: "listening",
    extension: (_, controls) => {
      const state = built.push(built.length + 1);
      controls.onClose(() => closed.push(state));
      return [];
    },
  };
  const { unmount } = editor("text", [listening], (node) => <StrictMode>{node}</StrictMode>);

  expect(built).toEqual([1, 2]);
  expect(closed).toEqual([1]);
  unmount();
  expect(closed).toEqual([1, 2]);
});

test("an extension's leave reaches the edit's, with its reason", async () => {
  const { kept, extension } = keeping();
  const { leave } = editor("text", [extension]);

  await kept.controls?.leave("idle");
  expect(leave.mock.calls).toEqual([["idle"]]);
});

test("lockReadOnly sets the content read-only once the session is lost, a content loaded too; the text stays", () => {
  const { view, handle, session } = editor("text", [lockReadOnly]);

  expect(editable(view())).toBe(true);
  session.lose();
  expect(editable(view())).toBe(false);
  expect(handle().text()).toBe("text");
  handle().load("new");
  expect(editable(view())).toBe(false);
});

test("an extension's save waits for the composition's end, as Mod+S does", async () => {
  const { kept, extension } = keeping();
  const { view, save } = editor("text", [extension]);
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);

  const saved = kept.controls?.save();
  await new Promise((resolve) => setTimeout(resolve, 100));
  expect(save).not.toHaveBeenCalled();
  composing.mockReturnValue(false);
  view().contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await saved;
  expect(save).toHaveBeenCalledOnce();
});

test("an extension's save that waits for a composition rejects with EditorClosed as the editor goes; nothing is saved (nt-3)", async () => {
  const { kept, extension } = keeping();
  const { save, unmount } = editor("text", [extension]);
  vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);

  const saved = kept.controls?.save();
  unmount();
  await expect(saved).rejects.toBeInstanceOf(EditorClosed);
  expect(save).not.toHaveBeenCalled();
});

test("a content loaded keeps what waits on a composition: once it ends, an extension's save goes", async () => {
  const { kept, extension } = keeping();
  const { handle, view, save } = editor("text", [extension]);
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);

  const saved = kept.controls?.save();
  handle().load("new");
  expect(save).not.toHaveBeenCalled();
  composing.mockReturnValue(false);
  view().contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await saved;
  expect(save).toHaveBeenCalledOnce();
});

test("what the handle has wait on a composition is dropped as the editor goes, its drop told", () => {
  const { handle, unmount } = editor("text");
  vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  const act = vi.fn();
  const drop = vi.fn();

  handle().whenComposed(act, drop);
  unmount();
  expect(drop).toHaveBeenCalledOnce();
  expect(act).not.toHaveBeenCalled();
});

test("what waits on a composition goes with the editor", async () => {
  const { handle, view, unmount } = editor("text");
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  const act = vi.fn();
  handle().whenComposed(act);
  composing.mockReturnValue(false);
  view().contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));

  unmount();
  await vi.advanceTimersByTimeAsync(100);
  expect(act).not.toHaveBeenCalled();
});

test("Tab indents; the content refers to the line that says how to move out; its words follow the app's language", () => {
  const { view, speak } = editor("text");

  const hint = document.getElementById(view().contentDOM.getAttribute("aria-describedby") ?? "");
  expect(hint?.textContent).toBe("Tab indents. To move out of the editor, press Esc, then Tab.");
  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab", bubbles: true }));
  expect(view().state.doc.toString()).toBe("  text");
  speak("zh-CN");
  expect(view().contentDOM.getAttribute("aria-label")).toBe("页面正文");
  expect(screen.getByText("Tab 键缩进。要离开编辑区，先按 Esc，再按 Tab。")).toBe(hint);
});
