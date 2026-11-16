import { deleteMarkupBackward, insertNewlineContinueMarkup, markdownLanguage } from "@codemirror/lang-markdown";
import { HighlightStyle, Language, LanguageSupport, syntaxHighlighting } from "@codemirror/language";
import { Prec, type Extension } from "@codemirror/state";
import { keymap } from "@codemirror/view";
import { tags } from "@lezer/highlight";
import type { InlineContext, MarkdownConfig, MarkdownParser } from "@lezer/markdown";

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
 * table cell's, in which the editor finds links, emphasis and code, in
 * UTF-16 code units, its line breaks and the marks of the blocks it is in
 * counted; a longer one shows as plain text. Many of lezer's inline
 * parsers scan on from each mark they meet, to the text's end or back to
 * its start: a paragraph's links, a link's address or title not closed,
 * its emphasis closed in part, its marks of emphasis of both kinds one
 * after another each cost the square of their length. 96 KB of spaces took
 * 5.6 s to enter the edit, in one task a writer's content alone decides
 * (M6 closeout B-I1, FB-I2, FB2-I1); a run of spaces, a "<?" and a URL's
 * end cost only their length since (linearScans). At the limit the
 * costliest, "*_" over and over, takes about 60 ms a parse in Chromium
 * (M12/P4/S5 design 1.5). A table keeps its rows and cells, each cell
 * bounded alone: link completion needs them.
 */
export const inlineLimit = 10_000;

/**
 * referenceLimit is how long a paragraph that may be a link reference
 * definition grows, in UTF-16 code units, its line breaks and the marks of
 * the blocks it is in counted, and still is one: once its lines before the
 * next pass it, it is a paragraph (one of one line, or whose last line
 * crosses it, stays one; M6 closeout FB4-N1). lezer's definition parser
 * builds the paragraph's text anew at each line and reads it from its
 * start (LinkReferenceParser.nextLine), so that one of many lines costs the
 * square of its length: a JSON array pasted as text, 20,000 lines, took
 * 0.74 s a keystroke (FB3-I1). Bounded by inlineLimit, a definition whose
 * title never closes took 85-90 ms a character typed in Chromium, and a key
 * in the editor 75-110 ms; at 2,000, some 5 ms and 18-32 ms. The repo's
 * definitions are of one line, 24 code units at most (M12/P4/S5 design
 * 1.1, 1.5, 2.4). A writer sees it in the editor only, and only in a
 * definition over lines, closed or not, that long: it shows as a paragraph,
 * a "[[" in its URL completes (design 3.3).
 */
export const referenceLimit = 2_000;

/**
 * blockDepth is how deep in blocks (lists, their items, quotes) the editor
 * reads a line's structure, the document one of them; the rest of a line
 * deeper is plain text. Each list mark counted the line's columns again
 * from its start: a line of 80,000 "- " took 19 s (M6 closeout FB2-I1).
 * 100 holds 50 lists one in another, or 99 quotes.
 */
export const blockDepth = 100;

/**
 * bounded reads what is past inlineLimit or blockDepth as plain text, and a
 * definition over lines past referenceLimit as a paragraph. Exported for the
 * tests, as the limits are.
 */
export const bounded: MarkdownConfig = {
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
    {
      name: "LongReferenceAsText",
      after: "LinkReference",
      // Made right after LinkReference's: what the leaf has then is that one's.
      leaf: (_, leaf) => {
        const references = [...leaf.parsers];
        if (references.length === 0) {
          return null;
        }
        return {
          nextLine: (_cx, _line, longer) => {
            if (longer.content.length > referenceLimit) {
              longer.parsers = longer.parsers.filter((parser) => !references.includes(parser));
            }
            return false;
          },
          finish: () => false,
        };
      },
    },
  ],
  parseInline: [
    {
      // Before every parser that takes a mark: after emphasis's or code's, a
      // paragraph of their marks alone would never reach it (M6 closeout FB4-N2).
      name: "LongInlineAsText",
      before: "Escape",
      parse: (cx) => (cx.end - cx.offset > inlineLimit ? cx.end : -1),
    },
  ],
};

/** Per inline text: where its last ">" and its last "?>" are, each looked for once. */
const closers = new WeakMap<InlineContext, { gt: number; pi: number }>();

