import { ensureSyntaxTree } from "@codemirror/language";
import { EditorSelection, EditorState } from "@codemirror/state";
import { EditorView, runScopeHandlers } from "@codemirror/view";
import { expect, test } from "vitest";

import { blockDepth, inlineLimit, markdownEditing } from "./markdown";

/** The document after key is pressed with the cursor at the end of doc. */
function press(doc: string, key: string): string {
  const view = new EditorView({
    state: EditorState.create({ doc, selection: EditorSelection.cursor(doc.length), extensions: markdownEditing() }),
    parent: document.body,
  });
  expect(runScopeHandlers(view, new KeyboardEvent("keydown", { key }), "editor")).toBe(true);
  const after = view.state.doc.toString();
  view.destroy();
  return after;
}

test.each([
  ["a bullet", "- item", "- item\n- "],
  ["a number", "1. first", "1. first\n2. "],
  ["a task", "- [ ] task", "- [ ] task\n- [ ] "],
  ["a quote", "> said", "> said\n> "],
])("Enter continues %s", (_, doc, after) => {
  expect(press(doc, "Enter")).toBe(after);
});

test("Enter on an empty item makes room before it, and on that again ends the list", () => {
  expect(press("- item\n- ", "Enter")).toBe("- item\n\n- ");
  expect(press("- item\n\n- ", "Enter")).toBe("- item\n\n");
});

test("Backspace at an item's mark takes the mark away, the item's indent kept", () => {
  expect(press("- item\n- ", "Backspace")).toBe("- item\n  ");
});

/** The links, emphasis and code spans of doc, as the editor parses it. */
function inlineNodes(doc: string): string[] {
  const state = EditorState.create({ doc, extensions: markdownEditing() });
  const names: string[] = [];
  ensureSyntaxTree(state, doc.length, 10_000)?.iterate({
    enter: ({ name }) => void (["Link", "Emphasis", "InlineCode"].includes(name) && names.push(name)),
  });
  return names;
}

/** inline is a text length long that begins with a link, emphasis and code. */
function inline(length: number): string {
  const head = "[a](b) *e* `c` ";
  return head + "x".repeat(length - head.length);
}

test("a paragraph up to inlineLimit has its links, emphasis and code; a longer one, a heading's too, is plain text, and the next is read as ever", () => {
  const all = ["Link", "Emphasis", "InlineCode"];
  expect(inlineNodes(inline(inlineLimit))).toEqual(all);
  expect(inlineNodes(`# ${inline(inlineLimit)}`)).toEqual(all);
  expect(inlineNodes(inline(inlineLimit + 1))).toEqual([]);
  expect(inlineNodes(`# ${inline(inlineLimit + 1)}`)).toEqual([]);
  expect(inlineNodes(`${inline(inlineLimit + 1)}\n\n${inline(20)}`)).toEqual(all);
});

/** A paragraph of each of the shapes about length long whose inline parse costs the square of its length. */
const costly: [string, (length: number) => string][] = [
  ["links", (length) => "[[Page]] ".repeat(length / 9)],
  ["links and brackets that close none", (length) => "[a](b) ] ".repeat(length / 9)],
  ["emphasis that closes none", (length) => "a* ".repeat(length / 3)],
  ["emphasis closed in part", (length) => "*a** ".repeat(length / 5)],
  ["strong emphasis in emphasis", (length) => "***a*** ".repeat(length / 8)],
  ["a run of spaces", (length) => `x${" ".repeat(length - 2)}y`],
  ["an escape, then a run of spaces", (length) => `\\*${" ".repeat(length - 3)}y`],
  ["processing instructions", (length) => `x${"<?".repeat(length / 2 - 1)}`],
  ["images not closed", (length) => "![a](".repeat(length / 5)],
  ["autolinks not closed", (length) => "<http://a".repeat(length / 9)],
  ["link titles not closed", (length) => "[a](b (x ".repeat(length / 9)],
  [
    "code spans of distinct lengths",
    (length) => {
      let text = "";
      for (let ticks = 1; text.length < length; ticks++) {
        text += `${"`".repeat(ticks)}a`;
      }
      return text;
    },
  ],
  ["nested emphasis", (length) => "*a ".repeat(length / 6) + "b* ".repeat(length / 6)],
  ["nested brackets", (length) => "[".repeat(length / 2) + "]".repeat(length / 2)],
  // Each of its marks begins what an inline parser after the limit's takes (M6 closeout FB4-N2).
  ["marks of emphasis and code only", (length) => "*`*`".repeat(length / 4)],
];

