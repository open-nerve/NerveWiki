import type { EditorState } from "@codemirror/state";

/** The quote of a YAML string: single or double; none where no string in quotes starts. */
export type Quote = "'" | '"' | "";

/**
 * openingQuote is the quote of the YAML string that at, in a frontmatter,
 * is the first character of, if at is one's: the quote just before it
 * opens a string (Codex review R1). A property link is a string's whole
 * value (M6/P1 rule 10), so a link completes there alone.
 *
 * It reads the YAML from the frontmatter's second line, after its "---", as
 * far as at, as the server's YAML library reads it (go.yaml.in/yaml/v3), as
 * much as tells that: a quote opens a string where a key or a value starts
 * (a line's start, after "- ", "? ", ": ", "[", "{", ",", in them a '?', an
 * anchor or a tag); a string in quotes goes on over lines; an alias is a
 * node of its own; a block's header ('|' or '>') makes the lines indented
 * past its node its text, and a plain string the lines indented past its
 * node, where nothing starts; a '#' starts a comment but in a plain
 * string's text after no space, and in [ ] and { } ends the plain string. A
 * line's node is where its key (its anchor or tag first), "-", "?" or ":"
 * is; a line with none, a value alone, is of the node of the line before
 * that left a value to come (Codex review fix checks A-M1, A2-M1). A line
 * ends at '\n' and at YAML's other line breaks (U+0085, U+2028, U+2029;
 * CodeMirror's lines end at the first alone).
 */
export function openingQuote(state: EditorState, at: number): Quote {
  let quote: Quote = "";
  let opened = -1; // where the string in quotes opened, on the line read
  let block: number | undefined; // the column a block's lines are indented past
  let goesOn: number | undefined; // the column a plain string's lines are indented past
  let flow = 0; // how deep in [ ] and { }
  let start = true; // whether a key or a value may start
  let plain = false; // whether in a plain string's text
  let pending: number | undefined; // the node of the line before, if it left a value to come
  const lines = state.sliceDoc(state.doc.line(2).from, at).split(/[\n\u0085\u2028\u2029]/);
  for (const line of lines) {
    let i = 0;
    let node = 0; // the column of the line's key, "-" or ":"
    let value = false; // whether past the line's ": ", where a string starts a value, not a key
    let key = 0; // where the line's last string that may be a key started
    let own = false; // whether the line has its own node
    let props: number | undefined; // where the anchor or tag before a key started
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
      if (i === 0 && /^---(?:[ \t]|$)/.test(line)) {
        i = 3; // a document's start: what follows it on its line is the document's
      }
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
      if (c === "#" && (!plain || i === 0 || line[i - 1] === " " || line[i - 1] === "\t")) {
        plain &&= flow === 0; // in [ ] and { } a comment ends a plain string: the next line starts anew
        break;
      }
      if (flow > 0 && (c === "," || c === "]" || c === "}")) {
        flow -= c === "," ? 0 : 1;
        start = c === ",";
        plain = false;
      } else if (c === ":" && (spaceNext || (flow > 0 && (!plain || ",[]{}".includes(next))))) {
        // A value's: after a key, or alone at the start (after "? k"), an empty key's anchor or tag before it.
        // Alone it is as "- " and "? ": a key may follow it on the line, its node (fix check A4-M3).
        const alone: boolean = flow === 0 && !value && start && props === undefined;
        if (flow === 0 && !value) {
          node = start ? (props ?? i) : key;
          own = true;
        }
        [start, value, plain] = [true, !alone, false];
      } else if (plain || !start) {
        // A plain string's text, or past a string in quotes.
      } else if (c === "'" || c === '"') {
        [quote, opened, key, start] = [c, i, value ? key : (props ?? i), false];
      } else if (c === "[" || c === "{") {
        flow++;
      } else if ((c === "-" || c === "?") && flow === 0 && spaceNext) {
        node = i;
        value = false;
        own = true;
      } else if (c === "?" && flow > 0) {
        // In [ ] and { } a '?' is a key's, a space after it or not, as the library reads it.
      } else if (c === "&" || c === "!") {
        // An anchor's name is letters, digits, '-' and '_' (what follows it starts anew); a tag goes on to a space.
        props ??= value ? undefined : i;
        const name = c === "&" ? /[\w-]/ : /[^ \t]/;
        while (i + 1 < line.length && name.test(line[i + 1] ?? "")) {
          i++;
        }
      } else if ((c === "|" || c === ">") && flow === 0) {
        block = own ? node : (pending ?? node);
        break;
      } else if (c === "*") {
        // An alias, a node of its own, no plain string's text: its name is an anchor's (fix check A4-M2).
        [start, key] = [false, value ? key : (props ?? i)];
        while (i + 1 < line.length && /[\w-]/.test(line[i + 1] ?? "")) {
          i++;
        }
      } else {
        [plain, start, key] = [true, false, value ? key : (props ?? i)];
      }
    }
    if (plain && flow === 0) {
      goesOn = own ? node : (pending ?? node);
    }
    if (quote === "" && flow === 0 && block === undefined) {
      pending = start ? (own ? node : pending) : undefined;
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
