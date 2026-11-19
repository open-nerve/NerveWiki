/** A part of a text: one of its hits, or what lies between them. */
export type TextPart = { text: string; hit: boolean };

/** The text's clusters: a letter and its marks, as the browser draws them together. */
const graphemes = new Intl.Segmenter(undefined, { granularity: "grapheme" });

/**
 * partsOf cuts text into the hits ranges name and the text between them
 * (M8/P5 design 4.3): ranges are the server's, from start up to end, which
 * it does not hold, in UTF-16 code units as JavaScript's strings count
 * them, sorted and apart, as the contract says; the page does not find
 * hits itself (the server's case folding is not the browser's). A range
 * past the text's end is cut at it, and one left empty is none. A range
 * is widened to the clusters it cuts (M5-polish 26.18a): a letter and its
 * marks are drawn together, a mark alone not drawn as one; its end to the
 * next cluster's start, its start to its cluster's, past the hit before.
 */
export function partsOf(text: string, ranges: readonly { start: number; end: number }[]): TextPart[] {
  const parts: TextPart[] = [];
  // Where the text's clusters start, and its end.
  const bounds = [...graphemes.segment(text)].map((segment) => segment.index).concat(text.length);
  let at = 0;
  for (const range of ranges) {
    const start = Math.max(bounds.findLast((bound) => bound <= Math.min(range.start, text.length)) ?? 0, at);
    const end = bounds.find((bound) => bound >= Math.min(range.end, text.length)) ?? text.length;
    if (end <= start) {
      continue;
    }
    if (start > at) {
      parts.push({ text: text.slice(at, start), hit: false });
    }
    parts.push({ text: text.slice(start, end), hit: true });
    at = end;
  }
  if (at < text.length) {
    parts.push({ text: text.slice(at), hit: false });
  }
  return parts;
}
