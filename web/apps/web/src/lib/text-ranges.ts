/** A part of a text: one of its hits, or what lies between them. */
export type TextPart = { text: string; hit: boolean };

/**
 * partsOf cuts text into the hits ranges name and the text between them
 * (M8/P5 design 4.3): ranges are the server's, from start up to end, which
 * it does not hold, in UTF-16 code units as JavaScript's strings count
 * them, sorted and apart, as the contract says; the page does not find
 * hits itself (the server's case folding is not the browser's). A range
 * past the text's end is cut at it, and one left empty is none.
 */
export function partsOf(text: string, ranges: readonly { start: number; end: number }[]): TextPart[] {
  const parts: TextPart[] = [];
  let at = 0;
  for (const range of ranges) {
    const start = Math.min(range.start, text.length);
    const end = Math.min(range.end, text.length);
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
