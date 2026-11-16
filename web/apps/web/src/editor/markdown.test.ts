import { markdownLanguage } from "@codemirror/lang-markdown";
import { ensureSyntaxTree } from "@codemirror/language";
import { EditorSelection, EditorState } from "@codemirror/state";
import { EditorView, runScopeHandlers } from "@codemirror/view";
import { InlineContext, type InlineParser, type MarkdownConfig, type MarkdownParser } from "@lezer/markdown";
import { expect, test } from "vitest";

import { blockDepth, bounded, inlineLimit, linearScans, markdownEditing, referenceLimit } from "./markdown";

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
  // Found by M12/P4/S5's design and its reviews. Those with an ending have it at length, the limit in the test below:
  // cut there, they would lose it and be cheap (review B's M1). Those ending in a ">" end in "x>", never in a "?>",
  // which would close the last "<?" (check B's C1-N1).
  ["a www. URL ending in parentheses", (length) => `www.a.b/${")".repeat(length - 8)}`],
  ["an http:// URL ending in parentheses", (length) => `http://a.b/${")".repeat(length - 11)}`],
  ["an https:// URL ending in parentheses", (length) => `https://a.b/${")".repeat(length - 12)}`],
  [
    "a URL ending in entities",
    (length) => {
      const k = Math.floor((length - 8) / 3);
      return `www.a.b/${"x".repeat(length - 8 - 3 * k)}${"&a;".repeat(k)}`;
    },
  ],
  [
    "a URL ending in entities and parentheses",
    (length) => {
      const k = Math.floor((length - 8) / 4);
      return `www.a.b/${"x".repeat(length - 8 - 4 * k)}${"&a;)".repeat(k)}`;
    },
  ],
  ["emphasis marks of both kinds", (length) => "*_".repeat(length / 2)],
  ["brackets closing none, and emphasis marks", (length) => "]*".repeat(length / 2)],
  [
    "processing instructions, a > at the end",
    (length) => `${`x${"<?".repeat(Math.floor((length - 3) / 2))}`.padEnd(length - 1, "x")}>`,
  ],
  [
    "e-mail local parts after <?, a > at the end",
    (length) => `${`x${"<?@".repeat(Math.floor((length - 3) / 3))}`.padEnd(length - 1, "x")}>`,
  ],
  [
    "e-mail local parts of a letter after <?, a > at the end",
    (length) => `${`x${"<?a@".repeat(Math.floor((length - 3) / 4))}`.padEnd(length - 1, "x")}>`,
  ],
  // lezer's own square: the scans hand it back, a ">" being after each "<" (check B's C1-M1).
  [
    "HTML autolinks not closed, a > after a space at the end",
    (length) => {
      const k = Math.floor((length - 3) / 4);
      return `${"<ab:".repeat(k)}${"a".repeat(length - 2 - 4 * k)} >`;
    },
  ],
  [
    "URLs refused for an _ in their last two domain words",
    (length) => {
      const k = Math.floor((length - 2) / 6);
      return `${"www.a.".repeat(k)}${"a".repeat(length - 2 - 6 * k)}x_`;
    },
  ],
  [
    "URLs refused, each after a hyphen",
    (length) => {
      const k = Math.floor((length - 4) / 6);
      return `${"-www.a".repeat(k)}${"a".repeat(length - 4 - 6 * k)}._.b`;
    },
  ],
  [
    "URLs refused, a hyphen in their words",
    (length) => {
      const k = Math.floor((length - 2) / 6);
      return `${"www.a-".repeat(k)}${"a".repeat(length - 2 - 6 * k)}._`;
    },
  ],
];

/** parsed is how long the editor takes to parse doc whole, in ms. */
function parsed(doc: string): number {
  const started = performance.now();
  const state = EditorState.create({ doc, extensions: markdownEditing() });
  expect(ensureSyntaxTree(state, doc.length, 10_000)?.length).toBe(doc.length);
  return performance.now() - started;
}

