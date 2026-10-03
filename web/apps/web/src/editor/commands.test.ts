import { EditorSelection, EditorState, type StateCommand } from "@codemirror/state";
import { expect, test } from "vitest";

import { insertLink, toggleStrong } from "./commands";

/** The document and the selection's ranges after command, on doc with ranges selected. */
function apply(command: StateCommand, doc: string, ...ranges: [number, number][]) {
  let state = EditorState.create({
    doc,
    selection: EditorSelection.create(ranges.map(([from, to]) => EditorSelection.range(from, to))),
    extensions: EditorState.allowMultipleSelections.of(true),
  });
  expect(command({ state, dispatch: (tr) => (state = tr.state) })).toBe(true);
  return { doc: state.doc.toString(), ranges: state.selection.ranges.map((r) => [r.from, r.to]) };
}

test("Mod+B puts ** around a selection and keeps it selected", () => {
  expect(apply(toggleStrong, "a word here", [2, 6])).toEqual({ doc: "a **word** here", ranges: [[4, 8]] });
});

test("Mod+B takes away the ** around a selection", () => {
  expect(apply(toggleStrong, "a **word** here", [4, 8])).toEqual({ doc: "a word here", ranges: [[2, 6]] });
});

test("Mod+B on an empty selection puts the cursor between ****", () => {
  expect(apply(toggleStrong, "ab", [1, 1])).toEqual({ doc: "a****b", ranges: [[3, 3]] });
});

test("Mod+B takes each selection on its own", () => {
  expect(apply(toggleStrong, "one two", [0, 3], [4, 7])).toEqual({
    doc: "**one** **two**",
    ranges: [
      [2, 5],
      [10, 13],
    ],
  });
});

test("Mod+K makes a selection a link's text, the cursor where its address goes", () => {
  expect(apply(insertLink, "see docs now", [4, 8])).toEqual({ doc: "see [docs]() now", ranges: [[11, 11]] });
});

test("Mod+K on an empty selection puts the cursor where the link's text goes", () => {
  expect(apply(insertLink, "ab", [1, 1])).toEqual({ doc: "a[]()b", ranges: [[2, 2]] });
});

const refused = () => {
  throw new Error("dispatched");
};

test("Mod+B and Mod+K leave a read-only content as it is", () => {
  const state = EditorState.create({ doc: "word", extensions: EditorState.readOnly.of(true) });

  expect(toggleStrong({ state, dispatch: refused })).toBe(false);
  expect(insertLink({ state, dispatch: refused })).toBe(false);
});