/** parsed is how long the editor takes to parse doc whole, in ms. */
function parsed(doc: string): number {
  const started = performance.now();
  const state = EditorState.create({ doc, extensions: markdownEditing() });
  expect(ensureSyntaxTree(state, doc.length, 10_000)?.length).toBe(doc.length);
  return performance.now() - started;
}

// Each took seconds to enter the edit in a paragraph of a few hundred KB: 50,000 links 9.5 s, 96 KB of spaces
// 5.6 s (M6 closeout B-I1, FB-I2, FB2-I1). Past inlineLimit a paragraph is plain text; up to it, the costliest
// takes about 80 ms.
test.each(costly)("a paragraph of 200,000 characters, %s, is parsed in a time as long as it", (_, paragraph) => {
  expect(parsed(paragraph(200_000))).toBeLessThan(1_000);
});

test.each(costly)("a paragraph of inlineLimit characters, %s, is parsed in under a second", (_, paragraph) => {
  const doc = paragraph(inlineLimit).slice(0, inlineLimit);
  expect(parsed(doc)).toBeLessThan(1_000);
});

/** The nodes named name of doc's tree, as the editor parses it, each from where to where. */
function nodes(doc: string, name: string): [number, number][] {
  const state = EditorState.create({ doc, extensions: markdownEditing() });
  const found: [number, number][] = [];
  ensureSyntaxTree(state, doc.length, 10_000)?.iterate({
    enter: (node) => void (node.name === name && found.push([node.from, node.to])),
  });
  return found;
}

const count = (doc: string, name: string) => nodes(doc, name).length;

test("lists in lists, quotes in quotes are read to blockDepth; past it the rest of the line is plain text, and the next is read as ever", () => {
  // A list and its item are two blocks: the document's and blockDepth - 1 more hold half as many items.
  const deepest = blockDepth / 2;
  expect(count(`${"- ".repeat(deepest)}a`, "ListItem")).toBe(deepest);
  const deeper = `${"- ".repeat(deepest + 10)}a`;
  expect(count(deeper, "ListItem")).toBe(deepest);
  expect(nodes(deeper, "Paragraph")).toEqual([[2 * deepest, deeper.length]]);
  expect(count(`${deeper}\n\n- b`, "ListItem")).toBe(deepest + 1);
  // A quote is one: below the document, blockDepth - 1 of them.
  expect(count(`${"> ".repeat(blockDepth + 10)}a`, "Blockquote")).toBe(blockDepth - 1);
});

/** A link reference definition whose title ends on its second line, its first before characters long. */
const ending = (before: number) => `[a]: http://x "${"t".repeat(before - 15)}\nt"\n\ntext`;

test("a link reference definition is one up to inlineLimit, over lines too; one still open past it is plain text; a table past it keeps its rows and cells", () => {
  expect(count('[a]: http://x "t"\n\ntext', "LinkReference")).toBe(1);
  expect(count('[a]:\n  http://x\n  "t"', "LinkReference")).toBe(1);
  const open = `[a]: http://x "${"t\n".repeat(inlineLimit / 2)}"`;
  expect([count(open, "LinkReference"), count(open, "Paragraph")]).toEqual([0, 1]);
  // Its lines are counted before the next is read: the line that ends it may cross the limit (M6 closeout FB4-N1).
  expect(count(ending(inlineLimit), "LinkReference")).toBe(1);
  expect(count(ending(inlineLimit + 1), "LinkReference")).toBe(0);
  expect(count(`[a]: http://x "${"t".repeat(2 * inlineLimit)}"\n\ntext`, "LinkReference")).toBe(1);
  const rows = Array.from({ length: 1_000 }, (_, i) => `| [a](b) | ${i} |`);
  const table = ["| x | y |", "|---|---|", ...rows].join("\n");
  expect(table.length).toBeGreaterThan(inlineLimit);
  expect([count(table, "Table"), count(table, "TableRow"), count(table, "Link")]).toEqual([1, 1_000, 1_000]);
  // One whose head may begin a definition: no longer one, it is a table still.
  const bracketed = ["[x] | y", "---|---", ...rows].join("\n");
  expect([count(bracketed, "Table"), count(bracketed, "TableRow"), count(bracketed, "Link")]).toEqual([
    1, 1_000, 1_001,
  ]);
});