// Each took seconds to enter the edit in a paragraph of a few hundred KB: 50,000 links 9.5 s, 96 KB of spaces
// 5.6 s (M6 closeout B-I1, FB-I2, FB2-I1). Past inlineLimit a paragraph is plain text; up to it, the costliest,
// emphasis marks of both kinds, takes about 62-67 ms a parse in Chromium (M12/P4/S5).
test.each(costly)("a paragraph of 200,000 characters, %s, is parsed in a time as long as it", (_, paragraph) => {
  expect(parsed(paragraph(200_000))).toBeLessThan(1_000);
});

test.each(costly)("a paragraph of inlineLimit characters, %s, is parsed in under a second", (_, paragraph) => {
  const doc = paragraph(inlineLimit).slice(0, inlineLimit);
  expect(parsed(doc)).toBeLessThan(1_000);
});

const bare = markdownLanguage.parser as MarkdownParser;
const scansOnly = bare.configure(linearScans);

/**
 * Each cost the square of its length in lezer, one scan with linearScans (M12/P4/S5 design 3.1). 200,000 long, or
 * as long as lezer, without the scans, takes well over a second (the design's 6.2): the e-mail local parts of a
 * letter took 905 ms at 200,000 (check C's C1-M1), 5.4 s at 400,000.
 */
const linear: [string, (length: number) => string, number?][] = [
  ["a run of spaces", (length) => `x${" ".repeat(length - 2)}y`],
  // Ending the text: HardBreak counted it from each space to there as well. Without it, the scan handing such a run
  // back went unseen (M12/P4/S5's implementation).
  ["a run of spaces at its end", (length) => `x${" ".repeat(length - 1)}`],
  ["an escape, then a run of spaces", (length) => `\\*${" ".repeat(length - 3)}y`],
  ["a line with a pipe, then spaces", (length) => `a\nb|c\n${" ".repeat(length - 7)}x`],
  ["processing instructions", (length) => `x${"<?".repeat(length / 2 - 1)}`],
  ["processing instructions, a > at the end", (length) => `x${"<?".repeat(length / 2 - 2)}a>`],
  ["e-mail local parts after <?, a > at the end", (length) => `x${"<?@".repeat(length / 3 - 1)}>`],
  [
    "e-mail local parts of a letter after <?, a > at the end",
    (length) => `x${"<?a@".repeat(length / 4 - 1)}>`,
    400_000,
  ],
  ["autolinks not closed", (length) => "<http://a".repeat(length / 9)],
  ["a www. URL ending in parentheses", (length) => `www.a.b/${")".repeat(length - 8)}`],
  ["an http:// URL ending in parentheses", (length) => `http://a.b/${")".repeat(length - 11)}`],
  ["an https:// URL ending in parentheses", (length) => `https://a.b/${")".repeat(length - 12)}`],
  ["a URL ending in entities", (length) => `www.a.b/${"&a;".repeat(length / 3)}`],
  ["a URL ending in entities and parentheses", (length) => `www.a.b/${"&a;)".repeat(length / 4)}`],
  ["URLs refused for an _ in their last two domain words", (length) => `${"www.a.".repeat(length / 6)}x_`],
  ["URLs refused, each after a hyphen", (length) => `${"-www.a".repeat(length / 6)}._.b`],
  ["URLs refused, a hyphen in their words", (length) => `${"www.a-".repeat(length / 6)}._`],
];

// Without inlineLimit, so that only the scans keep them linear; lezer without them took 1.8-76 s here (the design's
// 6.2). A second is the bound of the tests above: the scans' cost is 1-53 ms, the square's is seconds.
test.each(linear)(
  "with the limits off, a paragraph of %s is parsed in a time as long as it",
  (_, paragraph, length = 200_000) => {
    const text = paragraph(length);
    const started = performance.now();
    scansOnly.parse(text);
    expect(performance.now() - started).toBeLessThan(1_000);
  }
);

