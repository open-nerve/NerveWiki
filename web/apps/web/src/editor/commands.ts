import { EditorSelection, type StateCommand } from "@codemirror/state";

const strong = "**";

/**
 * toggleStrong is Mod+B (M4/P6 design 3.5): it puts ** on both sides of
 * each selection, or takes them away when they are there already; an
 * empty selection gets **** with the cursor between.
 */
export const toggleStrong: StateCommand = ({ state, dispatch }) => {
  const changes = state.changeByRange((range) => {
    const before = state.sliceDoc(range.from - strong.length, range.from);
    const after = state.sliceDoc(range.to, range.to + strong.length);
    if (before === strong && after === strong) {
      return {
        changes: [
          { from: range.from - strong.length, to: range.from },
          { from: range.to, to: range.to + strong.length },
        ],
        range: EditorSelection.range(range.from - strong.length, range.to - strong.length),
      };
    }
    return {
      changes: [
        { from: range.from, insert: strong },
        { from: range.to, insert: strong },
      ],
      range: EditorSelection.range(range.from + strong.length, range.to + strong.length),
    };
  });
  dispatch(state.update(changes, { scrollIntoView: true, userEvent: "input" }));
  return true;
};

/**
 * insertLink is Mod+K (M4/P6 design 3.5): a selection becomes the text of
 * [selection]() with the cursor in the parentheses, for the address; an
 * empty one gets []() with the cursor in the brackets, for the text.
 */
export const insertLink: StateCommand = ({ state, dispatch }) => {
  const changes = state.changeByRange((range) => {
    const text = state.sliceDoc(range.from, range.to);
    const cursor = range.empty ? range.from + 1 : range.from + text.length + 3;
    return {
      changes: { from: range.from, to: range.to, insert: `[${text}]()` },
      range: EditorSelection.cursor(cursor),
    };
  });
  dispatch(state.update(changes, { scrollIntoView: true, userEvent: "input" }));
  return true;
};
