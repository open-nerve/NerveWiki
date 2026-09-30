// The server's rule for a new password that can be checked here: the length
// of its NFKC form in UTF-16 code units, the form and the count the server
// judges (it hashes that form: full-width letters and decomposed accents are
// the same password as their plain twins). Whether it is too common only the
// server knows.
const minPassword = 8;
const maxPassword = 128;

/** passwordLength is what is wrong with the length of a new password, if anything. */
export function passwordLength(password: string): "too_short" | "too_long" | undefined {
  const { length } = password.normalize("NFKC");
  if (length < minPassword) {
    return "too_short";
  }
  return length > maxPassword ? "too_long" : undefined;
}
