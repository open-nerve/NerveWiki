import { EditorState, StateEffect, type Extension } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, expect, test, vi } from "vitest";

import { composeExtensions, readOnly, readOnlyAs, type Composed } from "./extensions";
import type { EditorContext, EditorControls, EditorExtension } from "./registry";

// The editor's extension pipeline (M4/P6 design 3.4; M0/P1 editor handoff,
// item 2), with an extension shaped as each of M5, M6 and M7 will register.

afterEach(() => vi.useRealTimers());

const context: EditorContext = { workspace: "lab", notebook: "n1", page: "p1", role: "editor" };

/** M5's lock, as a push would tell it. */
const locked = StateEffect.define<boolean>();

/** M5: read-only while another holds the lock; a save a second after the last change. */
const lockAndAutosave: EditorExtension = {
  name: "lock-and-autosave",
  extension: (_, controls) => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    return EditorView.updateListener.of((update) => {
      for (const tr of update.transactions) {
        for (const effect of tr.effects) {
          if (effect.is(locked)) {
            controls.setReadOnly(effect.value);
          }
        }
      }
      if (update.docChanged) {
        clearTimeout(timer);
        timer = setTimeout(() => void controls.save(), 1000);
      }
    });
  },
};

/** M6: completion of page names, as a completion source the language data offers; then of tags. */
const pageNames = () => null;
const tags = () => null;
const completion = (source: () => null): EditorExtension => ({
  name: "completion",
  extension: () => EditorState.languageData.of(() => [{ autocomplete: source }]),
});

/** M7: a paste of files goes to an upload. */
const pasted: string[] = [];
const pasteUpload = (name: string, takes = false): EditorExtension => ({
  name,
  extension: () =>
    EditorView.domEventHandlers({
      paste: () => {
        pasted.push(name);
        return takes;
      },
    }),
});

/** A view with registered composed after a document, and its controls' save. */
function editing(registered: readonly EditorExtension[]) {
  pasted.length = 0;
  const save = vi.fn(() => Promise.resolve());
  let view: EditorView | undefined;
  const controls: EditorControls = {
    save,
    saving: () => false,
    setReadOnly: (on) => view?.dispatch({ effects: readOnly.reconfigure(readOnlyAs(on)) }),
  };
  const composed: Composed = composeExtensions(registered, context, controls);
  const extensions: Extension = [readOnly.of(readOnlyAs(false)), composed.extension];
  view = new EditorView({ state: EditorState.create({ doc: "text", extensions }), parent: document.body });
  const type = () => view.dispatch({ changes: { from: 0, insert: "x" } });
  const paste = () => view.contentDOM.dispatchEvent(new Event("paste", { bubbles: true, cancelable: true }));
  const sources = () => view.state.languageDataAt<() => null>("autocomplete", 0);
  return { view, composed, save, type, paste, sources };
}

test("the three compose in their order, each doing its own", () => {
  vi.useFakeTimers();
  const { view, save, type, paste, sources } = editing([
    lockAndAutosave,
    completion(pageNames),
    pasteUpload("upload"),
    pasteUpload("second paste handler"),
  ]);

  type();
  vi.advanceTimersByTime(1000);
  expect(save).toHaveBeenCalledOnce();
  view.dispatch({ effects: locked.of(true) });
  expect(view.state.readOnly).toBe(true);
  expect(sources()).toEqual([pageNames]);
  paste();
  expect(pasted).toEqual(["upload", "second paste handler"]);
  view.destroy();
});

test("one unloaded, the others go on", () => {
  vi.useFakeTimers();
  const { view, composed, save, type, paste, sources } = editing([
    lockAndAutosave,
    completion(pageNames),
    pasteUpload("upload"),
  ]);

  view.dispatch({ effects: composed.reconfigure("lock-and-autosave", null) });
  type();
  vi.advanceTimersByTime(1000);
  expect(save).not.toHaveBeenCalled();
  expect(sources()).toEqual([pageNames]);
  paste();
  expect(pasted).toEqual(["upload"]);
  view.destroy();
});

test("one replaced, the new one is in and the old one out", () => {
  const { view, composed, sources } = editing([completion(pageNames), pasteUpload("upload")]);

  view.dispatch({
    effects: composed.reconfigure("completion", completion(tags).extension(context, {} as EditorControls)),
  });
  expect(sources()).toEqual([tags]);
  expect(composed.reconfigure("not registered", null)).toBeUndefined();
  view.destroy();
});

test("one whose building throws is left out, the others are not", () => {
  const failed = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const broken: EditorExtension = {
    name: "broken",
    extension: () => {
      throw new Error("no");
    },
  };
  const { view, paste, sources } = editing([broken, completion(pageNames), pasteUpload("upload")]);

  expect(failed).toHaveBeenCalledOnce();
  expect(sources()).toEqual([pageNames]);
  paste();
  expect(pasted).toEqual(["upload"]);
  view.destroy();
});
