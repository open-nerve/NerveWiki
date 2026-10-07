import type { EditorState } from "@codemirror/state";

/** The quote of a YAML string: single, double, or none (a plain string, a block's text, a comment). */
export type Quote = "'" | '"' | "";

/**
 * quoteAt is the quote of the YAML string that at, in a frontmatter, is
 * in: read from the frontmatter's second line, after its "---", as far as
 * at (Codex review R1). It reads as much of YAML as tells that: a quote
 * opens a string where a value or a key may start (a line's start, after
 * "- ", "? ", ": ", "[", "{", ",", an anchor or a tag), a string in
 * quotes goes on over lines, a block's header ('|' or '>') makes the
 * lines indented past its key or "-" text, and a '#' after a space starts
 * a comment. A line that goes on a plain string over lines is read as
 * one of its own: a quote at its start opens a string (accepted).
 */
export function quoteAt(state: EditorState, at: number): Quote {
  let quote: Quote = "";
  let block: number | undefined; // the column a block's lines are indented past
  let flow = 0; // how deep in [ ] and { }
  let start = true; // whether a value or a key may start
  for (const line of state.sliceDoc(state.doc.line(2).from, at).split("\n")) {
    let i = 0;
    if (quote === "") {
      i = line.search(/[^ \t]|$/); // YAML indents with spaces; a tab is taken as one
      if (block !== undefined && (i === line.length || i > block)) {
        continue;
      }
      block = undefined;
      start ||= flow === 0;
    }
    let node = i; // where the line's key or "-" is
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
      if (c === "#" && (i === 0 || line[i - 1] === " " || line[i - 1] === "\t")) {
        break;
      }
      if (flow > 0 && c === ",") {
        start = true;
      } else if (flow > 0 && (c === "]" || c === "}")) {
        flow--;
        start = false;
      } else if (!start) {
        start = c === ":" && (spaceNext || (flow > 0 && ",[]{}".includes(next)));
      } else if (c === "'" || c === '"') {
        quote = c;
        start = false;
        node = flow === 0 ? i : node;
      } else if (c === "[" || c === "{") {
        flow++;
      } else if ((c === "-" || c === "?") && flow === 0 && spaceNext) {
        node = i;
      } else if (c === "&" || c === "!") {
        while (i + 1 < line.length && line[i + 1] !== " " && line[i + 1] !== "\t") {
          i++;
        }
      } else if ((c === "|" || c === ">") && flow === 0) {
        block = node;
        break;
      } else {
        start = false;
        node = flow === 0 ? i : node;
      }
    }
  }
  return quote;
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
