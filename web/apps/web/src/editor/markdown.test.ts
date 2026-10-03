import { EditorSelection, EditorState } from "@codemirror/state";
import { EditorView, runScopeHandlers } from "@codemirror/view";
import { expect, test } from "vitest";

import { markdownEditing } from "./markdown";

/** The document after key is pressed with the cursor at the end of doc. */
function press(doc: string, key: string): string {
  const view = new EditorView({
    state: EditorState.create({ doc, selection: EditorSelection.cursor(doc.length), extensions: markdownEditing() }),
    parent: document.body,
  });
  expect(runScopeHandlers(view, new KeyboardEvent("keydown", { key }), "editor")).toBe(true);
  const after = view.state.doc.toString();
  view.destroy();
  return after;
}

test.each([
  ["a bullet", "- item", "- item\n- "],
  ["a number", "1. first", "1. first\n2. "],
  ["a task", "- [ ] task", "- [ ] task\n- [ ] "],
  ["a quote", "> said", "> said\n> "],
])("Enter continues %s", (_, doc, after) => {
  expect(press(doc, "Enter")).toBe(after);
});

test("Enter on an empty item makes room before it, and on that again ends the list", () => {
  expect(press("- item\n- ", "Enter")).toBe("- item\n\n- ");
  expect(press("- item\n\n- ", "Enter")).toBe("- item\n\n");
});

test("Backspace at an item's mark takes the mark away, the item's indent kept", () => {
  expect(press("- item\n- ", "Backspace")).toBe("- item\n  ");
});
