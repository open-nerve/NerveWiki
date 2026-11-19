import { syntaxTree } from "@codemirror/language";
import type { EditorState } from "@codemirror/state";

/** A node of the editor's syntax tree. */
type Node = ReturnType<typeof syntaxTree>["topNode"];

/**
 * inTable tells whether pos is in a table as the server finds one
 * (goldmark v1.8.6's extension.Table, table.go), which lezer's GFM tables
 * do not always: goldmark takes a header of fewer cells than its delimiter
 * row, splits a header's cells only at a '|' no backslash is just before,
 * and finds a table's header at any line of a paragraph, the one before
 * its first delimiter row, the table going on to the paragraph's end. A
 * header of more cells than that delimiter row makes no table, nor does
 * the paragraph look further. A paragraph's lines are read as goldmark has
 * them: past the marks of the quotes (a callout's too) and the indent of
 * the list items it is in, to the column its first line starts at (a tab
 * across it leaving the columns past it), a lazy line's as written (M6/P7
 * fix check fc3-c1-2; M12/P2/S5, 12.8). A setext heading lezer makes of a
 * paragraph and the underline after it, goldmark makes a table of first,
 * if the paragraph has one, each of its lines trimmed: the underline is not
 * the table's. Spaces are ASCII's.
 */
export function inTable(state: EditorState, pos: number): boolean {
  for (let node: Node | null = syntaxTree(state).resolveInner(pos, -1); node; node = node.parent) {
    if (node.name === "Table") {
      return true;
    }
    if (node.name === "Paragraph" || node.name === "SetextHeading1" || node.name === "SetextHeading2") {
      return inParagraphTable(state, node, state.doc.lineAt(pos).number, node.name !== "Paragraph");
    }
  }
  return false;
}

/**
 * inParagraphTable tells whether the line numbered at of paragraph is in a
 * table goldmark finds in it; of a setext heading's, the lines before its
 * underline, trimmed.
 */
function inParagraphTable(state: EditorState, paragraph: Node, at: number, setext: boolean): boolean {
  const head = state.doc.lineAt(paragraph.from);
  const first = head.number;
  const last = state.doc.lineAt(paragraph.to).number - (setext ? 1 : 0);
  const quoted = within(paragraph, "Blockquote");
  const listed = within(paragraph, "ListItem");
  // A list item's column, past the marks of the quotes it is in: the first line's, past the last of them.
  const before = state.doc.sliceString(head.from, paragraph.from);
  const mark = before.lastIndexOf(">");
  const column = mark < 0 ? widthOf(before) : widthOf(before.slice(mark + 1).replace(/^[ \t]/, ""));
  const lineOf = (n: number): string => {
    const line = state.doc.line(n);
    if (n === first) {
      return state.doc.sliceString(paragraph.from, line.to);
    }
    // At the top level a line is as written: a delimiter row indented more than 3 is none.
    let text = line.text;
    if (quoted) {
      text = text.replace(/^(?:[ \t]*>[ \t]?)*/, "");
    }
    if (setext) {
      return text.replace(/^[ \t]+/, "");
    }
    return listed ? pastColumn(text, column) : text;
  };
  for (let n = first + 1; n <= last; n++) {
    const delimiter = delimiterCells(lineOf(n));
    if (delimiter === undefined) {
      continue;
    }
    return rowCells(lineOf(n - 1)) <= delimiter && at >= n - 1 && at <= last;
  }
  return false;
}

/** within tells whether node is in a node named name. */
function within(node: Node, name: string): boolean {
  for (let up = node.parent; up; up = up.parent) {
    if (up.name === name) {
      return true;
    }
  }
  return false;
}

/** A delimiter row's cell, by goldmark's four expressions (tableDelimLeft and the others): its spaces ASCII's. */
const delimiterCell = /^[ \t]*:?-+:?[ \t]*$/;

/**
 * delimiterCells is how many cells line has as goldmark's delimiter row
 * (isTableDelim, parseDelimiter): indented at most 3, of nothing but
 * spaces, '-', '|' and ':', not of '-' alone; split at each '|', a blank
 * first and last part dropped, each part ":?-+:?" in spaces. Undefined for
 * a line that is none.
 */
function delimiterCells(line: string): number | undefined {
  if (indentWidth(line) > 3 || !/^[ \t\-|:]*$/.test(line) || /^-*$/.test(line)) {
    return undefined;
  }
  const cells = line.split("|");
  if (blank(cells[0])) {
    cells.shift();
  }
  if (blank(cells.at(-1))) {
    cells.pop();
  }
  return cells.every((cell) => delimiterCell.test(cell)) ? cells.length : undefined;
}

/** blank tells whether a delimiter row's part, before its first '|' or after its last, is of spaces alone. */
function blank(part: string | undefined): boolean {
  return part !== undefined && /^[ \t]*$/.test(part);
}

/**
 * rowCells is how many cells line has as goldmark's header row
 * (parseRow): trimmed of ASCII's spaces, a '|' at each end dropped, split
 * at each '|' no backslash is just before.
 */
function rowCells(line: string): number {
  let row = line.replace(/^[ \t]+|[ \t]+$/g, "");
  if (row.startsWith("|")) {
    row = row.slice(1);
  }
  if (row.endsWith("|")) {
    row = row.slice(0, -1);
  }
  let cells = 0;
  for (let at = 0; at < row.length; cells++) {
    let end = at;
    while (end < row.length && !(row[end] === "|" && (end === 0 || row[end - 1] !== "\\"))) {
      end++;
    }
    at = end + 1;
  }
  return cells;
}

/** indentWidth is the width of line's leading spaces and tabs, a tab to the next multiple of 4. */
function indentWidth(line: string): number {
  let width = 0;
  for (const c of line) {
    if (c === " ") {
      width++;
    } else if (c === "\t") {
      width += 4 - (width % 4);
    } else {
      break;
    }
  }
  return width;
}

/** widthOf is the width of text, a tab to the next multiple of 4. */
function widthOf(text: string): number {
  let width = 0;
  for (const c of text) {
    width += c === "\t" ? 4 - (width % 4) : 1;
  }
  return width;
}

/**
 * pastColumn is line past the leading spaces and tabs within its first
 * columns; a tab that crosses the last of them leaves the columns past it,
 * as spaces: goldmark's padding, which a row's indent counts.
 */
function pastColumn(line: string, columns: number): string {
  let width = 0;
  let at = 0;
  for (; at < line.length && (line[at] === " " || line[at] === "\t") && width < columns; at++) {
    width += line[at] === "\t" ? 4 - (width % 4) : 1;
  }
  return " ".repeat(Math.max(width - columns, 0)) + line.slice(at);
}
