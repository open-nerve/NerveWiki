import { EditorState } from "@codemirror/state";
import { expect, test } from "vitest";

import { lineBreaks, splitBreaks } from "../line-breaks";
import { fenceAt, frontmatterAt, openingQuote, quoted } from "./yaml-quotes";

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
    ["---\n? k\n: '‸", "'"],
    ["---\nup:\n  '‸", "'"],
    ["---\nl: ['[[a]]', \"‸", '"'],
    ["---\nl: [a, '‸", "'"],
    ["---\nm: {a: '[[a]]', b: \"‸", '"'],
    ['---\nm: {"a":\'‸', "'"],
    ["---\nup: &x '‸", "'"],
    ['---\nup: !!str "‸', '"'],
    ["---\n'k''s': \"‸", '"'],
    // A string's whole name tags it too; a string at a line's start opens, in [ ] too (Codex M7 review fix checks
    // A2-M2, A2-N3).
    ["---\nup: !<tag:yaml.org,2002:str> '‸", "'"],
    ["---\nup: !!null\nb: '‸", "'"],
    ["---\nup: [!!null , '‸", "'"],
    ["---\nup: !!seq ['‸", "'"],
    // A key's tag, a collection's, a string's own, and the non-specific ones (Codex M7 review fix check A3-N1).
    ["---\nlist:\n  - !!null\n  - '‸", "'"],
    ["---\na: !!seq\n  - '‸", "'"],
    ["---\n? !!null\n: '‸", "'"],
    ["---\n!!null : '‸", "'"],
    ["---\na: {!!null : '‸", "'"],
    ["---\na: ! '‸", "'"],
    ["---\na: !<!> '‸", "'"],
    // A value's, the key's ':' before it; a ':' in it or after a comment; a key's on another line (fix check A4-M1).
    ["---\n'k': '‸': x", ""],
    ["---\n'k': '‸a:b'", "'"],
    ["---\nup: '‸' # :", "'"],
    ["---\n? k\n: '‸'", "'"],
    // In { } a value's, in [ ] an entry's; a "- " ends a '?' (fix checks A5-M1, A5-N2).
    ["---\nm: {a: '‸'}", "'"],
    ["---\nm: {a: ['‸']}", "'"],
    ["---\nm: {a: [b, '‸']}", "'"],
    ["---\nm: {a: {b: '‸'}}", "'"],
    ["---\nm: [a, '‸']", "'"],
    ["---\nm: [{a: 1}, '‸']", "'"],
    ["---\nl:\n  - ? a\n  - '‸", "'"],
    ["---\nup: '‸\n  a': 1", "'"],
    ["---\nup: '‸\u2028a': 1", "'"],
    ["---\n'k': !!null |\n'‸", "'"],
    ["---\n'k': !!int '1'\n'‸", "'"],
    ["---\na: [\n'‸", "'"],
    ["---\n'‸", "'"],
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
    // A line's start in a string over lines opens nothing (Codex M7 review fix check A-M1).
    ["---\nup: 'a long\n‸", ""],
    // After a tag other than a string's, which the server reads it by (Codex M7 review fix check A2-M2).
    // A key's: after a '?', or closing on its line before a ':' (Codex M7 review fix check A4-M1).
    ["---\n'‸': 1", ""],
    ['---\n"‸" : 1', ""],
    ["---\n'‸a''b': 1", ""],
    ['---\n"‸a\\\"": 1', ""],
    ["---\nm: {'‸': 1}", ""],
    ["---\nm: ['‸':1]", ""],
    ["---\nm:\n  '‸': 1", ""],
    ["---\n? '‸", ""],
    ["---\n?\n  '‸", ""],
    ["---\nm: {? '‸", ""],
    // An entry of { } without a ':' is a key, its value none; a tab before the ':' (fix checks A5-M1, A5-N1).
    ["---\nm: {'‸'}", ""],
    ["---\nm: {a, '‸'}", ""],
    ["---\nm: {'‸', b: 1}", ""],
    ["---\nm: {a: 1,\n  '‸", ""],
    ["---\nm: [{'‸'}]", ""],
    ["---\nm: [a, {b: 1, '‸'}]", ""],
    ["---\nm: {a: [1], '‸", ""],
    ["---\n'‸'\t: 1", ""],
    ["---\nup: !!null '‸", ""],
    ['---\nup: !!null "‸', ""],
    ["---\nup: !!int '‸1'", ""],
    ["---\nup: !!bool '‸true'", ""],
    ["---\nup: !<tag:yaml.org,2002:null> '‸", ""],
    ["---\nup: !!null\n  '‸", ""],
    ["---\n!!null '‸'", ""],
    ["---\nup: [!!float '‸", ""],
    ["---\n- &a !!null '‸", ""],
    ['---\nup: "a\n‸', ""],
    ["---\n- 'a\n  ‸", ""],
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
    // A value alone on its line is of the node of the line before; an anchor or a tag before a key is its node's.
    ["---\nsummary:\n  Notes from the\n  '‸", ""],
    ["---\nlist:\n  -\n    first\n    '‸", ""],
    ["---\n&d summary: Notes from the\n  '‸", ""],
    ["---\nsummary:\n  >\n  folded text\n  '‸", ""],
    ["---\nsummary:\n  >\n  'a\nref: '‸", "'"],
    ["---\nsummary:\n\n  Notes from the\n  '‸", ""],
    ["---\n!!str summary: |\n  text\n  '‸", ""],
    ["---\nnote:\n  Music from the\n  '90s\nref: '‸", "'"],
    // In [ ] a ',' closes none; past a plain string's text a ':' is no value's; a line goes on with what it left.
    ["---\nl: [a, b, '‸", "'"],
    ["---\nl: [k:'‸", ""],
    ["---\nl: [a\n  '‸", ""],
    // A '#' in a plain string's text after no space is text.
    ["---\nC#: '‸", "'"],
    // An empty key's anchor or tag is its node's; in [ ] and { } a '?' is a key's, a space after it or not.
    ["---\n&a : Notes from\n  '‸", ""],
    ["---\nk:\n  !!str : v\n   '‸", ""],
    ["---\nm: {? 'a,': 1}\nn: it's, '‸", ""],
    // The '?' a key's without a space after it: its string a key's, the next an entry's (Codex M7 review fix check A4-M1).
    ["---\nm: [?'‸", ""],
    ["---\nm: [?'k,', '‸", "'"],
    ["---\nm: [? 'k,', '‸", "'"],
    // A key's anchor and tag, the first of them its node; a value alone after a line of its own node.
    ["---\n!!str &a k: v\n '‸", ""],
    ["---\n&a 'k': v\n '‸", ""],
    ["---\nx:\n  y:\nk: |\n '‸", ""],
    ["---\nx:\n  y:\nk: v\n '‸", ""],
    ["---\n?\n  v\n '‸", ""],
    // An anchor's name ends at what is no letter, digit, '-' or '_': a '?' after it starts a plain string.
    ["---\nref: &x?y '‸", ""],
    ["---\nref: &x-1_b '‸", "'"],
    // A document's start: what follows it is the document's.
    ["---\n--- # c\n  k: |\n    '‸", ""],
    ["---\n--- # c\n  k: '‸", "'"],
    // In [ ] and { } a comment ends a plain string; an alias is a node of its own, no plain string's text;
    // a ':' alone at a line's start may have a key after it, its node (fix check A4-M1..A4-M3).
    ["---\nm: [? k # c\n  :\"a, '‸b c'\"]", ""],
    ["---\ntags: [a, b\n# old: ], c\n, \"it's\n'‸90s\"]", ""],
    ["---\nm: [? k # c\n  :'‸", "'"],
    ["---\nm: [? k\n  # c\n  :'‸", "'"],
    ["---\na: &a k\nm: [*a:\"x, '‸y'\"]", ""],
    ["---\nm: {? *a :\"x, '‸y'\"}", ""],
    ["---\nm: {*a:'‸", "'"],
    ["---\nm: {? *a :'‸", "'"],
    ['---\n? x\n: a: v\n  b: "first,\n\'‸second"', ""],
    ['---\nk:\n  ? x\n  : &a : v\n    b: "first,\n  \'‸second"', ""],
    ["---\n? x\n: a: v\n  c: '‸", "'"],
    ["---\n? x\n: a: v\n  '‸c'", "'"],
    ["---\n? x\n: &a : v\n  '‸c'", "'"],
    // An alias in a block is a node of its own, a key's; a comment at a line's start ends a plain string in [ ] (fix check A5-M2).
    ["---\nx: &a v\n*a: |\n  '‸", ""],
    ["---\nx: &a v\nk:\n  *a : v\n   '‸x", ""],
    ['---\nx: &a v\nk:\n  *a : v\n  b: "c,\n\'‸d"', ""],
    ["---\nx: &a v\nk:\n  *a : |\n    x\n  '‸b'", "'"],
    ["---\nm: [? k\n# c\n:'‸", "'"],
    // The library takes the ']' just past a '?' alone in [ ] as the key's and reads on in [ ] what is past it,
    // unlike YAML: no string is told to open there; past a '?' that something else follows, or in { }, they do
    // (fix checks A5-M1, A5-M3, A6-M1, A6-M2).
    ['---\nk: [?], "d,\n  \'‸x"]', ""],
    ["---\nk: [?], '‸", ""],
    ["---\ntags: [?], \"rock,\n  'n roll\"]\nref: '‸", ""],
    ["---\nk: [?], [a: b, c: d, \"e,\n     '‸x\"]]\nref: 'z'", ""],
    ["---\ntags: [?]\n  , a\n  , '‸b'\n]", ""],
    ["---\ntags: [?\n  # c\n  ], a\n  , '‸b'\n]", ""],
    ["---\nk: [a, ? ], ]#c: '‸x'", ""],
    ["---\nm: {a: [?]]\n}#c: '‸x'", ""],
    ["---\nm: [? k]\nref: '‸", "'"],
    ["---\nm: [?x]\nref: '‸", "'"],
    ["---\nm: {?}\nref: '‸", "'"],
    ["---\nm: [{?}]\nref: '‸", "'"],
    ["---\nm: [? :]\nref: '‸", "'"],
    ['---\ntags: [?\n  # c\n  ], "rock,\n  \'‸n roll"]', ""],
    ["---\nk: ?]\nref: '‸", "'"],
    ['---\ntags: [?\t], "rock,\n  \'‸n roll"]', ""],
    ["---\nm: [?, ]\nref: '‸", "'"],
    ["---\nm: [?,, '‸", "'"],
  ] as const;
  expect(openingAt(docs)).toEqual(docs);
});

