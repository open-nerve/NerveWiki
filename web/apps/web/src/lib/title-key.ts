/**
 * titleKey is how titles and names compare among siblings, near the
 * server's shared.TitleKey: NFC, full case folding, NFC. Upper case, then
 * lower case, folds as full case folding does where lower case alone does
 * not (ß and SS, the final sigma, the micro sign, ligatures); where the
 * two still differ the server answers 409 page.title_taken.
 */
export function titleKey(title: string): string {
  // Lower case writes a final sigma at the end of a word; folding does not.
  return title.normalize("NFC").toUpperCase().toLowerCase().replaceAll("\u03C2", "\u03C3").normalize("NFC");
}
