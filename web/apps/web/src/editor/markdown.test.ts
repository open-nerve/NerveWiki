import { ensureSyntaxTree } from "@codemirror/language";
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

// The links, ] and * of a paragraph each scanned the paragraph's marks
// before: 50,000 links took 9.5 s to edit (M6 closeout B-I1; the patch of
// @lezer/markdown). A closing mark longer than the one it closes moved the
// paragraph's marks after it: 80,000 *a** took 8 s (M6 closeout FB-I2).
test.each([
  [100_000, "links", "[[Page]] "],
  [100_000, "links and brackets that close none", "[a](b) ] "],
  [100_000, "emphasis that closes none", "a* "],
  [200_000, "emphasis closed by a longer mark", "*a** "],
  [200_000, "strong emphasis in emphasis", "***a*** "],
])("a paragraph of %i %s is parsed in a time as long as it", (count, _, unit) => {
  const doc = unit.repeat(count);
  const started = performance.now();
  const state = EditorState.create({ doc, extensions: markdownEditing() });
  expect(ensureSyntaxTree(state, doc.length, 10_000)?.length).toBe(doc.length);
  expect(performance.now() - started).toBeLessThan(3_000);
});
