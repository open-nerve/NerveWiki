/**
 * A wiki link or an embed as the server reads one (server/internal/platform/markdown/obsidian/wikilink.go): where it
 * is, its brackets and an embed's "!" with it; its target, anchor and display text, each trimmed of spaces and tabs;
 * where its target and anchor are written, trimmed; where its "#" and its "|" are, the "\" of a "\|" (a table's) with
 * the bar.
 */
export type WikiLink = {
  from: number;
  to: number;
  embed: boolean;
  target: string;
  anchor: string;
  display: string;
  targetAt: { from: number; to: number };
  anchorAt: { from: number; to: number } | undefined;
  hash: number | undefined;
  bar: { from: number; to: number } | undefined;
};

/** What a wiki link's brackets hold: from "[[" to the first "]]" of the line, with no other bracket between. */
const brackets = /!?\[\[([^[\]\n]*)\]\]/g;

/** blank is the white space the server trims a target, an anchor and a display text of. */
const blank = /^[ \t]*|[ \t]*$/g;

/** trimmed is where text, at offset, is once its spaces and tabs at both ends are trimmed. */
function trimmed(text: string, offset: number): { from: number; to: number } {
  const lead = /^[ \t]*/.exec(text)?.[0].length ?? 0;
  const kept = text.replace(blank, "");
  return { from: offset + lead, to: offset + lead + kept.length };
}

/** escaped tells whether the character at at in text is escaped: an odd count of "\" before it. */
function escaped(text: string, at: number): boolean {
  let count = 0;
  for (let i = at - 1; i >= 0 && text[i] === "\\"; i--) {
    count++;
  }
  return count % 2 === 1;
}

/**
 * wikiLinks is the wiki links and embeds of text, which starts at offset, as the server reads them: none whose first
 * "[" is escaped ("\[[a]]" is text; "\![[a]]" a link, its "!" text), none with neither a target nor an anchor. Code
 * and addresses, which hold no link, are the caller's to leave out.
 */
export function wikiLinks(text: string, offset = 0): WikiLink[] {
  const found: WikiLink[] = [];
  for (const match of text.matchAll(brackets)) {
    let from = match.index;
    let embed = match[0].startsWith("!");
    if (embed && escaped(text, from)) {
      embed = false;
      from++;
    }
    if (escaped(text, from + (embed ? 1 : 0))) {
      continue;
    }
    const inner = from + (embed ? 3 : 2);
    const body = match[1] ?? "";
    const bar = body.indexOf("|");
    const backslashed = bar > 0 && body[bar - 1] === "\\";
    const targetEnd = bar < 0 ? body.length : backslashed ? bar - 1 : bar;
    const hash = body.slice(0, targetEnd).indexOf("#");
    const targetText = body.slice(0, hash < 0 ? targetEnd : hash);
    const anchorText = hash < 0 ? "" : body.slice(hash + 1, targetEnd);
    const target = targetText.replace(blank, "");
    const anchor = anchorText.replace(blank, "");
    if (target === "" && anchor === "") {
      continue;
    }
    found.push({
      from: offset + from,
      to: offset + match.index + match[0].length,
      embed,
      target,
      anchor,
      display: bar < 0 ? "" : body.slice(bar + 1).replace(blank, ""),
      targetAt: trimmed(targetText, offset + inner),
      anchorAt: hash < 0 ? undefined : trimmed(anchorText, offset + inner + hash + 1),
      hash: hash < 0 ? undefined : offset + inner + hash,
      bar: bar < 0 ? undefined : { from: offset + inner + (backslashed ? bar - 1 : bar), to: offset + inner + bar + 1 },
    });
  }
  return found;
}

/** A size, as an embed's display text ends with it: a width, or a width "x" a height. */
const size = /^[0-9]+(?:x[0-9]+)?$/;

/** targetShown is what a link to target#anchor shows without a display text: "target > anchor", as Obsidian does. */
function targetShown({ target, anchor }: WikiLink): string {
  if (target === "") {
    return anchor;
  }
  return anchor === "" ? target : `${target} > ${anchor}`;
}

/**
 * An attachment's name: an extension, not a page's ".md", and no space in it: "v1.2 notes" is a page's (review A's
 * P4S3A-F1-N2, B's P4S3B-F2-N1).
 */
const attachment = /\.(?!md$)[^./\s]+$/i;

/**
 * shownOf is the text the reading view shows of link: a wiki link's display text, or its target and anchor; an
 * attachment's embed its caption, its display text but a size at its end, or its target and anchor (which an image
 * shows as its alt); a page's embed, its target of no extension but ".md", its target and anchor, as the server shows
 * one (wikilink.go's shown). The editor knows not which targets are found: one of an extension is taken for an
 * attachment.
 */
export function shownOf(link: WikiLink): string {
  if (!link.embed) {
    return link.display === "" ? targetShown(link) : link.display;
  }
  if (!attachment.test(link.target)) {
    return targetShown(link);
  }
  const last = link.display.lastIndexOf("|");
  const caption = size.test(link.display.slice(last + 1).replace(blank, ""))
    ? link.display.slice(0, Math.max(last, 0)).replace(blank, "")
    : link.display;
  return caption === "" ? targetShown(link) : caption;
}
