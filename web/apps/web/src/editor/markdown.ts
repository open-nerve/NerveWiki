import { deleteMarkupBackward, insertNewlineContinueMarkup, markdownLanguage } from "@codemirror/lang-markdown";
import { HighlightStyle, LanguageSupport, syntaxHighlighting } from "@codemirror/language";
import { Prec, type Extension } from "@codemirror/state";
import { keymap } from "@codemirror/view";
import { tags } from "@lezer/highlight";

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
 * markdownEditing is the editor's Markdown (M4/P6 design 3.2): CommonMark
 * and GFM by lezer, without markdown()'s languages for HTML, CSS and
 * JavaScript, so that code blocks are not highlighted inside. Enter
 * continues a list or a quote; Backspace at its mark takes the mark away.
 */
export function markdownEditing(): Extension {
  return [
    new LanguageSupport(markdownLanguage),
    syntaxHighlighting(markdownHighlight),
    Prec.high(
      keymap.of([
        { key: "Enter", run: insertNewlineContinueMarkup },
        { key: "Backspace", run: deleteMarkupBackward },
      ])
    ),
  ];
}