function closersOf(cx: InlineContext): { gt: number; pi: number } {
  let kept = closers.get(cx);
  if (kept === undefined) {
    kept = { gt: cx.text.lastIndexOf(">"), pi: cx.text.lastIndexOf("?>") };
    closers.set(cx, kept);
  }
  return kept;
}

/** HTMLTag's e-mail autolink, from just past its "<" (lezer's :1436): none of its code units is a "<". */
const emailAutolink =
  /[a-z\d.!#$%&'*+/=?^_`{|}~-]+@[a-z\d](?:[a-z\d-]{0,61}[a-z\d])?(?:\.[a-z\d](?:[a-z\d-]{0,61}[a-z\d])?)*>/iy;

/** GFM's Autolink's (lezer's :2193-2195), its www. and http(s):// branches only. */
const urlStart = /(www\.)|(https?:\/\/)/y;
const urlRE = /[\w-]+(\.[\w-]+)+(:\d+)?(\/[^\s<]*)?/y;
const lastTwoDomainWords = /[\w-]+\.[\w-]+($|[/:])/;
const entityAtEnd = /^&(?:#\d+|#x[a-f\d]+|\w+);$/;

/**
 * Per inline text: the last URL refused for an "_" in its last two domain
 * words, from where on its domain, and up to where a later start on that
 * domain comes to the same two words.
 */
const refused = new WeakMap<InlineContext, { from: number; upTo: number }>();

/**
 * lezer's autolinkURLEnd (:2205-2222): where the URL whose domain begins at
 * from ends, or -1 when it is none. A refusal is kept for the starts after
 * it on the same domain; the parentheses are counted at the first ")"
 * trimmed, then counted down; an entity's "&" is looked for back to the
 * URL's start only.
 */
function urlEnd(cx: InlineContext, from: number): number {
  const text = cx.text;
  const kept = refused.get(cx);
  if (kept !== undefined && kept.from < from && from <= kept.upTo) {
    return -1;
  }
  urlRE.lastIndex = from;
  const url = urlRE.exec(text);
  if (url === null) {
    return -1;
  }
  const two = lastTwoDomainWords.exec(url[0]);
  if (two !== null && two[0].includes("_")) {
    refused.set(cx, { from, upTo: from + two.index });
    return -1;
  }
  let end = from + url[0].length;
  let open = -1;
  let close = 0;
  for (;;) {
    // The domain ends in a word's character other than "_" (one with an "_" in its last two words is refused above):
    // what is trimmed never reaches it, so end - 1 is in the text.
    const last = text[end - 1] ?? "";
    if (/[?!.,:*_~]/.test(last)) {
      end--;
      continue;
    }
    if (last === ")") {
      if (open < 0) {
        open = 0;
        for (let i = from; i < end; i++) {
          if (text[i] === "(") {
            open++;
          } else if (text[i] === ")") {
            close++;
          }
        }
      }
      if (close > open) {
        close--;
        end--;
        continue;
      }
      return end;
    }
    if (last === ";") {
      let amp = end - 2;
      while (amp >= from && text[amp] !== "&") {
        amp--;
      }
      if (amp >= from && entityAtEnd.test(text.slice(amp, end))) {
        end = amp;
        continue;
      }
    }
    return end;
  }
}

/**
 * linearScans takes over three of lezer's inline scans that went on from
 * each mark of a run to the text's end, or counted the run again at each
 * step, so that a paragraph of the run cost the square of its length
 * (M12/P4/S5 design 1.2, 3.1). Each comes before lezer's own, hands back
 * (-1) what that one takes, and takes the rest, the run once, as that one
 * would: the tree is lezer's, node for node (markdown.test.ts compares
 * them). Lines are @lezer/markdown 1.7.2's dist/index.js, as patched.
 * - SpacesAsText, before HardBreak (:1471-1481): HardBreak counted a run of
 *   spaces from each of them to its end, a hard break only before a line's
 *   end; one that is none is text here at once. No other inline parser of
 *   lezer's begins at a space.
 * - UnclosedHTMLAsText, before HTMLTag (:1432-1455): each of HTMLTag's
 *   patterns ends in a ">", so a "<" with none after it begins nothing.
 *   A "<?" with no "?>" after it is no processing instruction, which HTMLTag
 *   looked for to the text's end (:1448), but may begin an e-mail autolink
 *   (its local part may begin with "?"): HTMLTag's pattern is tried here
 *   once, and stops at the next "<".
 * - URLAutolink, before GFM's Autolink (:2193-2272), whose www. and
 *   http(s):// branches it copies; e-mail, mailto: and xmpp: stay
 *   Autolink's. Each ")" trimmed off a URL's end counted its parentheses
 *   again (count, :2198); an entity at its end was looked for in all of it;
 *   one refused for an "_" in its last two domain words was matched again
 *   from each start after it on its domain, to the same end, and refused
 *   again. Here the parentheses are counted once, and a refusal is kept: a
 *   later start inside the domain, up to where its last two words begin,
 *   has urlRE match the same domain, port and path and lastTwoDomainWords
 *   take the same two words. One refused is text here: Autolink, at the
 *   same place, takes the same branch and refuses it again, and no inline
 *   parser after it takes a "w" or an "h".
 * The copy is checked again with the patch when @lezer/markdown is updated
 * (v0.1 design 13.5 item 5).
 */
export const linearScans: MarkdownConfig = {
  parseInline: [
    {
      name: "SpacesAsText",
      before: "HardBreak",
      parse: (cx, next, start) => {
        if (next !== 32) {
          return -1;
        }
        let end = start + 1;
        while (cx.char(end) === 32) {
          end++;
        }
        return cx.char(end) === 10 && end >= start + 2 ? -1 : end;
      },
    },
    {
      name: "UnclosedHTMLAsText",
      before: "HTMLTag",
      parse: (cx, next, start) => {
        if (next !== 60 || start === cx.end - 1) {
          return -1;
        }
        const at = start - cx.offset;
        const { gt, pi } = closersOf(cx);
        if (gt <= at) {
          return start + 1;
        }
        if (cx.text.charCodeAt(at + 1) !== 63 || pi >= at + 2) {
          return -1;
        }
        emailAutolink.lastIndex = at + 1;
        return emailAutolink.test(cx.text) ? -1 : start + 1;
      },
    },
    {
      name: "URLAutolink",
      before: "Autolink",
      parse: (cx, next, absPos) => {
        if (next !== 119 && next !== 104) {
          return -1;
        }
        const pos = absPos - cx.offset;
        if (pos > 0 && /\w/.test(cx.text[pos - 1] ?? "")) {
          return -1;
        }
        urlStart.lastIndex = pos;
        const start = urlStart.exec(cx.text);
        if (start === null) {
          return -1;
        }
        let end = urlEnd(cx, pos + start[0].length);
        if (end < 0) {
          return absPos + 1;
        }
        if (cx.hasOpenLink) {
          const noBracket = /([^[\]]|\[[^\]]*\])*/.exec(cx.text.slice(pos, end));
          end = pos + (noBracket?.[0].length ?? 0);
        }
        cx.addElement(cx.elt("URL", absPos, end + cx.offset));
        return end + cx.offset;
      },
    },
  ],
};

/**
 * markdownEditor is markdownLanguage with bounded and linearScans: of its
 * data, so that what asks for markdownLanguage (its commands) finds it. Whether a
 * paragraph's line heads a table, lezer matched the next line with a
 * pattern that cost the square of its leading spaces, past either bound: a
 * patch of @lezer/markdown has it take them one way (pnpm-workspace.yaml,
 * M6 closeout FB4-I1).
 */
const markdownEditor = new Language(
  markdownLanguage.data,
  (markdownLanguage.parser as MarkdownParser).configure([bounded, linearScans]),
  [],
  "markdown"
);

/**
 * markdownEditing is the editor's Markdown (M4/P6 design 3.2): CommonMark
 * and GFM by lezer, without markdown()'s languages for HTML, CSS and
 * JavaScript, so that code blocks are not highlighted inside; an inline
 * text longer than inlineLimit, a line deeper than blockDepth, is plain, a
 * definition over lines longer than referenceLimit a paragraph, and three
 * of lezer's inline scans take a run once (linearScans). Enter continues a
 * list or a quote; Backspace at its mark takes the mark away.
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