// A paragraph that may be a link reference definition read itself again at each line: a JSON array pasted as
// text, 20,000 lines, took 0.74 s a keystroke; a "[" and 100,000 lines 15 s (M6 closeout FB3-I1).
test.each<[string, (length: number) => string]>([
  ["a bracket never closed", (length) => `[${"\nx".repeat(length / 2)}`],
  ["a title never closed", (length) => `[a]: b "x${"\nx".repeat(length / 2)}`],
  [
    "a JSON array",
    (length) => {
      const lines = ["["];
      for (let i = 0, size = 1; size < length; i++) {
        const line = `  {"id": ${i}, "name": "item ${i}"},`;
        lines.push(line);
        size += line.length + 1;
      }
      return [...lines, "]"].join("\n");
    },
  ],
])(
  "a paragraph of 200,000 characters that may be a link reference definition, %s, is parsed in a time as long as it",
  (_, paragraph) => {
    expect(parsed(paragraph(200_000))).toBeLessThan(1_000);
  }
);

test("a table of 10,000 rows is parsed in a time as long as it", () => {
  const rows = Array.from({ length: 10_000 }, (_, i) => `| row ${i} | [link](http://a.b/${i}) | **b** *e* \`c\` |`);
  expect(parsed(["| x | y | z |", "|---|---|---|", ...rows].join("\n"))).toBeLessThan(1_000);
});

test("a paragraph's line with a pipe heads a table when the next is a row of dashes, in a quote and indented too", () => {
  for (const doc of [
    "a\nb | c\n--|--\nd | e",
    "a\nb | c\n   :--|--:\nd | e",
    "> a\n> b | c\n>  | --- | --- |\n> d | e",
    "- a\n  b | c\n  --|--\n  d | e",
  ]) {
    expect([count(doc, "Table"), count(doc, "TableRow")], doc).toEqual([1, 1]);
  }
  expect(count("a\nb | c\n--|--|--\nd | e", "Table")).toBe(0);
  expect(count("a\nb | c\n-- x|--\nd | e", "Table")).toBe(0);
});

// Whether a paragraph's line with a pipe heads a table, lezer matched the next line whole with a pattern that tried
// its leading spaces every way: 80,000 spaces took 4.3 s to enter the edit and as long a keystroke (M6 closeout
// FB4-I1). The patch of @lezer/markdown has it take them one way.
test.each<[string, (length: number) => string]>([
  ["spaces", (length) => `a\nb|c\n${" ".repeat(length)}x`],
  ["spaces only", (length) => `a\nb|c\n${" ".repeat(length)}`],
  ["tabs", (length) => `a\nb|c\n${"\t".repeat(length)}x`],
  ["ideographic spaces", (length) => `a\nb|c\n${"　".repeat(length)}x`],
  ["spaces in a quote", (length) => `> a\n> b|c\n> ${" ".repeat(length)}x`],
  ["spaces in a list item", (length) => `- a\n  b|c\n  ${" ".repeat(length)}x`],
])("a line of 200,000 %s after a paragraph's line with a pipe is parsed in a time as long as it", (_, doc) => {
  expect(parsed(doc(200_000))).toBeLessThan(1_000);
});

// Each list mark counted the line's columns again from its start: a line of 80,000 "- " took 19 s.
test.each<[string, (length: number) => string]>([
  ["list marks", (length) => `${"- ".repeat(length / 2)}a`],
  ["numbered list marks", (length) => `${"1. ".repeat(length / 3)}a`],
  ["quote and list marks", (length) => `${"> - 1. ".repeat(length / 7)}a`],
])("a line of 200,000 characters of %s is parsed in a time as long as it", (_, line) => {
  expect(parsed(line(200_000))).toBeLessThan(1_000);
});
