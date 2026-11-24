import { syntaxTree } from "@codemirror/language";
import { RangeSetBuilder, type Range } from "@codemirror/state";
import { Decoration, EditorView, ViewPlugin, type DecorationSet, type ViewUpdate } from "@codemirror/view";

import { frontmatterRange, type Frontmatter } from "./loaded/yaml-quotes";

/** The syntax tree, and one of its nodes, as the editor's Markdown has them. */
type Tree = ReturnType<typeof syntaxTree>;
type SyntaxNode = Tree["topNode"];

/**
 * proseTheme is the source editor's look as the page reads (00-M13 4.5.5; M13/P4/S3, decision 874): its prose in the
 * reading's font, 15.5px, 1.85 the line, on the surface (the code's background in the dark), as the mockup's .editor.
 * The source editor alone has it, before the editor's theme and over it (Prec.high: CodeMirror mounts the style modules
 * the other way round, the first winning); a comparison and a conflict stay monospaced.
 */
export const proseTheme = EditorView.theme({
  "&": { backgroundColor: "var(--surface)", borderRadius: "12px" },
  ".dark &": { backgroundColor: "var(--nw-code-background)" },
  ".cm-scroller": { fontFamily: "var(--font-sans)" },
  // The caret is the browser's (no drawSelection()): its colour is the content's caret-color (review B's R1-M1).
  ".cm-content": { fontSize: "15.5px", lineHeight: "1.85", padding: "22px 0", caretColor: "var(--accent)" },
  ".cm-line": { padding: "0 26px" },
  // Its line as high as its type is: the estimate of the lines not measured yet as off as the editor's of old
  // (section 1 (a)'s probe).
  ".nw-mono-line": { fontFamily: "var(--font-mono)", fontSize: "13.5px" },
  // The frontmatter is no Markdown: lezer's heading, rule and links in it drawn as plain text (decision 889).
  ".nw-frontmatter-line": { color: "var(--muted-foreground)" },
  ".nw-frontmatter-line *": { color: "inherit", fontSize: "inherit", fontWeight: "inherit" },
  // markupMarks': a faint mark fades the highlighting's spans inside it too, which lie flat in it.
  ".nw-md-faint, .nw-md-faint *": { color: "var(--faint)" },
  ".nw-md-link": {
    textDecoration: "underline",
    textDecorationColor: "var(--nw-link-line)",
    textUnderlineOffset: "3px",
  },
});

const mono = Decoration.line({ class: "nw-mono-line" });
const front = Decoration.line({ class: "nw-mono-line nw-frontmatter-line" });
/** The blocks whose lines are monospaced. */
const monoNodes = new Set(["FencedCode", "CodeBlock", "Table"]);
/** The blocks the blocks above may be in. */
const containers = new Set(["Document", "Blockquote", "BulletList", "OrderedList", "ListItem"]);

/** begunIn tells whether node starts in the frontmatter, which lezer reads as Markdown and the server does not. */
const begunIn = (frontmatter: Frontmatter | undefined, node: { from: number }) =>
  frontmatter !== undefined && node.from <= frontmatter.to;

/**
 * linesOf is the lines in view of the frontmatter, of code blocks and of tables, each decorated once, in order. The
 * syntax tree read is the one there is: no more of it is parsed (M12/P4/S5's bounds hold).
 */
function linesOf(view: EditorView): DecorationSet {
  const { state } = view;
  const { doc } = state;
  const frontmatter = frontmatterRange(state);
  const tree = syntaxTree(state);
  const builder = new RangeSetBuilder<Decoration>();
  // The start of the line decorated last: a line two ranges share is decorated once.
  let last = -1;
  const lines = (from: number, to: number, decoration: Decoration) => {
    for (let at = from; at <= to;) {
      const line = doc.lineAt(at);
      if (line.from > last) {
        builder.add(line.from, line.from, decoration);
        last = line.from;
      }
      at = line.to + 1;
    }
  };
  for (const { from, to } of view.visibleRanges) {
    if (frontmatter !== undefined && from <= frontmatter.to) {
      lines(from, Math.min(to, frontmatter.to), front);
    }
    tree.iterate({
      from,
      to,
      enter: (node) => {
        if (node.name === "Document") {
          return undefined;
        }
        if (begunIn(frontmatter, node)) {
          return false;
        }
        if (monoNodes.has(node.name)) {
          lines(Math.max(node.from, from), Math.min(node.to, to), mono);
          return false;
        }
        return containers.has(node.name) ? undefined : false;
      },
    });
  }
  return builder.finish();
}

/**
 * proseLines has the lines of code, of tables and of the frontmatter monospaced, those in view alone (decision 874):
 * the frontmatter's of the frontmatter too. Drawn anew as the content, the lines in view or the syntax tree change.
 */
export const proseLines = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;
    constructor(view: EditorView) {
      this.decorations = linesOf(view);
    }
    update(update: ViewUpdate) {
      if (update.docChanged || update.viewportChanged || syntaxTree(update.startState) !== syntaxTree(update.state)) {
        this.decorations = linesOf(update.view);
      }
    }
  },
  { decorations: (plugin) => plugin.decorations }
);