/**
 * Each costs the square of its length in lezer itself, the scans or not (M12/P4/S5 design 4.1 item 2, the second):
 * the scans must not add to it. Each long enough that lezer takes some 20-65 ms, so that a share the scans add shows
 * above the 2 ms; with the most its parse with the scans may take, times its parse without, the two having come
 * alike to 0.87-1.18 times, and a fix of review A's M2 undone taking them 1.7-2.4 times:
 * - URLs in a link not closed: half over. In about one run of the whole file in ten, its parse with the scans is
 *   some 15% slower all through the run (1.12-1.18 times; why, not looked into; check C's C3-M1). Counting the
 *   parentheses of each URL at once, not at its first ")" trimmed, takes it 2.1-2.4 times;
 * - URLs refused before a path: a quarter over. A refused URL handed back to Autolink, which matches it again, takes
 *   it 1.7-1.75 times, this its only catch;
 * - URLs then a semicolon: a quarter over. An entity's "&" looked for back to the text's start takes it 2-2.1 times;
 * - the HTML autolinks: a quarter over. The scans hand them back: only a check that lezer's own shapes are no slower.
 */
const lezerOwn: [string, (length: number) => string, number, number][] = [
  ["URLs in a link not closed", (length) => "[www.a.b/".repeat(length / 9), 20_000, 1.5],
  ["URLs refused for an _, each before a path", (length) => `x ${"www.a.b_/".repeat(length / 9)}`, 20_000, 1.25],
  ["URLs, each then a semicolon", (length) => "www.a.b/; ".repeat(length / 10), 60_000, 1.25],
  [
    "HTML autolinks not closed, a > after a space at the end",
    (length) => `${"<ab:".repeat(length / 4)} >`,
    20_000,
    1.25,
  ],
];

/** The fastest of seven parses of text by each parser, by turns: the machine's ups and downs fall on both alike. */
function fastestOfEach(first: MarkdownParser, second: MarkdownParser, text: string): [number, number] {
  let [fastestFirst, fastestSecond] = [Infinity, Infinity];
  for (let i = 0; i < 7; i++) {
    let started = performance.now();
    first.parse(text);
    fastestFirst = Math.min(fastestFirst, performance.now() - started);
    started = performance.now();
    second.parse(text);
    fastestSecond = Math.min(fastestSecond, performance.now() - started);
  }
  return [fastestFirst, fastestSecond];
}

test.each(lezerOwn)(
  "a paragraph of %s is parsed no slower with the scans than without",
  (_, paragraph, length, most) => {
    const text = paragraph(length);
    const [without, withScans] = fastestOfEach(bare, scansOnly, text);
    expect(withScans).toBeLessThan(most * without + 2);
  }
);

test("the editor's parser takes a run of spaces at once", () => {
  // The paragraph inlineLimit long: within the limit, where only the scan keeps it linear.
  const doc = `intro\n\nx${" ".repeat(inlineLimit - 2)}y\n`;
  const char = InlineContext.prototype.char;
  let reads = 0;
  InlineContext.prototype.char = function (pos: number) {
    reads++;
    return char.call(this, pos);
  };
  try {
    ensureSyntaxTree(EditorState.create({ doc, extensions: markdownEditing() }), doc.length, 10_000);
  } finally {
    InlineContext.prototype.char = char;
  }
  // Read at least once through: none read, and it is not the InlineContext the editor's parser has. Twice, as
  // CodeMirror parses the paragraph across its first 3,000 characters again (the design's 1.3); lezer read it 50
  // million times.
  expect(reads, "the editor's parser reads its paragraph through InlineContext.char").toBeGreaterThanOrEqual(
    inlineLimit
  );
  expect(reads).toBeLessThanOrEqual(4 * inlineLimit);
});

