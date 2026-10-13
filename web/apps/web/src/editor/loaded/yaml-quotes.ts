import type { EditorState } from "@codemirror/state";

import { writtenBreak } from "../line-breaks";

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
 * string's text after no space, and ends the plain string. A line's node is
 * where its key (its anchor or tag first), "-", "?" or ":" is; a line with
 * none, a value alone, is of the node of the line before that left a value
 * to come (Codex M6 review fix checks A-M1, A2-M1). A line ends at '\n' and at
 * YAML's other line breaks (U+0085, U+2028, U+2029; CodeMirror's lines end
 * at the first alone). Past a '?' alone in [ ] that a ']' closes, which the
 * library reads unlike YAML, it tells no string to open; nor after a tag
 * other than a string's ("!!str", its whole name, or the non-specific
 * "!"), on its line or a line before with no node between: the server
 * reads such a string by its tag, a value that fits it, which a link
 * would not (Codex M7 review fix checks A2-M2, A3-N1); nor a key's, after
 * a '?' or in { } before its ':' (an entry there with none is a key), or
 * closing on its line before a ':' (keyClose): a key holds no property
 * link, and one written there may make it another key's, or too long (fix
 * checks A4-M1, A5-M1).
 */
export function openingQuote(state: EditorState, at: number): Quote {
  let quote: Quote = "";
  let opened = -1; // where the string in quotes opened, on the line read
  let block: number | undefined; // the column a block's lines are indented past
  let goesOn: number | undefined; // the column a plain string's lines are indented past
  const flow: string[] = []; // the [ and { open, the innermost last
  let start = true; // whether a key or a value may start
  let plain = false; // whether in a plain string's text
  let pending: number | undefined; // the node of the line before, if it left a value to come
  let keyAlone = false; // whether just past a '?' in [ ] or { }, no token after it yet
  let tag = false; // whether a tag other than a string's is before the node to come
  let tagged = false; // whether the string in quotes opened last had such a tag
  let explicit = false; // whether past a '?', before its ':': a key to come
  let keyed = false; // whether the string in quotes opened last is such a key
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
      if (flow.length === 0) {
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
        plain = false; // a comment ends a plain string: the next line is none of its text
        break;
      }
      if (keyAlone && c === "]") {
        // The YAML library takes the ']' after a '?' alone in [ ] as the key's and reads on in [ ] what is
        // past it: no string is told to open there (Codex M6 review fix checks A5-M1, A6-M1, A6-M2).
        return "";
      }
      keyAlone = c === "?" && flow.length > 0;
      if (flow.length > 0 && (c === "," || c === "]" || c === "}")) {
        if (c !== ",") {
          flow.pop();
        }
        start = c === ",";
        plain = false;
        tag = false; // the node it tagged was empty
        // In { } an entry is a key, its ':' and value or none (Codex M7 review fix check A5-M1).
        explicit = c === "," && flow.at(-1) === "{";
      } else if (c === ":" && (spaceNext || (flow.length > 0 && (!plain || ",[]{}".includes(next))))) {
        // A value's: after a key, or alone at the start (after "? k"), an empty key's anchor or tag before it.
        // Alone it is as "- " and "? ": a key may follow it on the line, its node (fix check A4-M3).
        const alone: boolean = flow.length === 0 && !value && start && props === undefined;
        if (flow.length === 0 && !value) {
          node = start ? (props ?? i) : key;
          own = true;
        }
        [start, value, plain, tag, explicit] = [true, !alone, false, false, false]; // a tag before it was its key's
      } else if (plain || !start) {
        // A plain string's text, or past a string in quotes.
      } else if (c === "'" || c === '"') {
        [quote, opened, key, start, tagged, tag, keyed] = [
          c,
          i,
          value ? key : (props ?? i),
          false,
          tag,
          false,
          explicit,
        ];
      } else if (c === "[" || c === "{") {
        flow.push(c);
        tag = false;
        explicit = c === "{";
      } else if ((c === "-" || c === "?") && flow.length === 0 && spaceNext) {
        node = i;
        value = false;
        own = true;
        tag = false; // a tag before it was its collection's (Codex M7 review fix check A3-N1)
        explicit = c === "?";
      } else if (c === "?" && flow.length > 0) {
        // In [ ] and { } a '?' is a key's, a space after it or not, as the library reads it.
        explicit = true;
      } else if (c === "&" || c === "!") {
        // An anchor's name is letters, digits, '-' and '_' (what follows it starts anew); a tag goes on to a space.
        props ??= value ? undefined : i;
        const from = i;
        const name = c === "&" ? /[\w-]/ : /[^ \t]/;
        while (i + 1 < line.length && name.test(line[i + 1] ?? "")) {
          i++;
        }
        if (c === "!") {
          tag = !stringTags.has(line.slice(from, i + 1));
        }
      } else if ((c === "|" || c === ">") && flow.length === 0) {
        block = own ? node : (pending ?? node);
        tag = false;
        break;
      } else if (c === "*") {
        // An alias, a node of its own, no plain string's text: its name is an anchor's (fix check A4-M2).
        [start, key, tag] = [false, value ? key : (props ?? i), false];
        while (i + 1 < line.length && /[\w-]/.test(line[i + 1] ?? "")) {
          i++;
        }
      } else {
        [plain, start, key, tag] = [true, false, value ? key : (props ?? i), false];
      }
    }
    if (plain && flow.length === 0) {
      goesOn = own ? node : (pending ?? node);
    }
    if (quote === "" && flow.length === 0 && block === undefined) {
      pending = start ? (own ? node : pending) : undefined;
    }
  }
  const last = lines.at(-1) ?? "";
  // At a line's start in a string over lines, nothing opened on it (Codex M7 review fix check A-M1).
  if (quote === "" || opened < 0 || opened !== last.length - 1 || tagged || keyed) {
    return "";
  }
  return keyClose(state, at, quote) ? "" : quote;
}

