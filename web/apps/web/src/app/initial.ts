const graphemes = new Intl.Segmenter(undefined, { granularity: "grapheme" });

/**
 * initialOf is the letter an avatar shows for name: its first grapheme, a letter with its accent or an emoji of
 * several code points whole, upper-cased; nothing for a name of spaces alone (M13/P2/S1, decision 770). The account's
 * round one and the workspace's square one take it alike.
 */
export function initialOf(name: string): string {
  const [first] = graphemes.segment(name.trim());
  return first === undefined ? "" : first.segment.toLocaleUpperCase();
}
