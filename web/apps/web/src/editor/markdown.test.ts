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

/** How many nodes named name doc's tree has, as the editor parses it. */
function count(doc: string, name: string): number {
  const state = EditorState.create({ doc, extensions: markdownEditing() });
  let found = 0;
  ensureSyntaxTree(state, doc.length, 10_000)?.iterate({ enter: (node) => void (node.name === name && found++) });
  return found;
}

test("lists in lists, quotes in quotes are read to blockDepth; past it the rest of the line is plain text, and the next is read as ever", () => {
  // A list and its item are two blocks: the document's and blockDepth - 1 more hold half as many items.
  const deepest = blockDepth / 2;
  expect(count(`${"- ".repeat(deepest)}a`, "ListItem")).toBe(deepest);
  expect(count(`${"- ".repeat(deepest + 10)}a`, "ListItem")).toBe(deepest);
  expect(count(`${"- ".repeat(deepest + 10)}a\n\n- b`, "ListItem")).toBe(deepest + 1);
  // A quote is one: below the document, blockDepth - 1 of them.
  expect(count(`${"> ".repeat(blockDepth + 10)}a`, "Blockquote")).toBe(blockDepth - 1);
});

// Each list mark counted the line's columns again from its start: a line of 80,000 "- " took 19 s.
test.each<[string, (length: number) => string]>([
  ["list marks", (length) => `${"- ".repeat(length / 2)}a`],
  ["numbered list marks", (length) => `${"1. ".repeat(length / 3)}a`],
  ["quote and list marks", (length) => `${"> - 1. ".repeat(length / 7)}a`],
])("a line of 200,000 characters of %s is parsed in a time as long as it", (_, line) => {
  expect(parsed(line(200_000))).toBeLessThan(1_000);
});