/**
 * keyClose tells whether the string in quote that opens at at closes on
 * its line before a ':', a key's: a key not after a '?' is on one line
 * (Codex M7 review fix check A4-M1).
 */
function keyClose(state: EditorState, at: number, quote: Quote): boolean {
  const line = state.doc.lineAt(at);
  // As far as YAML's other line breaks too (U+0085, U+2028, U+2029).
  const rest = line.text.slice(at - line.from).split(/[\u0085\u2028\u2029]/)[0] ?? "";
  for (let i = 0; i < rest.length; i++) {
    const c = rest[i];
    if (quote === '"' && c === "\\") {
      i++;
    } else if (c === quote && quote === "'" && rest[i + 1] === "'") {
      i++;
    } else if (c === quote) {
      return /^[ \t]*:/.test(rest.slice(i + 1));
    }
  }
  return false;
}

/** The tags of a string: its shorthand, its whole name, and the non-specific ones, which make a scalar in quotes one (Codex M7 review fix check A3-N1). */
const stringTags = new Set(["!!str", "!<tag:yaml.org,2002:str>", "!", "!<!>"]);

/**
 * frontmatterAt tells whether pos is in the content's frontmatter, as the
 * server's frontmatterSpan finds it ("closed"): after its first line
 * "---", up to a later line "---" ("..." closes nothing), as fenceSpan
 * reads them. The content the editor loads has no byte order mark
 * (line-breaks.ts); one pasted at its start is not looked for (accepted).
 * One not closed, which the server reads as the body until it is, is one
 * being written ("open") as far as a blank line: a link or a tag written
 * there as the body's would be nothing once it is closed.
 */
export function frontmatterAt(state: EditorState, pos: number): "closed" | "open" | undefined {
  const { doc } = state;
  const at = doc.lineAt(pos).number;
  const open = opening(state);
  if (open === 0 || at <= open) {
    return undefined;
  }
  const close = closing(state, open + 1);
  if (close !== undefined) {
    return at < close.line ? "closed" : undefined;
  }
  // Line by line, as the text's own iterator goes: a content opened by "---" and never closed is read to its end.
  let n = open;
  for (const text of doc.iterLines(open + 1)) {
    n++;
    if (/^[ \t]*$/.test(text)) {
      return at < n ? "open" : undefined;
    }
  }
  return "open";
}

/**
 * fenceAt tells whether pos is on the lines "---" that open a frontmatter,
 * closed or being written, or that close it, as frontmatterAt finds them:
 * what goes in there, but a line's end at the closing one's end, makes it
 * none (Codex M7 review R1, fix checks A2-M1, A3-M1).
 */
export function fenceAt(state: EditorState, pos: number): boolean {
  const open = opening(state);
  if (open === 0) {
    return false;
  }
  const at = state.doc.lineAt(pos).number;
  const close = at <= open ? undefined : closing(state, open + 1);
  return at <= open || (close !== undefined && at >= close.line && at < close.line + close.span);
}

/**
 * opening is how many of the editor's lines the server reads as the line
 * "---" that opens the content's frontmatter: one ended by an LF or a CRLF,
 * the content going on; 0 for none (Codex M7 review fix checks A-M3,
 * A3-M1).
 */
function opening(state: EditorState): number {
  const span = fenceSpan(state, 1);
  if (span === 1) {
    return writtenBreak(state, 1) === undefined ? 0 : 1;
  }
  // "---", a CR alone, and an empty line an LF ends, written apart: the server reads a CRLF.
  return span === 2 && writtenBreak(state, 2) === "\n" ? 2 : 0;
}

/**
 * closing is the line "---" that closes the frontmatter whose YAML starts
 * at the line from, the first the server reads as one, and how many of
 * the editor's lines it spans; undefined for none.
 */
function closing(state: EditorState, from: number): { line: number; span: number } | undefined {
  let n = from - 1;
  for (const text of state.doc.iterLines(from)) {
    n++;
    const span = text === "---" ? fenceSpan(state, n) : 0;
    if (span > 0) {
      return { line: n, span };
    }
  }
  return undefined;
}

/**
 * fenceSpan is how many of the editor's lines, from its line n, the server
 * reads as one line "---", 0 for none. The editor ends a line at a CR
 * alone, which the server does not (line-breaks.ts keeps how each break is
 * written); the server ends one at an LF and takes off a CR before it.
 * So a line "---" after no CR alone, ended by an LF, a CRLF or the
 * content's end, is one; and with the empty line after it, when a CR
 * alone ends it and that empty line is the content's last or ends at an
 * LF written apart, two (Codex M7 review fix checks A-M3, A2-M1, A3-M1).
 */
function fenceSpan(state: EditorState, n: number): number {
  const { doc } = state;
  if (doc.line(n).text !== "---" || (n > 1 && writtenBreak(state, n - 1) === "\r")) {
    return 0;
  }
  if (writtenBreak(state, n) !== "\r") {
    return 1;
  }
  const then = writtenBreak(state, n + 1);
  return doc.line(n + 1).length === 0 && (then === undefined || then === "\n") ? 2 : 0;
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