/** Every node of doc's tree by parser, its name, from and to. */
function treeOf(parser: MarkdownParser, doc: string): string {
  const parts: string[] = [];
  parser.parse(doc).iterate({ enter: (node) => void parts.push(`${node.name}:${node.from}-${node.to}`) });
  return parts.join(" ");
}

/** How many times the random documents took each branch of the scans, as the wrapped scans tell them apart. */
const taken = new Map<string, number>();
const tally = (branch: string) => taken.set(branch, (taken.get(branch) ?? 0) + 1);
/** Per inline text: where the last URL refused began. */
const lastRefused = new WeakMap<InlineContext, number>();
const urlRE = /[\w-]+(\.[\w-]+)+(:\d+)?(\/[^\s<]*)?/y;

/** Which branch a scan took at pos, by what it answered. */
function tell(name: string, cx: InlineContext, next: number, pos: number, result: number): void {
  const at = pos - cx.offset;
  const text = cx.text;
  if (name === "SpacesAsText" && next === 32) {
    tally(result === -1 ? "spaces: a hard break, handed back" : result - pos >= 2 ? "spaces: a run" : "spaces: one");
  } else if (name === "UnclosedHTMLAsText" && next === 60 && at < text.length - 1) {
    if (result === pos + 1) {
      tally(text.lastIndexOf(">") <= at ? "<: no > ahead" : "<?: no ?> ahead, no e-mail");
    } else if (text[at + 1] === "?") {
      tally(text.lastIndexOf("?>") >= at + 2 ? "<?: its ?> ahead, handed back" : "<?: an e-mail, handed back");
    } else {
      tally("<: a > ahead, handed back");
    }
  } else if (name === "URLAutolink" && (next === 119 || next === 104)) {
    if (result === -1) {
      if (at > 0 && /\w/.test(text[at - 1] ?? "")) {
        tally("url: after a word character");
      }
      return;
    }
    if (result === pos + 1) {
      const before = lastRefused.get(cx);
      // Refused again on the domain of the last refused: the refusal kept.
      const again = before !== undefined && /^[\w.-]*$/.test(text.slice(before, at));
      tally(again ? "url: refused again, kept" : "url: refused");
      lastRefused.set(cx, at);
      return;
    }
    const prefix = text.startsWith("www.", at) ? "www." : text.startsWith("https://", at) ? "https://" : "http://";
    tally(`url: ${prefix}`);
    urlRE.lastIndex = at + prefix.length;
    const matched = urlRE.exec(text);
    const trimmed = matched === null ? "" : text.slice(result - cx.offset, at + prefix.length + matched[0].length);
    if (trimmed.includes(")")) {
      tally("url: a ) trimmed");
    }
    if (/[?!.,:*_~]/.test(trimmed)) {
      tally("url: a mark trimmed");
    }
    if (trimmed.includes("&")) {
      tally("url: an entity trimmed");
    }
    if (cx.hasOpenLink) {
      tally("url: in a link not closed");
    }
  }
}

/** A scan wrapped to tell its branches, its name and place as they are. */
function telling(parser: InlineParser): InlineParser {
  return {
    name: parser.name,
    before: parser.before,
    after: parser.after,
    parse: (cx: InlineContext, next: number, pos: number) => {
      const result = parser.parse(cx, next, pos);
      tell(parser.name, cx, next, pos, result);
      return result;
    },
  };
}

