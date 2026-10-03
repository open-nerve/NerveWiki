import { EditorView } from "@codemirror/view";
import { render } from "@testing-library/react";
import { createRef, StrictMode, type ReactNode } from "react";
import { expect, test, vi } from "vitest";

import { I18nProvider } from "../i18n/i18n";
import { EditorExtensions, type EditorContext, type EditorExtension } from "./registry";
import { SourceEditor, type SourceEditorHandle } from "./source-editor";

const context: EditorContext = { workspace: "lab", notebook: "n1", page: "p1", role: "editor" };

/** The editor on content, its handle, and what it told onChange and save. */
function editor(content: string, extensions: readonly EditorExtension[] = [], wrap = (node: ReactNode) => node) {
  const handle = createRef<SourceEditorHandle>();
  const onChange = vi.fn();
  const save = vi.fn(() => Promise.resolve());
  const { container } = render(
    wrap(
      <I18nProvider locale="en">
        <EditorExtensions value={extensions}>
          <SourceEditor
            ref={handle}
            content={content}
            context={context}
            controls={{ save, saving: () => false }}
            onChange={onChange}
          />
        </EditorExtensions>
      </I18nProvider>
    )
  );
  const view = () => {
    const element = container.querySelector<HTMLElement>(".cm-editor");
    const found = element === null ? null : EditorView.findFromDOM(element);
    if (found === null) {
      throw new Error("no editor");
    }
    return found;
  };
  const type = (text: string, at = view().state.doc.length) => view().dispatch({ changes: { from: at, insert: text } });
  return { container, handle: () => handle.current as SourceEditorHandle, view, type, onChange, save };
}

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

test("the registered extensions are in, after the editor's own; their controls reach the latest save", async () => {
  const saving: EditorExtension = {
    name: "save on change",
    extension: (_, controls) => EditorView.updateListener.of((update) => void (update.docChanged && controls.save())),
  };
  const { type, save } = editor("text", [saving]);

  type("!");
  await Promise.resolve();
  expect(save).toHaveBeenCalledOnce();
});

test("Mod+B is the editor's", () => {
  const { view, handle } = editor("word");
  view().dispatch({ selection: { anchor: 0, head: 4 } });

  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "b", ctrlKey: true, bubbles: true }));
  expect(handle().text()).toBe("**word**");
});
