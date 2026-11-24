import { ensureSyntaxTree, syntaxTree, syntaxTreeAvailable } from "@codemirror/language";
import { Facet, type EditorState, type Extension } from "@codemirror/state";
import { ViewPlugin, type EditorView, type ViewUpdate } from "@codemirror/view";

import { frontmatterRange } from "./loaded/yaml-quotes";
import { shownOf, wikiLinks } from "./wiki-links";

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

/** A part of a heading's text its item does not show as written: cut, or shown as text. */
type Cut = { from: number; to: number; text: string };

/** The marks an item drops: a heading's, a quote's, emphasis', code's, a strikethrough's; HTML's tags. */
const dropped = new Set(["HeaderMark", "QuoteMark", "EmphasisMark", "CodeMark", "StrikethroughMark", "HTMLTag"]);
/** The nodes whose text holds no wiki link nor highlight: code, HTML, an address. */
const verbatim = new Set(["InlineCode", "HTMLTag", "Autolink", "URL"]);
/** A highlight: its text, which neither starts nor ends with a space or a "=". */
const highlight = /==(?=[^=\s])([^=\n]*?[^=\s])==/g;

/** decoded is the character an HTML entity stands for, as the server's Markdown reads it. */
function decoded(entity: string): string {
  const area = document.createElement("textarea");
  area.innerHTML = entity;
  return area.value;
}

/**
 * cutsOf adds to cuts the marks of node's inline children, before end, that its item does not show, and to codes
 * where code and addresses are: a footnote's reference whole; a link's parts but its text, an image's but its alt (a
 * shortcut reference "[x]" stays as written: the server reads it as a link only where it is defined); an escape's
 * "\"; an entity as its character.
 */
function cutsOf(state: EditorState, node: SyntaxNode, end: number, cuts: Cut[], codes: Cut[]): void {
  for (let child = node.firstChild; child !== null && child.from < end; child = child.nextSibling) {
    const { name, from, to } = child;
    if (verbatim.has(name)) {
      codes.push({ from, to, text: "" });
    }
    if (dropped.has(name)) {
      cuts.push({ from, to, text: "" });
    } else if (name === "Escape") {
      cuts.push({ from, to: from + 1, text: "" });
    } else if (name === "Entity") {
      cuts.push({ from, to, text: decoded(state.sliceDoc(from, to)) });
    } else if (name === "Link" && child.getChild("URL") === null && state.sliceDoc(from + 1, from + 2) === "^") {
      cuts.push({ from, to, text: "" });
    } else if (name === "Autolink") {
      const [open, close] = child.getChildren("LinkMark");
      for (const mark of [open, close]) {
        if (mark !== undefined) {
          cuts.push({ from: mark.from, to: mark.to, text: "" });
        }
      }
    } else if (
      name === "Image" ||
      (name === "Link" && (child.getChild("URL") !== null || child.getChild("LinkLabel") !== null))
    ) {
      // Its text alone: its "[" (an image's "![") cut, and all from the "]" after its text to its end.
      const [open, close] = child.getChildren("LinkMark");
      if (open !== undefined) {
        cuts.push({ from: open.from, to: open.to, text: "" });
      }
      if (close !== undefined) {
        // Its address, its title or its label holds no wiki link nor highlight.
        cuts.push({ from: close.from, to: Math.min(child.to, end), text: "" });
        codes.push({ from: close.from, to: child.to, text: "" });
      }
      cutsOf(state, child, close?.from ?? end, cuts, codes);
    } else {
      cutsOf(state, child, end, cuts, codes);
    }
  }
}

/** inside tells whether at is in one of ranges. */
const inside = (ranges: readonly Cut[], at: number) => ranges.some(({ from, to }) => at >= from && at < to);

/**
 * textOf is a heading's text as the reading view's outline has it (M13/P4/S3, decision 907): without its marks (an ATX
 * heading's #s; a setext heading's underline, its line's quote marks with it, and the quote's marks on its lines),
 * its inline marks, addresses and footnotes' references; an image by its alt; a wiki link as the view shows it, a
 * highlight without its "=="; in code as written. Its spaces made one.
 */
function textOf(state: EditorState, node: SyntaxNode): string {
  let end = node.to;
  const underline = node.name.startsWith("Setext") ? node.lastChild : null;
  if (underline?.name === "HeaderMark") {
    end = state.doc.lineAt(underline.from).from;
  }
  const marks: Cut[] = [];
  const codes: Cut[] = [];
  cutsOf(state, node, end, marks, codes);
  const text = state.sliceDoc(node.from, end);
  const links: Cut[] = wikiLinks(text, node.from)
    .filter((link) => !inside(codes, link.from))
    .map((link) => ({ from: link.from, to: link.to, text: shownOf(link) }));
  for (const found of text.matchAll(highlight)) {
    const from = node.from + found.index;
    const to = from + found[0].length;
    if (!inside(codes, from) && !inside(codes, to - 2) && !inside(links, from)) {
      marks.push({ from, to: from + 2, text: "" }, { from: to - 2, to, text: "" });
    }
  }
  // A wiki link is whole: lezer's reading of its brackets as a link's or an image's is not cut again.
  const cuts = [...links, ...marks.filter(({ from, to }) => !links.some((link) => from < link.to && to > link.from))];
  cuts.sort((a, b) => a.from - b.from);
  let shown = "";
  let at = node.from;
  for (const cut of cuts) {
    if (cut.from >= at) {
      shown += state.sliceDoc(at, cut.from) + cut.text;
      at = cut.to;
    }
  }
  return `${shown}${state.sliceDoc(at, end)}`.replace(/\s+/g, " ").trim();
}

/**
 * headingsOf is the headings of tree, the state's text's: the first upTo listed, the rest counted. In quotes, lists and
 * their items too; not in code, tables nor a closed frontmatter (lezer reads its YAML as a rule and a setext heading,
 * which the server does not); one being written, which the server reads as the body, is the body's (decision 908);
 * those without text not at all.
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
      if (frontmatter?.closed === true && node.from <= frontmatter.to) {
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

/**
 * same tells whether two tellings tell the same: each heading's level and text, and how many more. Their places are
 * the plugin's, mapped: text before them tells nothing again (review A's P4S3A-N3).
 */
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
      return heading.level === now?.level && heading.text === now.text;
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