/** One table of tokens: the marks the scans look at, their edges, and blocks around them. */
const tokens = [
  [" ", " ", " ", "  ", "\n", "\n", "\n\n", "\t", "<", "<", ">", ">", "?", "?>", "<?", "<?@", "<?a@", "!", "-", "--"],
  ["<!--", "-->", "[", "]", "<![CDATA[", "]]>", "w", "h", "www.", "www.", "http://", "https://", "-www.", "[www."],
  ["a", "b", "x", "x_", "_", ".", "/", "(", ")", "))", ",", "&", ";", "&a;", "&a;)", "&#1;", "&#xa;", "&#x1f;", "#"],
  ["@", "a@b.c", "a.b@c.d", "c.d>", "+", "`", "*", "\\", ":", ":1", "~", "|", "<a ", "</a>", "<a>", "<!A", "mailto:"],
  ["[a](", "](", "<http://a>", "?a", "a.b", "-a", "x-", "- ", "> ", "# ", "\u3000"],
  // Whole URLs and autolinks, so that their rarer branches come often enough to count.
  ["www.a.b/(", "www.a.b/)", "www.a.b/c.", "www.a.b/c,", "www.a.b/&a;", "http://a.b/)", "https://a_b.c", "www.a.b_"],
  ["<?a@b.c>", "<?x?>"],
].flat();

/** By hand, each in a line's middle ("<?" at a line's start is an HTML block): the edges the random documents meet
 * too seldom, and a document for each condition copied from lezer (review C's I2). */
const edges = [
  // "<?" and an e-mail autolink.
  ["a <?b@c.d>", "x <?a@b.c> y", "a <!b@c.d>", "x <?a@b.c y>", "a <?b@c.d", "a <?x?>", "a <?>", "x <?@a>"],
  ["x <?@<?@b.c>", "x <?a.b@c.d>", "x <?a.b+c@d.e>", "x <?a@b-.c>", "x <?a@-b.c>", "x <?a@b.c.>", "x <?a@b.c d>"],
  // CDATA, left to HTMLTag.
  ["a <![CDATA[x]]>", "a <![CDATA[x>"],
  // A URL's end, trimmed.
  ["www.a.b/(x))", "www.a.b/((x)))", "www.a.b/x)(", "(www.a.b/x)", "www.a.b/&a;&b;)", "www.a.b/(x)&a;)"],
  ["http://a.b/c)).", "https://a.b/c)", "x www.a.b/c&#x1f;", "x www.a.b/c&#1;", "x www.a.b/c&#X1f;"],
  ["x www.a.b/c&a b;", "x www.a.b/c,", "x www.a.b/c?!.,:*_~", "x www.a.b/c;", "x www.a.b/c&;"],
  // Where a URL may begin; one refused, and the starts after it.
  ["xwww.a.b/c", "_www.a.b", "x www.a_b.c", "x www.a.b_", "x www.a_.b", "x www.a.b_/c", "x www.a.b_:1"],
  ["x www.a.b_ www.c.d", "www.a.www.b_ www.c.d", "-www.a-www.a._.b www.c.d", "www.a.www.a.x_ http://c.d"],
  ["x http://a_.b.c", "x https://a.b_", "x www.a.b_.www.c.d"],
  // A URL in a link not closed.
  ["[x www.a.b/c](d)", "[www.a.b/c] d", "[a www.a.b/c](d) e", "[www.a.b/c", "[x www.a.b/c](d"],
  // Spaces.
  ["x  \ny", "x \ny", "x   ", "\\  \nx", "x  y  \nz"],
].flat();

/**
 * The fewest times the 20,000 random documents take each branch: two thirds of what they took when this was written,
 * rounded down (as decision 479 has it). A change to the generator or the tokens takes them all anew.
 */
const fewest: Record<string, number> = {
  "spaces: a run": 9_794,
  "spaces: one": 53_932,
  "spaces: a hard break, handed back": 187,
  "<: no > ahead": 15_960,
  "<: a > ahead, handed back": 64_188,
  "<?: its ?> ahead, handed back": 17_069,
  "<?: an e-mail, handed back": 5_382,
  "<?: no ?> ahead, no e-mail": 10_644,
  "url: www.": 31_044,
  "url: http://": 6_352,
  "url: https://": 1_359,
  "url: after a word character": 86_732,
  "url: refused": 43_268,
  "url: refused again, kept": 1_820,
  "url: a ) trimmed": 5_463,
  "url: a mark trimmed": 10_660,
  "url: an entity trimmed": 4_376,
  "url: in a link not closed": 12_222,
};

