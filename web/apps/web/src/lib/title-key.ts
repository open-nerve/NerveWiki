/**
 * titleKey is how titles and names compare among siblings, near the
 * server's shared.TitleKey: NFC, full case folding, NFC. Upper case, then
 * lower case, folds as full case folding does where lower case alone does
 * not (ß and SS, the final sigma, the micro sign, ligatures); the capital
 * sharp s lowers to ß, which folds to ss. Where the two still differ, the
 * key here is the coarser (the dotless i is i; a few letters newer than
 * the server's tables fold): an upload takes a number it need not. A key
 * finer than the server's would have it answer 409 page.title_taken.
 */
export function titleKey(title: string): string {
  // Lower case writes a final sigma at the end of a word; folding does not.
  return title
    .normalize("NFC")
    .toUpperCase()
    .toLowerCase()
    .replaceAll("\u03C2", "\u03C3")
    .replaceAll("\u00DF", "ss")
    .normalize("NFC");
}
