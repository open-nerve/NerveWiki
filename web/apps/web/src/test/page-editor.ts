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

/**
 * marked is what a diff's unified view, the textbox of its text after,
 * marks: the lines it shows removed and those it shows added, in order.
 */
export function marked(diff: HTMLElement): { removed: string[]; added: string[] } {
  const lines = (selector: string) => [...diff.querySelectorAll(selector)].map((line) => line.textContent);
  return { removed: lines(".cm-deletedLine"), added: lines(".cm-changedLine") };
}
