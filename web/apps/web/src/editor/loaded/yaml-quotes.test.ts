import { EditorState } from "@codemirror/state";
import { expect, test } from "vitest";

import { quoteAt, quoted } from "./yaml-quotes";

// The quote of the YAML string a frontmatter's link is written in (Codex
// review R1): what the completion escapes a page's link and an alias by.

/** quoteOf is quoteAt the ‸ in doc. */
function quoteOf(doc: string) {
  const at = doc.indexOf("‸");
  return quoteAt(EditorState.create({ doc: doc.replace("‸", "") }), at);
}

test("a quote opens a string where a value or a key starts: after a key's ': ', a list's '- ', in [ ] or { }, after an anchor or a tag; a quoted key's string ends at its quote", () => {
  for (const [doc, quote] of [
    ["---\nup: '‸", "'"],
    ['---\nup: "‸', '"'],
    ["---\n- '‸", "'"],
    ["---\n? '‸", "'"],
    ["---\nup:\n  '‸", "'"],
    ["---\nl: ['[[a]]', \"‸", '"'],
    ["---\nl: [a, '‸", "'"],
    ["---\nm: {a: '[[a]]', b: \"‸", '"'],
    ["---\nm: {a: b}\nup: '‸", "'"],
    ["---\nup: &x '‸", "'"],
    ['---\nup: !!str "‸', '"'],
    ["---\n'k''s': \"‸", '"'],
    ["---\n- k: '‸", "'"],
  ]) {
    expect([doc, quoteOf(doc ?? "")]).toEqual([doc, quote]);
  }
});

test("a string in quotes goes on to its closing quote: one escaped stays in it, another quote is its text, over lines too", () => {
  for (const [doc, quote] of [
    ["---\nup: 'it''s ‸", "'"],
    ["---\nup: \"it's '‸", '"'],
    ["---\nup: 'say \"‸", "'"],
    ['---\nup: "a\\"b ‸', '"'],
    ['---\nup: "a\\\\" ‸', ""],
    ["---\nup: 'a'' b' ‸", ""],
    ["---\nup: \"first\n  it's '‸", '"'],
    ["---\nup: 'first\n  \"‸", "'"],
    ["---\nup: 'one'\nnext: ‸", ""],
  ]) {
    expect([doc, quoteOf(doc ?? "")]).toEqual([doc, quote]);
  }
});

test("a quote in a plain string, a comment or a block's lines is their text; the lines after a block are YAML again", () => {
  for (const [doc, quote] of [
    ["---\nup: it'‸", ""],
    ["---\nup:'‸", ""],
    ["---\nup: a:b '‸", ""],
    ["---\nup: a#b '‸", ""],
    ["---\n# it's '‸", ""],
    ["---\nup: x # it's '‸", ""],
    // A comment's ': ' starts nothing, and its quote opens no string over lines.
    ["---\n# a: '‸", ""],
    ["---\nup: x # a: 'b\nnext: \"‸", '"'],
    ["---\nnote: |\n  it's '‸", ""],
    ["---\nnote: >-\n  '‸", ""],
    ["---\nnote: | # c\n  '‸", ""],
    ["---\nnote: &x |\n  '‸", ""],
    ["---\nnote: |\n  x\n\n    '‸", ""],
    ["---\n- note: |\n    '‸", ""],
    ["---\n- |\n  '‸", ""],
    ["---\nnote: |\n  x\nup: '‸", "'"],
    ["---\n- note: |\n    x\n  up: '‸", "'"],
    ["---\n- 'k': |\n    x\n  up: '‸", "'"],
    ["---\n- - |\n    x\n  - '‸", "'"],
    ["---\nl: [\n  a,\n  '‸", "'"],
    ["---\nl: [a, b]\nnote: |\n  '‸", ""],
  ]) {
    expect([doc, quoteOf(doc ?? "")]).toEqual([doc, quote]);
  }
});

test("written in a string of a quote: in single quotes a ' twice, in double quotes a \\ and a \" escaped; elsewhere as it is", () => {
  const s = String.raw`Bob's "Hi" a\nb`;
  expect(quoted("'", s)).toBe(String.raw`Bob''s "Hi" a\nb`);
  expect(quoted('"', s)).toBe(String.raw`Bob's \"Hi\" a\\nb`);
  expect(quoted("", s)).toBe(s);
});
