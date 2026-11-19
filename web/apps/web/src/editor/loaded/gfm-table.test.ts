import { ensureSyntaxTree } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import { expect, test } from "vitest";

import { markdownEditing } from "../markdown";
import { inTable } from "./gfm-table";

// Whether a link being written is in a table as the server (goldmark v1.8.6) finds one (M12/P2/S5, 12.8): each shape
// here was rendered by the server with the link written each way, in the Step's probe (its record).

/** at tells whether "[[Road" typed at ‸ in doc is in a table. */
function at(doc: string): boolean {
  const where = doc.indexOf("‸");
  const typed = `${doc.slice(0, where)}[[Road${doc.slice(where + 1)}`;
  const state = EditorState.create({ doc: typed, extensions: [markdownEditing()] });
  ensureSyntaxTree(state, state.doc.length, 5_000);
  return inTable(state, where + "[[Road".length);
}

test.each<[string, string]>([
  ["a header of fewer cells than its delimiter row", "| a |\n| - | - |\n| x | ‸"],
  ["a header whose \\\\| goldmark does not split at", "| a \\\\| b |\n| - |\n| ‸"],
  ["as many cells: lezer's table too", "| a | b |\n| - | - |\n| ‸ |"],
  ["a delimiter row without a '|'", "a\n:--\n‸"],
  ["a delimiter row of no spaces, its cells aligned", "| a |\n|-|:-:|--:|\n| ‸"],
  ["a delimiter row indented 3", "| a |\n   | - | - |\n| ‸"],
  ["a header after lines of the paragraph", "x\ny\n| a |\n| - | - |\nz ‸"],
  ["a row without a '|', the paragraph's", "| a |\n| - | - |\nplain ‸"],
  ["the header, in a paragraph lezer ends before a table of its own", "text\n| a |\n| - | - |\n| ‸\n| h |\n| - |\n| y"],
  ["a delimiter row indented 5 is none: the next is the table's", "| a | b | c |\n     | - |\n| d |\n| - |\n| ‸"],
  ["in a callout", "> [!note]\n> | a |\n> | - | - |\n> | x | ‸"],
  ["in a quote, a row lazily without its mark", "> | a |\n> | - | - |\n| ‸"],
  ["in a quote in a quote", "> > | a |\n> > | - | - |\n> > | ‸"],
  ["in a list's item", "- | a |\n  | - | - |\n  | ‸"],
  ["in an item of a list in an item", "- a\n    - | a |\n      | - | - |\n      | ‸"],
  ["in a list in a quote", "> - | a |\n>   | - | - |\n>   | ‸"],
  // A setext underline right after: lezer makes a heading of it all, goldmark a table first.
  ["a setext underline after, ---", "| a |\n| - | - |\n| ‸\n---"],
  ["a setext underline after, ===", "| a |\n| - | - |\n| ‸\n==="],
  ["a setext underline after a table of as many cells", "| a | b |\n| - | - |\n| ‸ |\n---"],
  ["a setext underline after, -", "| a |\n| - | - |\n| ‸\n-"],
  ["a setext underline after, the link in the header", "| a ‸ | b |\n| - | - |\n| c | d |\n---"],
  ["a setext underline after, a delimiter row indented 4: goldmark trims the lines", "| a |\n    | - | - |\n| ‸\n---"],
  // goldmark's spaces are ASCII's: a row with another space is no delimiter row, and the paragraph looks further.
  ["a no-break space in a row before the delimiter row", "| a | b | c |\n| -\u00a0| - |\n| x |\n| - | - |\n| ‸"],
  [
    "ideographic spaces in a row before the delimiter row",
    "| a | b | c |\n|\u3000-\u3000|\u3000-\u3000|\n| x |\n| - | - |\n| ‸",
  ],
  // An item's lines are indented to its column: one indented 4 past it is no delimiter row.
  [
    "in a list's item, a row indented 4 past its column before the delimiter row",
    "- | a | b | c |\n      | - |\n  | d |\n  | - | - |\n  | ‸",
  ],
  [
    "in a quote in a list's item, a row indented 4 past the mark before the delimiter row",
    "- > | a | b | c |\n  >     | - |\n  > | d |\n  > | - | - |\n  > | ‸",
  ],
  // A tab that crosses the column leaves the columns past it, as goldmark's padding.
  [
    "in a list's item, a row indented a tab and 3 past its column before the delimiter row",
    "- | a | b | c |\n\t   | - |\n  | d |\n  | - | - |\n  | ‸",
  ],
  // A quote's mark takes one column of a tab after it, which leaves the columns past that.
  [
    "in a quote, a tab after the mark and 2 spaces before the delimiter row",
    ">\t| a | b | c |\n>\t  | - |\n>\t| d |\n>\t| - | - |\n>\t| ‸",
  ],
  [
    "in a list in a quote, a tab after the mark and 4 past the column before the delimiter row",
    "> - | a | b | c |\n>\t    | - |\n>   | d |\n>   | - | - |\n>   | ‸",
  ],
  [
    "in a quote in a quote, a tab after each mark and 2 spaces before the delimiter row",
    "> > | a | b | c |\n>\t>\t  | - |\n> > | d |\n> > | - | - |\n> > | ‸",
  ],
  [
    "in a quote, the mark at column 1, its tab 2 wide and 3 spaces before the delimiter row",
    " > | a | b | c |\n >\t   | - |\n > | d |\n > | - | - |\n > | ‸",
  ],
])("in a table: %s", (_, doc) => {
  expect(at(doc)).toBe(true);
});

