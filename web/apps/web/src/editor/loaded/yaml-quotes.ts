import type { EditorState } from "@codemirror/state";

/** The quote of a YAML string: single or double; none where no string in quotes starts. */
export type Quote = "'" | '"' | "";

/**
 * openingQuote is the quote of the YAML string that at, in a frontmatter,
 * is the first character of, if at is one's: the quote just before it
 * opens a string (Codex review R1). A property link is a string's whole
 * value (M6/P1 rule 10), so a link completes there alone.
 *
 * It reads the YAML from the frontmatter's second line, after its "---",
 * as far as at, as the server's YAML library reads it (go.yaml.in/yaml/v3),
 * as much as tells that: a quote opens a string where a key or a value
 * starts (a line's start, after "- ", "? ", ": ", "[", "{", ",", an anchor
 * or a tag); a string in quotes goes on over lines; a block's header ('|'
 * or '>') makes the lines indented past its key, "-" or ":" its text, and
 * a plain string the lines indented past them, where nothing starts; a
 * '#' starts a comment but in a plain string's text after no space. A
 * line ends at '\n' and at YAML's other line breaks (U+0085, U+2028,
 * U+2029; CodeMirror's lines end at the first alone).
 */
export function openingQuote(state: EditorState, at: number): Quote {
  let quote: Quote = "";
  let opened = -1; // where the string in quotes opened, on the line read
  let block: number | undefined; // the column a block's lines are indented past
  let goesOn: number | undefined; // the column a plain string's lines are indented past
  let flow = 0; // how deep in [ ] and { }
  let start = true; // whether a key or a value may start
  let plain = false; // whether in a plain string's text
  const lines = state.sliceDoc(state.doc.line(2).from, at).split(/[\n\u0085\u2028\u2029]/);
  for (const line of lines) {
    let i = 0;
    let node = 0; // the column of the line's key, "-" or ":"
    let value = false; // whether past the line's ": ", where a string starts a value, not a key
    let key = 0; // where the line's last string that may be a key started
    opened = -1;
    if (quote === "") {
      i = line.search(/[^ \t]|$/); // YAML indents with spaces; a tab is taken as one
      const blank = i === line.length;
      if (block !== undefined && (blank || i > block)) {
        continue;
      }
      if (goesOn !== undefined && (blank || (i > goesOn && line[i] !== "#"))) {
        continue;
      }
      block = goesOn = undefined;
      if (flow === 0) {
        start = true;
        plain = false;
      }
      node = i;
    }
    for (; i < line.length; i++) {
      const c = line[i];
      const next = line[i + 1];
      if (quote === "'") {
        if (c === "'" && next === "'") {
          i++;
        } else if (c === "'") {
          quote = "";
        }
        continue;
      }
      if (quote === '"') {
        if (c === "\\") {
          i++;
        } else if (c === '"') {
          quote = "";
        }
        continue;
      }
      const spaceNext = next === undefined || next === " " || next === "\t";
      if (c === " " || c === "\t") {
        continue;
      }
      if (c === "#" && (!plain || line[i - 1] === " " || line[i - 1] === "\t")) {
        break;
      }
      if (flow > 0 && (c === "," || c === "]" || c === "}")) {
        flow -= c === "," ? 0 : 1;
        start = c === ",";
        plain = false;
      } else if (c === ":" && (spaceNext || (flow > 0 && (!plain || ",[]{}".includes(next))))) {
        // A value's: after a key, or alone at the start (after "? k").
        if (flow === 0 && !value) {
          node = start ? i : key;
        }
        start = value = true;
        plain = false;
      } else if (plain || !start) {
        // A plain string's text, or past a string in quotes.
      } else if (c === "'" || c === '"') {
        [quote, opened, key, start] = [c, i, value ? key : i, false];
      } else if (c === "[" || c === "{") {
        flow++;
      } else if ((c === "-" || c === "?") && flow === 0 && spaceNext) {
        node = i;
        value = false;
      } else if (c === "&" || c === "!") {
        while (i + 1 < line.length && line[i + 1] !== " " && line[i + 1] !== "\t") {
          i++;
        }
      } else if ((c === "|" || c === ">") && flow === 0) {
        block = node;
        break;
      } else {
        [plain, start, key] = [true, false, value ? key : i];
      }
    }
    if (plain && flow === 0) {
      goesOn = node;
    }
  }
  const last = lines.at(-1) ?? "";
  return quote !== "" && opened === last.length - 1 ? quote : "";
}

/** quoted is s written in a YAML string of quote: in single quotes a ' twice, in double quotes a \ and a " escaped. */
export function quoted(quote: Quote, s: string): string {
  if (quote === "'") {
    return s.replaceAll("'", "''");
  }
  if (quote === '"') {
    return s.replaceAll("\\", "\\\\").replaceAll('"', String.raw`\"`);
  }
  return s;
}