test("a line ends at YAML's other line breaks too: U+0085, U+2028, U+2029", () => {
  const [nel, ls, ps] = [0x85, 0x2028, 0x2029].map((c) => String.fromCodePoint(c));
  const docs = [
    [`---\n# c${nel}ref: '‸`, "'"],
    [`---\n# c${ls}ref: "‸`, '"'],
    [`---\n# c${ps}ref: '‸`, "'"],
    [`---\nup: 'a${ps}next: '‸`, ""],
  ] as const;
  expect(openingAt(docs)).toEqual(docs);
});

test("written in a string of a quote: in single quotes a ' twice, in double quotes a \\ and a \" escaped", () => {
  const s = String.raw`Bob's "Hi" a\nb`;
  expect(quoted("'", s)).toBe(String.raw`Bob''s "Hi" a\nb`);
  expect(quoted('"', s)).toBe(String.raw`Bob's \"Hi\" a\\nb`);
  expect(quoted("", s)).toBe(s);
});

/** stateOf is an editor's state of raw, its line breaks kept. */
function stateOf(raw: string): EditorState {
  const split = splitBreaks(raw);
  return EditorState.create({ doc: split.text, extensions: lineBreaks(split) });
}

/** read is, for each line of raw (or of a state), frontmatterAt and fenceAt at its start, as the editor keeps the line breaks. */
function read(raw: string | EditorState) {
  const state = typeof raw === "string" ? stateOf(raw) : raw;
  return Array.from({ length: state.doc.lines }, (_, i) => {
    const at = state.doc.line(i + 1).from;
    return [frontmatterAt(state, at) ?? "-", fenceAt(state, at)];
  });
}

