import { EditorView } from "@codemirror/view";
import { act, screen } from "@testing-library/react";

/**
 * pageEditor is the page's editor once it shows, and a way to type at its
 * end. StrictMode makes the editor again as the effects of the first one
 * run: they are run before it is looked for.
 */
export async function pageEditor() {
  await screen.findByRole("textbox", { name: "Page content" });
  await act(async () => {});
  const content = screen.getByRole("textbox", { name: "Page content" });
  const element = content.closest<HTMLElement>(".cm-editor");
  const view = element === null ? null : EditorView.findFromDOM(element);
  if (view === null) {
    throw new Error("no editor");
  }
  const type = (text: string) =>
    view.dispatch({ changes: { from: view.state.doc.length, insert: text }, userEvent: "input.type" });
  return { content, view, type };
}
