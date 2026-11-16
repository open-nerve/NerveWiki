import { ensureSyntaxTree, language } from "@codemirror/language";
import { EditorState } from "@codemirror/state";

import { inlineLimit, markdownEditing, referenceLimit } from "../../../web/apps/web/src/editor/markdown";

// The parse page (M12/P4 design 4.4, B9): the editor's own Markdown (markdownEditing), built by parse/build.ts into
// one script that a blank page in Chromium runs. It parses as the editor does: a whole document once, as a paste
// into an empty one; or the change of a character in one already parsed, CodeMirror reusing what it can of the tree.
// It says what it parsed, so that a measure can tell it parsed within the limits, not past them, where a paragraph
// is plain text.
//
// A whole document is parsed once (B9's budget is a parse's): a state of an empty document, then one transaction to
// the whole, its tree to the end. Not EditorState.create and its tree to the end, as the first rounds had it: create
// parses up to 3,000 characters and stops there, and the next parse cannot reuse a paragraph across that point, so
// that one parsed twice (review B's I1 on M12/P4/S2). The editor's parser alone, outside CodeMirror, is a check on it.

type Tree = NonNullable<ReturnType<typeof ensureSyntaxTree>>;

/** One parse: how long it took, in ms; the paragraph at the offset asked about; the nodes of the whole tree. */
export interface Parsed {
  ms: number;
  /** The innermost paragraph holding the offset: its length, and its links, emphasis and code spans, once each. */
  paragraph: number;
  inline: string[];
  /** The length of the link reference definition holding the offset; 0 when none does. */
  definition: number;
  /** How many of each block node the tree has. */
  blocks: Record<string, number>;
}

/** What the page puts on window: the limits, and the parses. */
export interface NwParse {
  inlineLimit: number;
  /** A definition's over lines (M12/P4/S5). */
  referenceLimit: number;
  /** Parses doc whole, once, as a paste into an empty document; at is an offset in the paragraph to look at. */
  whole(doc: string, at: number): Parsed;
  /** The editor's parser alone parses doc, once, outside CodeMirror's state: a check on whole. */
  parser(doc: string, at: number): Parsed;
  /** Parses doc whole, untimed, then inserts "a" at where and parses what changed. */
  insert(doc: string, at: number, where: number): Parsed;
}

const blockNames = new Set(["Paragraph", "ListItem", "Blockquote", "LinkReference", "Table"]);
const inlineNames = new Set(["Link", "Emphasis", "InlineCode"]);

function treeOf(state: EditorState): Tree {
  const tree = ensureSyntaxTree(state, state.doc.length, 10_000);
  if (tree === null) {
    throw new Error("the parse did not finish within 10 s");
  }
  return tree;
}

/** What tree holds, at the offset at. */
function what(tree: Tree, at: number, ms: number): Parsed {
  let paragraph: { from: number; to: number } | undefined;
  let definition = 0;
  const blocks: Record<string, number> = {};
  tree.iterate({
    enter: ({ name, from, to }) => {
      if (blockNames.has(name)) blocks[name] = (blocks[name] ?? 0) + 1;
      if (name === "Paragraph" && from <= at && at < to) paragraph = { from, to };
      if (name === "LinkReference" && from <= at && at < to) definition = to - from;
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
    definition,
    blocks,
  };
}

const nwParse: NwParse = {
  inlineLimit,
  referenceLimit,
  whole: (doc, at) => {
    const empty = EditorState.create({ extensions: markdownEditing() });
    treeOf(empty);
    const started = performance.now();
    const state = empty.update({ changes: { from: 0, insert: doc } }).state;
    const tree = treeOf(state);
    return what(tree, at, performance.now() - started);
  },
  parser: (doc, at) => {
    const editing = EditorState.create({ extensions: markdownEditing() }).facet(language);
    if (editing === null) throw new Error("no language");
    const started = performance.now();
    const tree = editing.parser.parse(doc);
    return what(tree, at, performance.now() - started);
  },
  insert: (doc, at, where) => {
    const state = EditorState.create({ doc, extensions: markdownEditing() });
    treeOf(state);
    const started = performance.now();
    const next = state.update({ changes: { from: where, insert: "a" } }).state;
    const tree = treeOf(next);
    return what(tree, at, performance.now() - started);
  },
};

(window as unknown as { nwParse: NwParse }).nwParse = nwParse;
