import { EditorState } from "@codemirror/state";
import { expect, test } from "vitest";

import { openingQuote, quoted } from "./yaml-quotes";

// The quote of the YAML string a frontmatter's link starts (Codex review
// R1): where a link completes, and what the completion escapes a page's
// link and an alias by. Each shape as the server's YAML library reads it.

/** openingAt is openingQuote at the ‸ in each doc, by doc. */
function openingAt(docs: readonly (readonly [string, string])[]) {
  return docs.map(([doc]) => {
    const at = doc.indexOf("‸");
    return [doc, openingQuote(EditorState.create({ doc: doc.replace("‸", "") }), at)];
  });
}

test("a quote opens a string where a key or a value starts: after ': ' (a tab too), '- ', '? ', a ': ' alone, in [ ] or { }, after an anchor or a tag, after a quoted key", () => {
  const docs = [
    ["---\nup: '‸", "'"],
    ['---\nup: "‸', '"'],
    ["---\nup:\t'‸", "'"],
    ["---\n- '‸", "'"],
    ["---\n- k: '‸", "'"],
    ["---\n? '‸", "'"],
    ["---\n? k\n: '‸", "'"],
    ["---\nup:\n  '‸", "'"],
    ["---\nl: ['[[a]]', \"‸", '"'],
    ["---\nl: [a, '‸", "'"],
    ["---\nm: {a: '[[a]]', b: \"‸", '"'],
    ['---\nm: {"a":\'‸', "'"],
    ["---\nup: &x '‸", "'"],
    ['---\nup: !!str "‸', '"'],
    ["---\n'k''s': \"‸", '"'],
  ] as const;
  expect(openingAt(docs)).toEqual(docs);
});

test("a quote opens none in a string in quotes, a plain string, a comment or a block's lines, nor where it ends a string", () => {
  const docs = [
    ["---\nup: \"it's '‸", ""],
    ["---\nup: 'say \"‸", ""],
    ['---\nup: "a\\" \'‸', ""],
    ["---\nup: 'a'' '‸", ""],
    ["---\nup: it'‸", ""],
    ["---\nup:'‸", ""],
    ["---\nup: a:b '‸", ""],
    ["---\nup: a#b '‸", ""],
    ["---\n# it's '‸", ""],
    ["---\n# a: '‸", ""],
    ["---\nup: x # a: '‸", ""],
    ["---\nup: x\t# a: '‸", ""],
    ["---\nnote: |\n  it's '‸", ""],
    ["---\nnote: >-\n  '‸", ""],
    ["---\nnote: | # c\n  '‸", ""],
    ["---\nnote: &x |\n  '‸", ""],
    ["---\nnote: |\n  x\n\n    '‸", ""],
    ["---\n- note: |\n    '‸", ""],
    ["---\n- |\n  '‸", ""],
    ["---\n? k\n: |\n  '‸", ""],
    ["---\nnote: a\n  '‸", ""],
  ] as const;
  expect(openingAt(docs)).toEqual(docs);
});

test("what a line leaves goes on to the next: a string in quotes, a block, a plain string's lines, [ ] and { }; past them a quote opens a string again", () => {
  const docs = [
    ["---\nup: \"first\n  '‸", ""],
    // Where it opened on its line is no place on the next one.
    ["---\nup: \"x\n    '‸", ""],
    ["---\nup: 'first\n  \"‸", ""],
    ["---\nup: 'a''b\nnext: '‸", ""],
    ['---\nup: "a\\"b\nnext: \'‸', ""],
    ["---\nup: 'one'\nnext: '‸", "'"],
    ["---\nnote: |\n  x\nup: '‸", "'"],
    ["---\n- note: |\n    x\n  up: '‸", "'"],
    ["---\n- 'k': |\n    x\n  up: '‸", "'"],
    ["---\n- - |\n    x\n  - '‸", "'"],
    ["---\nl: [a, b]\nnote: |\n  '‸", ""],
    ["---\nnote: Music from the\n  '90s\nref: '‸", "'"],
    ["---\nnote: pick one of\n  [a or b\nlist:\n  - '‸", "'"],
    ["---\n- some\n  'text\n- '‸", "'"],
    ["---\nnote: a\n\n  'b\nref: '‸", "'"],
    ["---\nup: a, 'b\nnext: '‸", "'"],
    ["---\nl: [\n  a,\n  '‸", "'"],
    ["---\nl: [a\n  'b', '‸", "'"],
    ["---\nl: [#c\n  '‸", "'"],
    ["---\nup: x # a: 'b\nnext: \"‸", '"'],
  ] as const;
  expect(openingAt(docs)).toEqual(docs);
});

test("a line ends at YAML's other line breaks too: U+0085, U+2028, U+2029", () => {
  const docs = [
    ["---\n# c\u0085ref: '‸", "'"],
    ['---\n# c ref: "‸', '"'],
    ["---\nup: 'a next: '‸", ""],
  ] as const;
  expect(openingAt(docs)).toEqual(docs);
});

test("written in a string of a quote: in single quotes a ' twice, in double quotes a \\ and a \" escaped", () => {
  const s = String.raw`Bob's "Hi" a\nb`;
  expect(quoted("'", s)).toBe(String.raw`Bob''s "Hi" a\nb`);
  expect(quoted('"', s)).toBe(String.raw`Bob's \"Hi\" a\\nb`);
  expect(quoted("", s)).toBe(s);
});
