// The server's rule for a new password that can be checked here: its length
// in UTF-16 code units, as the server counts it. Whether it is too common
// only the server knows.
const minPassword = 8;
const maxPassword = 128;

/** passwordLength is what is wrong with the length of a new password, if anything. */
export function passwordLength(password: string): "too_short" | "too_long" | undefined {
  if (password.length < minPassword) {
    return "too_short";
  }
  return password.length > maxPassword ? "too_long" : undefined;
}
