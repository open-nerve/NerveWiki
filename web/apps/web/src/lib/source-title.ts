export type SourceTitleProblem = "required" | "too_long" | "invalid_format" | "not_allowed";

const utf8 = new TextEncoder();
const forbidden = /[/\\:*?"<>|#^[\]]/u;
const unshowable = /[\p{Cc}\p{Zl}\p{Zp}\p{Bidi_Control}]/u;
const reserved = /^(CON|PRN|AUX|NUL|COM[1-9¹²³]|LPT[1-9¹²³])$/i;

/**
 * A source needs its explicit title before it is created: a writer cannot
 * repair it afterward. Check the server's title rules without fixing the
 * name or choosing another one (shared.CheckTitle); the server still has
 * the last word. Only the title is trimmed and composed, never its content.
 */
export function sourceTitleProblem(name: string): SourceTitleProblem | undefined {
  const title = name.trim().normalize("NFC");
  if (title === "") {
    return "required";
  }
  if (utf8.encode(title).length > 255) {
    return "too_long";
  }
  if (forbidden.test(title) || unshowable.test(title) || title.startsWith(".") || title.endsWith(".")) {
    return "invalid_format";
  }
  if (reserved.test(title.split(".", 1)[0] ?? "")) {
    return "not_allowed";
  }
  return undefined;
}