test.each<[string, string]>([
  ["a header of more cells than its delimiter row", "| a | b | c |\n| - | - |\n| ‸"],
  ["the line before the header", "‸\n| a |\n| - |"],
  ["the line before a header lezer does not take, the table's paragraph", "‸\n| a |\n| - | - |"],
  ["two lines before such a header", "‸\nx\n| a |\n| - | - |\n| y |"],
  ["after a blank line", "| a |\n| - | - |\n\n‸"],
  ["a setext heading's underline", "| a |\n---\n‸"],
  ["a delimiter row of ':' alone", "| a |\n| : |\n| ‸"],
  ["a delimiter row indented a tab", "| a |\n\t| - | - |\n| ‸"],
  ["a list's item, which ends the paragraph", "a | b\n- | -\n‸"],
  ["a header of more cells, though a delimiter row matches a later one", "| a | b |\n| - |\n| c |\n| - |\n| ‸"],
  ["in a quote, a header of more cells", "> | a | b | c |\n> | - | - |\n> | ‸"],
  ["in a list's item, a header of more cells", "- | a | b | c |\n  | - | - |\n  | ‸"],
  ["a setext heading of a header of more cells", "| a | b | c |\n| - | - |\n| ‸\n---"],
  ["a header ending in a no-break space, a cell more", "| a |\u00a0\n| - |\n| ‸"],
  ["in a list's item, a delimiter row indented 4 past its column", "- | a |\n      | - | - |\n  | ‸"],
  // A tab after a quote's mark is as wide as its column has it.
  [
    "in a quote, the mark at column 2, its tab 1 wide and 2 spaces: a delimiter row",
    "  > | a | b | c |\n  >\t  | - |\n  > | d |\n  > | - | - |\n  > | ‸",
  ],
  [
    "in a quote in a quote, a tab after each mark and 1 space: a delimiter row",
    "> > | a | b | c |\n>\t>\t | - |\n> > | d |\n> > | - | - |\n> > | ‸",
  ],
  [
    "in a list in a quote, a tab after its first line's mark (column 4), a row 2 past it: a delimiter row",
    ">\t- | a | b | c |\n>       | - |\n>     | d |\n>     | - | - |\n>     | ‸",
  ],
])("no table: %s", (_, doc) => {
  expect(at(doc)).toBe(false);
});