const faint = Decoration.mark({ class: "nw-md-faint" });
const link = Decoration.mark({ class: "nw-md-link" });
/** The nodes whose text is code or an address: no mark in them. */
const verbatim = new Set([
  "InlineCode",
  "CodeText",
  "FencedCode",
  "CodeBlock",
  "URL",
  "Autolink",
  "HTMLTag",
  "HTMLBlock",
  "Comment",
  "CommentBlock",
  "ProcessingInstructionBlock",
]);
/** The nodes markupMarks reads no link in. */
const code = new Set(["FencedCode", "CodeBlock", "InlineCode"]);
/** A wiki link, an embed's too: what is between its brackets. */
const wiki = /!?\[\[([^[\]\n]*)\]\]/g;
/** A highlight: its text, which neither starts nor ends with a space or a "=". */
const highlight = /==(?=[^=\s])([^=\n]*?[^=\s])==/g;

/** verbatimAt tells whether pos is in code or an address, as the syntax tree has it. */
function verbatimAt(tree: Tree, pos: number): boolean {
  for (let node: SyntaxNode | null = tree.resolveInner(pos, 1); node !== null; node = node.parent) {
    if (verbatim.has(node.name)) {
      return true;
    }
  }
  return false;
}

/**
 * wikiMarks marks a wiki link at from whose inner text, at inner, is text: its brackets (an embed's "!" with them),
 * its section's "#" and its alias's "|" faint; its page underlined, or with no page its section.
 */
function wikiMarks(marks: Range<Decoration>[], from: number, to: number, inner: number, text: string) {
  marks.push(faint.range(from, inner), faint.range(to - 2, to));
  const bar = text.indexOf("|");
  const target = bar < 0 ? text : text.slice(0, bar);
  const hash = target.indexOf("#");
  const page = hash < 0 ? target : target.slice(0, hash);
  if (page !== "") {
    marks.push(link.range(inner, inner + page.length));
  } else if (hash >= 0 && hash + 1 < target.length) {
    marks.push(link.range(inner + hash + 1, inner + target.length));
  }
  if (hash >= 0) {
    marks.push(faint.range(inner + hash, inner + hash + 1));
  }
  if (bar >= 0) {
    marks.push(faint.range(inner + bar, inner + bar + 1));
  }
}

/** linkMarks underlines a link's text, [text](address) of a link of an address: an image's is none, nor a reference's. */
function linkMarks(marks: Range<Decoration>[], node: SyntaxNode) {
  if (node.getChild("URL") === null) {
    return;
  }
  const [open, close] = node.getChildren("LinkMark");
  if (open !== undefined && close !== undefined && open.to < close.from) {
    marks.push(link.range(open.to, close.from));
  }
}

/**
 * marksOf is the marks of the lines in view, past the frontmatter: a link's text underlined, its brackets and address
 * faint by the highlighting; a wiki link's and a highlight's marks, which lezer has no syntax of, found in each line.
 */
function marksOf(view: EditorView): DecorationSet {
  const { state } = view;
  const frontmatter = frontmatterRange(state);
  const tree = syntaxTree(state);
  const marks: Range<Decoration>[] = [];
  for (const { from, to } of view.visibleRanges) {
    tree.iterate({
      from,
      to,
      enter: (node) => {
        if (node.name === "Document") {
          return undefined;
        }
        if (begunIn(frontmatter, node) || code.has(node.name) || node.name === "Image") {
          return false;
        }
        if (node.name === "Link") {
          linkMarks(marks, node.node);
          return false;
        }
        return undefined;
      },
    });
    for (let at = from; at <= to;) {
      const line = state.doc.lineAt(at);
      at = line.to + 1;
      if (!begunIn(frontmatter, line)) {
        lineMarks(marks, tree, line);
      }
    }
  }
  return Decoration.set(marks, true);
}

/** lineMarks marks a line's wiki links and highlights, but those that start (or a highlight's end) in code. */
function lineMarks(marks: Range<Decoration>[], tree: Tree, line: { from: number; text: string }): void {
  for (const found of line.text.matchAll(wiki)) {
    const from = line.from + found.index;
    if (!verbatimAt(tree, from)) {
      const inner = from + (found[0].startsWith("!") ? 3 : 2);
      wikiMarks(marks, from, from + found[0].length, inner, found[1] ?? "");
    }
  }
  for (const found of line.text.matchAll(highlight)) {
    const from = line.from + found.index;
    const to = from + found[0].length;
    if (!verbatimAt(tree, from) && !verbatimAt(tree, to - 2)) {
      marks.push(faint.range(from, from + 2), faint.range(to - 2, to));
    }
  }
}

/**
 * markupMarks fades a wiki link's and a highlight's marks and underlines a link's text (decision 889), in the lines in
 * view alone: never in code, an address or the frontmatter. Drawn anew as the content, the lines in view or the syntax
 * tree change. A wiki link or a highlight over lines is not found: the server's cross no paragraph, and those over
 * lines are few.
 */
export const markupMarks = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;
    constructor(view: EditorView) {
      this.decorations = marksOf(view);
    }
    update(update: ViewUpdate) {
      if (update.docChanged || update.viewportChanged || syntaxTree(update.startState) !== syntaxTree(update.state)) {
        this.decorations = marksOf(update.view);
      }
    }
  },
  { decorations: (plugin) => plugin.decorations }
);