test("the linear scans change no tree: with them and without, the editor's parser gives every node alike", () => {
  const without = bare.configure([bounded]);
  const told: MarkdownConfig = { parseInline: (linearScans.parseInline ?? []).map(telling) };
  const withScans = bare.configure([bounded, told]);
  // 32 bits: in a double, seed * 1103515245 went past 2^53, and 20,000 documents were 164 (review C's I1).
  let seed = 20261116;
  const random = () => (seed = (Math.imul(seed, 1103515245) + 12345) & 0x7fffffff) / 0x80000000;
  const seen = new Set<string>();
  const differ: string[] = [];
  taken.clear();
  for (let i = 0; i < 20_000; i++) {
    let doc = "";
    for (let k = 1 + Math.floor(random() * 200); k > 0; k--) {
      doc += tokens[Math.floor(random() * tokens.length)];
    }
    seen.add(doc);
    if (treeOf(without, doc) !== treeOf(withScans, doc)) {
      differ.push(doc);
    }
  }
  const randomly = new Map(taken);
  for (const doc of [...linear.map(([, paragraph]) => paragraph(600)), ...edges]) {
    if (treeOf(without, doc) !== treeOf(withScans, doc)) {
      differ.push(doc);
    }
  }
  expect(seen.size).toBeGreaterThanOrEqual(19_900);
  for (const [branch, least] of Object.entries(fewest)) {
    expect(randomly.get(branch) ?? 0, branch).toBeGreaterThanOrEqual(least);
  }
  expect(differ.slice(0, 5), `${differ.length} differ`).toEqual([]);
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

test("a link reference definition over lines is one up to referenceLimit, one of one line past inlineLimit too; one still open past referenceLimit is plain text; a table past inlineLimit keeps its rows and cells", () => {
  expect(count('[a]: http://x "t"\n\ntext', "LinkReference")).toBe(1);
  expect(count('[a]:\n  http://x\n  "t"', "LinkReference")).toBe(1);
  const open = `[a]: http://x "${"t\n".repeat(referenceLimit / 2)}"`;
  expect([count(open, "LinkReference"), count(open, "Paragraph")]).toEqual([0, 1]);
  // Its lines are counted before the next is read: the line that ends it may cross the limit (M6 closeout FB4-N1).
  expect(count(ending(referenceLimit), "LinkReference")).toBe(1);
  expect(count(ending(referenceLimit + 1), "LinkReference")).toBe(0);
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
    // A tab before the quote's mark (M6 closeout FB5-N1).
    "- a\n\t> x\n\t> b | c\n\t> --|--\n\t> d | e",
  ]) {
    expect([count(doc, "Table"), count(doc, "TableRow")], doc).toEqual([1, 1]);
  }
  expect(count("a\nb | c\n--|--|--\nd | e", "Table")).toBe(0);
  expect(count("a\nb | c\n-- x|--\nd | e", "Table")).toBe(0);
  // A row of dashes with no pipe is no delimiter line.
  expect([count("|b\n:-:\n|c", "Table"), count("|b\n:-:\n|c", "Paragraph")]).toEqual([0, 1]);
});

// Whether a paragraph's line with a pipe heads a table, lezer matched the next line whole with a pattern that tried
// its leading spaces every way: 80,000 spaces took 4.3 s to enter the edit and as long a keystroke (M6 closeout
// FB4-I1). The patch of @lezer/markdown has it take them one way.
test.each<[string, (length: number) => string]>([
  ["spaces", (length) => `a\nb|c\n${" ".repeat(length)}x`],
  ["spaces only", (length) => `a\nb|c\n${" ".repeat(length)}`],
  ["tabs", (length) => `a\nb|c\n${"\t".repeat(length)}x`],
  ["ideographic spaces", (length) => `a\nb|c\n${"\u3000".repeat(length)}x`],
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
