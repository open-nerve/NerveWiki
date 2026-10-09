import { titleKey } from "./title-key";

/**
 * The name an upload goes out with (M7/P4 design 3.4): a file's name fixed
 * by the rules an import fixes it by (M7 design 4.11), then one no sibling
 * and no upload out has, so that the server takes it as it is.
 */

/** maxNameBytes is how long a name is at most, in UTF-8: a title's. */
const maxNameBytes = 255;

/** Characters a name may not hold: a title's (the server's shared.CheckTitle). */
const forbidden = /[/\\:*?"<>|#^[\]]/g;

/**
 * Characters that show nothing: control characters, the line and paragraph
 * separators, and the bidirectional controls (the server's unshowable).
 */
const unshowable = /[\p{Cc}\p{Zl}\p{Zp}\p{Bidi_Control}]/gu;

/** Windows' device names, alone or before an extension, in any case. */
const reserved = /^(CON|PRN|AUX|NUL|COM[1-9¹²³]|LPT[1-9¹²³])$/i;

const encoder = new TextEncoder();

/**
 * fixedName is raw, a file's name, as a name of the notebook: NFC; each
 * character a title may not hold an underscore; without the blanks and
 * dots around it; a name Windows reserves with an underscore after its
 * stem; at most 255 bytes, cut at a character, its extension kept;
 * untitled when nothing is left.
 */
export function fixedName(raw: string, untitled: string): string {
  let name = raw.normalize("NFC").replace(forbidden, "_").replace(unshowable, "_");
  name = trimmed(name);
  if (name === "") {
    return untitled;
  }
  const dot = name.indexOf(".");
  const stem = dot < 0 ? name : name.slice(0, dot);
  if (reserved.test(stem)) {
    name = `${stem}_${name.slice(stem.length)}`;
  }
  return fitted(name, "");
}

/**
 * freeName is name, or the first of "stem 2.ext", "stem 3.ext", … that none
 * of taken has, compared by their title keys, as a title is among its
 * siblings (M7 design 4.11).
 */
export function freeName(name: string, taken: Iterable<string>): string {
  const used = new Set(Array.from(taken, titleKey));
  if (!used.has(titleKey(name))) {
    return name;
  }
  for (let n = 2; ; n++) {
    const numbered = fitted(name, ` ${n.toString()}`);
    if (!used.has(titleKey(numbered))) {
      return numbered;
    }
  }
}

/** isPageName tells whether name ends with ".md", in any case: a page's file, which an upload may not be. */
export function isPageName(name: string): boolean {
  return name.toLowerCase().endsWith(".md");
}

/** extensionOf is name's extension, its last dot on, or "" for a name without one (a dot that starts it is none). */
export function extensionOf(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(dot) : "";
}

/**
 * fitted is name with suffix after its stem, its stem cut at a character
 * so that the whole is at most maxNameBytes in UTF-8, its extension kept;
 * an extension longer than that is cut as well, after the stem's first
 * character: a name does not start with its extension's dot.
 */
function fitted(name: string, suffix: string): string {
  let extension = extensionOf(name);
  let stem = name.slice(0, name.length - extension.length);
  const first = bytes(Array.from(stem)[0] ?? "");
  if (first + bytes(extension) + bytes(suffix) > maxNameBytes) {
    extension = cut(extension, maxNameBytes - bytes(suffix) - first);
  }
  stem = cut(stem, maxNameBytes - bytes(suffix) - bytes(extension));
  return stem + suffix + extension;
}

/** cut is s cut to at most max bytes in UTF-8, at a character, without the blanks and dots it would end with then. */
function cut(s: string, max: number): string {
  if (bytes(s) <= max) {
    return s;
  }
  let out = "";
  let size = 0;
  for (const character of s) {
    size += bytes(character);
    if (size > max) {
      break;
    }
    out += character;
  }
  return trimmed(out, false);
}

/**
 * trimmed is s without the blanks and dots it ends with, and those it
 * starts with unless not asked to, a character at a time: a pattern
 * anchored at the end would try every run of them, a time the square of
 * their length.
 */
function trimmed(s: string, start = true): string {
  const characters = Array.from(s);
  let from = 0;
  let end = characters.length;
  if (start) {
    while (from < end && blankOrDot(characters[from] ?? "")) {
      from += 1;
    }
  }
  while (end > from && blankOrDot(characters[end - 1] ?? "")) {
    end--;
  }
  return characters.slice(from, end).join("");
}

function blankOrDot(character: string): boolean {
  return character === "." || /^\s$/u.test(character);
}

function bytes(s: string): number {
  return encoder.encode(s).length;
}
