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
 * the list items it is in (M6/P7 fix check fc3-c1-2; M12/P2/S5, 12.8).
 */
export function inTable(state: EditorState, pos: number): boolean {
  for (let node: Node | null = syntaxTree(state).resolveInner(pos, -1); node; node = node.parent) {
    if (node.name === "Table") {
      return true;
    }
    if (node.name === "Paragraph") {
      return inParagraphTable(state, node, state.doc.lineAt(pos).number);
    }
  }
  return false;
}

/** inParagraphTable tells whether the line numbered at of paragraph is in a table goldmark finds in it. */
function inParagraphTable(state: EditorState, paragraph: Node, at: number): boolean {
  const first = state.doc.lineAt(paragraph.from).number;
  const last = state.doc.lineAt(paragraph.to).number;
  const quoted = within(paragraph, "Blockquote");
  const listed = within(paragraph, "ListItem");
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
    return listed ? text.replace(/^[ \t]+/, "") : text;
  };
  for (let n = first + 1; n <= last; n++) {
    const delimiter = delimiterCells(lineOf(n));
    if (delimiter === undefined) {
      continue;
    }
    return rowCells(lineOf(n - 1)) <= delimiter && at >= n - 1;
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

/** A delimiter row's cell, by goldmark's four expressions (tableDelimLeft and the others). */
const delimiterCell = /^\s*:?-+:?\s*$/;

/**
 * delimiterCells is how many cells line has as goldmark's delimiter row
 * (isTableDelim, parseDelimiter): indented at most 3, of nothing but
 * spaces, '-', '|' and ':', not of '-' alone; split at each '|', a blank
 * first and last part dropped, each part ":?-+:?" in spaces. Undefined for
 * a line that is none.
 */
function delimiterCells(line: string): number | undefined {
  if (indentWidth(line) > 3 || !/^[\s\-|:]*$/.test(line) || /^-*$/.test(line)) {
    return undefined;
  }
  const cells = line.split("|");
  if (cells[0]?.trim() === "") {
    cells.shift();
  }
  if (cells.at(-1)?.trim() === "") {
    cells.pop();
  }
  return cells.every((cell) => delimiterCell.test(cell)) ? cells.length : undefined;
}

/**
 * rowCells is how many cells line has as goldmark's header row
 * (parseRow): trimmed of spaces, a '|' at each end dropped, split at each
 * '|' no backslash is just before.
 */
function rowCells(line: string): number {
  let row = line.trim();
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