/** enter is the state of raw after a line break typed at at. */
function enter(raw: string, at: (state: EditorState) => number): EditorState {
  const state = stateOf(raw);
  return state.update({ changes: { from: at(state), insert: "\n" } }).state;
}

test('a frontmatter\'s lines "---" end where the server ends a line, LF or CRLF: a CR alone goes on with the line (Codex M7 review fix check A-M3)', () => {
  const closed = [
    ["-", true],
    ["closed", false],
    ["-", true],
    ["-", false],
  ];
  expect(read("---\na: ''\n---\nBody")).toEqual(closed);
  expect(read("---\r\na: ''\r\n---\r\nBody")).toEqual(closed);
  // CRs alone: no frontmatter; one alone after the closing line, or before it, closes none: the one being written goes on.
  expect(read("---\ra: ''\r---\rBody")).toEqual(Array.from({ length: 4 }, () => ["-", false]));
  const open = [
    ["-", true],
    ["open", false],
    ["open", false],
    ["open", false],
  ];
  expect(read("---\na: ''\n---\rBody")).toEqual(open);
  expect(read("---\na: ''\r---\nBody")).toEqual(open);
  // A first line "---" with nothing after, or "--- ", opens none.
  expect(read("---")).toEqual([["-", false]]);
  expect(read("--- \na: ''\n---")).toEqual(Array.from({ length: 3 }, () => ["-", false]));
  // A CR alone ending the content closes: the server takes it off the closing line, whose empty line after it is a
  // fence's too (Codex M7 review fix check A2-M1).
  const tail = [...closed.slice(0, 3), ["-", true]];
  expect(read("---\na: ''\n---\r")).toEqual(tail);
  expect(read("---\r\na: ''\r\n---\r")).toEqual(tail);
  // Not when more than the CR is left on the server's line: a CR, or a CRLF, after it.
  const after = [
    ["-", true],
    ["open", false],
    ["open", false],
    ["-", false],
    ["-", false],
  ];
  expect(read("---\na: ''\n---\r\r\nBody")).toEqual(after);
  expect(read("---\na: ''\n---\r\rBody")).toEqual(after);
  // The first "---" a CR alone ends, or follows, closes none; a later one does; "--- " none (fix check A2-N3).
  expect(read("---\na: ''\r---\nb: x\n---\nBody")).toEqual([
    ["-", true],
    ["closed", false],
    ["closed", false],
    ["closed", false],
    ["-", true],
    ["-", false],
  ]);
  expect(read("---\na: x\n--- \nBody")).toEqual(open);

  // Line breaks the editor writes apart that the server reads as one: a CR alone, then an empty line an LF ends, typed
  // there (fix checks A3-M1, A3-N2). A CRLF opens; a CRLF, or the CR alone ending the content, closes.
  expect(read("---\r")).toEqual([
    ["-", false],
    ["-", false],
  ]);
  expect(read("---\rtitle: x\nimage: ''\n---\nBody\n")).toEqual(Array.from({ length: 6 }, () => ["-", false]));
  expect(read(enter("---\rtitle: x\nimage: ''\n---\nBody\n", (state) => state.doc.line(2).from))).toEqual([
    ["-", true],
    ["-", true],
    ["closed", false],
    ["closed", false],
    ["-", true],
    ["-", false],
    ["-", false],
  ]);
  // Not closed, being written after it (an LF the content's main line break, as typed).
  expect(read(enter("---\rimage: ''\nnote: x\nmore: y", (state) => state.doc.line(2).from))).toEqual([
    ["-", true],
    ["-", true],
    ["open", false],
    ["open", false],
    ["open", false],
  ]);
  expect(read(enter("---\na: ''\n---\r", (state) => state.doc.length))).toEqual([
    ["-", true],
    ["closed", false],
    ["-", true],
    ["-", true],
    ["-", false],
  ]);
});
