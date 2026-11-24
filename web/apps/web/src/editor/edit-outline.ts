import { ensureSyntaxTree, syntaxTree, syntaxTreeAvailable } from "@codemirror/language";
import { Facet, type EditorState, type Extension } from "@codemirror/state";
import { ViewPlugin, type EditorView, type ViewUpdate } from "@codemirror/view";

import { frontmatterRange } from "./loaded/yaml-quotes";

/** A heading the edit's outline lists: where it starts as it was read, its level, its text. */
export type OutlineHeading = { from: number; level: number; text: string };

/** OutlineSink is the page's side of the outline: how many it lists at most, and what it is told. */
export type OutlineSink = { upTo: number; told(listed: readonly OutlineHeading[], more: number): void };

/** How long after the last change the outline is read again. */
const settles = 300;

/** How long a slice of the parse the outline completes takes at most. */
const slice = 20;

/** The syntax tree, and one of its nodes, as the editor's Markdown has them. */
type Tree = ReturnType<typeof syntaxTree>;
type SyntaxNode = Tree["topNode"];

/** The sink the outline is told to, as the editor's latest props have it: none, none is read. */
const sinks = Facet.define<() => OutlineSink | undefined, () => OutlineSink | undefined>({
  combine: (values) => values[0] ?? (() => undefined),
});

/** The blocks a heading may be in. */
const containers = new Set(["Document", "Blockquote", "BulletList", "OrderedList", "ListItem"]);

/** The headings' nodes, by their level. */
const levels = new Map([
  ["ATXHeading1", 1],
  ["ATXHeading2", 2],
  ["ATXHeading3", 3],
  ["ATXHeading4", 4],
  ["ATXHeading5", 5],
  ["ATXHeading6", 6],
  ["SetextHeading1", 1],
  ["SetextHeading2", 2],
]);

/**
 * textOf is a heading's text without its marks: an ATX heading's #s; a setext heading's underline, its line's quote
 * marks with it, and the quote's marks on its lines; its spaces made one.
 */
function textOf(state: EditorState, node: SyntaxNode): string {
  let end = node.to;
  const underline = node.name.startsWith("Setext") ? node.lastChild : null;
  if (underline?.name === "HeaderMark") {
    end = state.doc.lineAt(underline.from).from;
  }
  let text = "";
  let at = node.from;
  for (let mark = node.firstChild; mark !== null && mark.to <= end; mark = mark.nextSibling) {
    if (mark.name === "HeaderMark" || mark.name === "QuoteMark") {
      text += `${state.sliceDoc(at, mark.from)} `;
      at = mark.to;
    }
  }
  return `${text}${state.sliceDoc(at, end)}`.replace(/\s+/g, " ").trim();
}

/**
 * headingsOf is the headings of tree, the state's text's: the first upTo listed, the rest counted. In quotes, lists and
 * their items too; not in code, tables nor the frontmatter (lezer reads its YAML as a rule and a setext heading, which
 * the server does not); those without text not at all.
 */
function headingsOf(state: EditorState, tree: Tree, upTo: number): { listed: OutlineHeading[]; more: number } {
  const frontmatter = frontmatterRange(state);
  const listed: OutlineHeading[] = [];
  let more = 0;
  tree.iterate({
    enter: (node) => {
      if (node.name === "Document") {
        return undefined;
      }
      if (frontmatter !== undefined && node.from <= frontmatter.to) {
        return false;
      }
      const level = levels.get(node.name);
      if (level === undefined) {
        return containers.has(node.name) ? undefined : false;
      }
      const text = textOf(state, node.node);
      if (text !== "") {
        if (listed.length < upTo) {
          listed.push({ from: node.from, level, text });
        } else {
          more++;
        }
      }
      return false;
    },
  });
  return { listed, more };
}

/** same tells whether two tellings tell the same: each heading's place, level and text, and how many more. */
function same(
  told: { listed: readonly OutlineHeading[]; more: number } | undefined,
  listed: readonly OutlineHeading[],
  more: number
): boolean {
  return (
    told !== undefined &&
    told.more === more &&
    told.listed.length === listed.length &&
    told.listed.every((heading, i) => {
      const now = listed[i];
      return heading.from === now?.from && heading.level === now.level && heading.text === now.text;
    })
  );
}

/**
 * Outline reads the editor's headings for the page's outline (M13/P4/S3, decision 876): at once as it is made (the
 * right column's outline is there as the edit opens, review A's M3), then once the changes settle. Past the parse the
 * editor has, it completes it a slice at a time, one a task; the text changed before the next slice ends the round,
 * which the change's settling reads again. A state given meanwhile of the same text (a selection, a completion, the
 * parse's own worker) goes on with it (review A's I4). What reads the same as last time is not told. The places of
 * the headings told are mapped through each change since (review A's M2): where answers them.
 */
class Outline {
  /** Where the headings last told start, as the text is now. */
  places: number[] = [];
  private told: { listed: readonly OutlineHeading[]; more: number } | undefined = undefined;
  private settling: ReturnType<typeof setTimeout> | undefined = undefined;
  private slicing: ReturnType<typeof setTimeout> | undefined = undefined;

  constructor(private readonly view: EditorView) {
    this.read();
  }

  update(update: ViewUpdate) {
    if (!update.docChanged) {
      return;
    }
    this.places = this.places.map((at) => update.changes.mapPos(at, 1));
    clearTimeout(this.settling);
    this.settling = setTimeout(() => this.read(), settles);
  }

  destroy() {
    clearTimeout(this.settling);
    clearTimeout(this.slicing);
  }

  private read() {
    const sink = this.view.state.facet(sinks)();
    if (sink === undefined) {
      return;
    }
    const { doc } = this.view.state;
    const step = () => {
      this.slicing = undefined;
      const { state } = this.view;
      if (state.doc !== doc) {
        return;
      }
      const tree = syntaxTreeAvailable(state, doc.length)
        ? syntaxTree(state)
        : ensureSyntaxTree(state, doc.length, slice);
      if (tree === null) {
        this.slicing = setTimeout(step, 0);
        return;
      }
      const { listed, more } = headingsOf(state, tree, sink.upTo);
      this.places = listed.map(({ from }) => from);
      if (!same(this.told, listed, more)) {
        this.told = { listed, more };
        sink.told(listed, more);
      }
    };
    step();
  }
}

const outline = ViewPlugin.fromClass(Outline);

/** editOutline tells sink, as it answers each time, the editor's headings: its outline while the page is edited. */
export function editOutline(sink: () => OutlineSink | undefined): Extension {
  return [sinks.of(sink), outline];
}

/** where is where the outline's index-th heading starts as the editor's text is now: its place told, mapped since. */
export function where(view: EditorView, index: number): number | undefined {
  return view.plugin(outline)?.places[index];
}
