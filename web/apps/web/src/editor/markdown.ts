import { deleteMarkupBackward, insertNewlineContinueMarkup, markdownLanguage } from "@codemirror/lang-markdown";
import { HighlightStyle, Language, LanguageSupport, syntaxHighlighting } from "@codemirror/language";
import { Prec, type Extension } from "@codemirror/state";
import { keymap } from "@codemirror/view";
import { tags } from "@lezer/highlight";
import type { MarkdownConfig, MarkdownParser } from "@lezer/markdown";

/**
 * The headings stand out by weight and size, the code is monospaced; the
 * syntax's own marks (#, *, `, [) stay as written, only fainter (M4/P6
 * design 3.2). The colours are the app's, light or dark.
 */
const markdownHighlight = HighlightStyle.define([
  { tag: tags.heading1, fontWeight: "700", fontSize: "1.6em" },
  { tag: tags.heading2, fontWeight: "700", fontSize: "1.4em" },
  { tag: tags.heading3, fontWeight: "700", fontSize: "1.25em" },
  { tag: tags.heading4, fontWeight: "700", fontSize: "1.1em" },
  { tag: [tags.heading5, tags.heading6], fontWeight: "700" },
  { tag: tags.strong, fontWeight: "700" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.strikethrough, textDecoration: "line-through" },
  { tag: tags.monospace, fontFamily: "var(--nw-editor-mono)" },
  { tag: [tags.link, tags.url], color: "var(--nw-editor-link)" },
  { tag: [tags.processingInstruction, tags.contentSeparator, tags.labelName], color: "var(--muted-foreground)" },
  { tag: tags.quote, color: "var(--muted-foreground)" },
]);

/**
 * inlineLimit is the longest inline text, a paragraph's, a heading's or a
 * table cell's, in which the editor finds links, emphasis and code; a
 * longer one shows as plain text. Many of lezer's inline parsers scan on
 * from each mark they meet, to the text's end or back to its start: a
 * paragraph's run of spaces, its "<?", its links, a link's address or
 * title not closed, its emphasis closed in part each cost the square of
 * their length. 96 KB of spaces took 5.6 s to enter the edit, in one task
 * a writer's content alone decides (M6 closeout B-I1, FB-I2, FB2-I1). At
 * the limit the costliest takes about 80 ms.
 */
export const inlineLimit = 10_000;

/**
 * blockDepth is how deep in blocks (lists, their items, quotes) the editor
 * reads a line's structure; the rest of a line deeper is plain text. Each
 * list mark counted the line's columns again from its start: a line of
 * 80,000 "- " took 19 s (M6 closeout FB2-I1). 100 holds 50 lists, one in another.
 */
export const blockDepth = 100;

/** bounded reads what is past inlineLimit or blockDepth as plain text, before any other parser. */
const bounded: MarkdownConfig = {
  parseBlock: [
    {
      name: "DeepBlockAsText",
      before: "LinkReference",
      parse: (cx, line) => {
        if (cx.depth < blockDepth) {
          return false;
        }
        cx.addElement(cx.elt("Paragraph", cx.lineStart + line.pos, cx.lineStart + line.text.length));
        cx.nextLine();
        return true;
      },
    },
  ],
  parseInline: [
    {
      name: "LongInlineAsText",
      before: "Escape",
      parse: (cx, _, pos) => (pos === cx.offset && cx.end - cx.offset > inlineLimit ? cx.end : -1),
    },
  ],
};

/**
 * markdownEditor is markdownLanguage with bounded: of its data,
 * so that what asks for markdownLanguage (its commands) finds it.
 */
const markdownEditor = new Language(
  markdownLanguage.data,
  (markdownLanguage.parser as MarkdownParser).configure(bounded),
  [],
  "markdown"
);

/**
 * markdownEditing is the editor's Markdown (M4/P6 design 3.2): CommonMark
 * and GFM by lezer, without markdown()'s languages for HTML, CSS and
 * JavaScript, so that code blocks are not highlighted inside; an inline
 * text longer than inlineLimit, a line deeper than blockDepth, is plain.
 * Enter continues a list or a quote; Backspace at its mark takes the mark
 * away.
 */
export function markdownEditing(): Extension {
  return [
    new LanguageSupport(markdownEditor),
    syntaxHighlighting(markdownHighlight),
    Prec.high(
      keymap.of([
        { key: "Enter", run: insertNewlineContinueMarkup },
        { key: "Backspace", run: deleteMarkupBackward },
      ])
    ),
  ];
}
