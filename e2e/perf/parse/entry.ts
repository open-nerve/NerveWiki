import { ensureSyntaxTree } from "@codemirror/language";
import { EditorState } from "@codemirror/state";

import { inlineLimit, markdownEditing } from "../../../web/apps/web/src/editor/markdown";

// The parse page (M12/P4 design 4.4, B9): the editor's own Markdown (markdownEditing), built by parse/build.ts into
// one script that a blank page in Chromium runs. It parses as the editor does, a whole document into an EditorState,
// or the change of a character in one already parsed, CodeMirror reusing what it can of the tree; and says what it
// parsed, so that a measure can tell it parsed within the limits, not past them, where a paragraph is plain text.

/** One parse: how long it took, in ms; the paragraph at the offset asked about; the nodes of the whole tree. */
export interface Parsed {
  ms: number;
  /** The innermost paragraph holding the offset: its length, and its links, emphasis and code spans, once each. */
  paragraph: number;
  inline: string[];
  /** How many of each block node the tree has. */
  blocks: Record<string, number>;
}

/** What the page puts on window: the limit, and the two parses. */
export interface NwParse {
  inlineLimit: number;
  /** Parses doc whole; at is an offset in the paragraph to look at. */
  whole(doc: string, at: number): Parsed;
  /** Parses doc whole, untimed, then inserts "a" at where and parses what changed. */
  insert(doc: string, at: number, where: number): Parsed;
}

const blockNames = new Set(["Paragraph", "ListItem", "Blockquote", "LinkReference", "Table"]);
const inlineNames = new Set(["Link", "Emphasis", "InlineCode"]);

function treeOf(state: EditorState) {
  const tree = ensureSyntaxTree(state, state.doc.length, 10_000);
  if (tree === null) {
    throw new Error("the parse did not finish within 10 s");
  }
  return tree;
}

/** What state's tree holds, at the offset at. */
function what(state: EditorState, at: number, ms: number): Parsed {
  const tree = treeOf(state);
  let paragraph: { from: number; to: number } | undefined;
  const blocks: Record<string, number> = {};
  tree.iterate({
    enter: ({ name, from, to }) => {
      if (blockNames.has(name)) blocks[name] = (blocks[name] ?? 0) + 1;
      if (name === "Paragraph" && from <= at && at < to) paragraph = { from, to };
    },
  });
  const inline = new Set<string>();
  if (paragraph !== undefined) {
    tree.iterate({
      from: paragraph.from,
      to: paragraph.to,
      enter: ({ name }) => void (inlineNames.has(name) && inline.add(name)),
    });
  }
  return {
    ms,
    paragraph: paragraph === undefined ? 0 : paragraph.to - paragraph.from,
    inline: [...inline].toSorted(),
    blocks,
  };
}

const nwParse: NwParse = {
  inlineLimit,
  whole: (doc, at) => {
    const started = performance.now();
    const state = EditorState.create({ doc, extensions: markdownEditing() });
    treeOf(state);
    return what(state, at, performance.now() - started);
  },
  insert: (doc, at, where) => {
    const state = EditorState.create({ doc, extensions: markdownEditing() });
    treeOf(state);
    const started = performance.now();
    const next = state.update({ changes: { from: where, insert: "a" } }).state;
    treeOf(next);
    return what(next, at, performance.now() - started);
  },
};

(window as unknown as { nwParse: NwParse }).nwParse = nwParse;
