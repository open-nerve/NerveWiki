import { ensureSyntaxTree } from "@codemirror/language";
import { EditorState, StateEffect } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, beforeEach, expect, test, vi } from "vitest";

import { editOutline, where, type OutlineHeading, type OutlineSink } from "./edit-outline";
import { markdownEditing } from "./markdown";
import { wikiLinks } from "./wiki-links";

// The outline while the page is edited (M13/P4/S3, decision 876): the editor's headings, read as it is made, again once
// its changes settle, the parse completed in slices. Parse.work times its slices by Date.now(), which fake timers stop,
// so that a slice never runs out: a slice cut short is the stub's (review C's M8). A timer of no delay set as fake
// timers run one is due 1 ms on: the slices go a millisecond each.

vi.mock("@codemirror/language", async (actual) => ({
  ...(await actual<typeof import("@codemirror/language")>()),
  ensureSyntaxTree: vi.fn(),
}));
const language = await vi.importActual<typeof import("@codemirror/language")>("@codemirror/language");
const ensured = vi.mocked(ensureSyntaxTree);
// Each heading's text read looks for its wiki links once: how many are read is how many times they are looked for.
vi.mock("./wiki-links", async (actual) => {
  const real = await actual<typeof import("./wiki-links")>();
  return { ...real, wikiLinks: vi.fn(real.wikiLinks) };
});
const scanned = vi.mocked(wikiLinks);

/** The editors the test made, and their parents in the page; a test that destroys one takes it out. */
const views = new Map<EditorView, HTMLElement>();
beforeEach(() => {
  ensured.mockImplementation(language.ensureSyntaxTree);
});
afterEach(() => {
  for (const [view, parent] of views) {
    view.destroy();
    parent.remove();
  }
  views.clear();
  vi.useRealTimers();
});

/** destroyed destroys view, as its page leaving would. */
function destroyed(view: EditorView) {
  view.destroy();
  views.get(view)?.remove();
  views.delete(view);
}

/** sinkOf is a sink of upTo that keeps what each telling told. */
function sinkOf(upTo = 1000) {
  const told: { listed: readonly OutlineHeading[]; more: number }[] = [];
  const sink: OutlineSink = { upTo, told: (listed, more) => void told.push({ listed, more }) };
  return { sink, told };
}

/** viewOf is an editor on doc with the editor's Markdown and the outline told to sink, in the page. */
function viewOf(doc: string, sink: OutlineSink): EditorView {
  const parent = document.createElement("div");
  document.body.append(parent);
  const view = new EditorView({
    state: EditorState.create({ doc, extensions: [markdownEditing(), editOutline(() => sink)] }),
    parent,
  });
  views.set(view, parent);
  return view;
}

/** headings is the last telling's headings, each its level and text. */
const headings = (told: { listed: readonly OutlineHeading[] }[]) =>
  told.at(-1)?.listed.map(({ level, text }) => [level, text]);

/** typed adds text at the content's end. */
const typed = (view: EditorView, text: string) =>
  view.dispatch({ changes: { from: view.state.doc.length, insert: text } });

test("lists the ATX and setext headings, in quotes and lists too, not in code, tables nor the frontmatter, by their text without their marks; empty ones not", () => {
  const { sink, told } = sinkOf();
  viewOf(
    [
      // lezer reads the frontmatter as a rule and a setext heading of its YAML (l6's): it starts in the frontmatter.
      "---",
      'up: "[[Hub]]"',
      'see: "[[Nowhere]]"',
      "---",
      "# Intro",
      "",
      "Setext one",
      "===",
      "",
      "Setext",
      "two",
      "---",
      "",
      "> ## Quoted",
      "",
      "> Quoted setext",
      "> over lines",
      "> ---",
      "",
      "- item",
      "",
      "  ### Listed",
      "",
      "```",
      "# fenced",
      "```",
      "",
      "    # indented",
      "",
      "| # a |",
      "|---|",
      "| # b |",
      "",
      "## Closed ##",
      "",
      "#",
      "",
      "##   Spaced    out  ",
      "",
    ].join("\n"),
    sink
  );
  expect(told).toHaveLength(1);
  expect(headings(told)).toEqual([
    [1, "Intro"],
    [1, "Setext one"],
    [2, "Setext two"],
    [2, "Quoted"],
    [2, "Quoted setext over lines"],
    [3, "Listed"],
    [2, "Closed"],
    [2, "Spaced out"],
  ]);
  expect(told[0]?.more).toBe(0);
});

test("an item reads as the reading view's outline has it: the inline marks, addresses and footnotes' references gone, an image by its alt, a wiki link as the view shows it; in code as written (decision 907)", () => {
  const { sink, told } = sinkOf();
  viewOf(
    [
      "# Setup **now**, _then_ and `code` ~~gone~~",
      '## See [[Page|alias]], [[Page#Sec]], [x](https://x.test "T"), [r][ref] and <https://a.b>',
      "### ![[img.png|300]] and [[#a]] and ![alt *b*](u.png)",
      "## a\\#b &amp; c <span>html</span> ==hl==",
      "## Note[^1] and [[A]]",
      "## `[[not a link]]` and `==not==`",
      "",
      "[^1]: the note",
      "",
      "[ref]: https://r.test",
      "",
    ].join("\n"),
    sink
  );
  expect(headings(told)).toEqual([
    [1, "Setup now, then and code gone"],
    [2, "See alias, Page > Sec, x, r and https://a.b"],
    [3, "img.png and a and alt b"],
    [2, "a#b & c html hl"],
    [2, "Note and A"],
    [2, "[[not a link]] and ==not=="],
  ]);
});

// What the reading view hides, and an embed of a page, as the server shows them (review A's P4S3A-F1-N2).
test("an item leaves out a comment, Obsidian's and HTML's, as the view hides them; an embed of a page shows the page, of an attachment its caption", () => {
  const { sink, told } = sinkOf();
  viewOf(
    [
      "## Comment %%hidden%% shown %%and this%%",
      "## An <!-- a note --> HTML comment",
      "## ![[Page|alias]] and ![[Page|cap|300]] and ![[Page.md|x]] and ![[a.png|Diagram]]",
      "## `%%kept%%` and a lone %% mark",
      "",
    ].join("\n"),
    sink
  );
  expect(headings(told)).toEqual([
    [2, "Comment shown"],
    [2, "An HTML comment"],
    [2, "Page and Page and Page.md and Diagram"],
    [2, "%%kept%% and a lone %% mark"],
  ]);
});

test("a frontmatter being written is the body's, as the server reads it: a closed one alone is left out (decision 908)", () => {
  const { sink, told } = sinkOf();
  viewOf(["---", "# Title", "Some text with [[Link]].", "## Next", "more", "", "## Later", ""].join("\n"), sink);
  expect(headings(told)).toEqual([
    [1, "Title"],
    [2, "Next"],
    [2, "Later"],
  ]);
});

test("lists upTo headings, the others counted", () => {
  const { sink, told } = sinkOf(3);
  viewOf(["# A", "## B", "## C", "## D", "### E", ""].join("\n"), sink);
  expect(headings(told)).toEqual([
    [1, "A"],
    [2, "B"],
    [2, "C"],
  ]);
  expect(told[0]?.more).toBe(2);
});

// A long document's headings past upTo are counted, their text not read: whether they have any is (review A's
// P4S3A-F1-M2).
test("past upTo a heading's text is not read: it is counted if it has any", () => {
  const { sink, told } = sinkOf(2);
  scanned.mockClear();
  viewOf(["# A", "## B", "##", "## C **x** [[Page]]", "> ## D", "##  ##", ""].join("\n"), sink);
  expect(headings(told)).toEqual([
    [1, "A"],
    [2, "B"],
  ]);
  expect(told[0]?.more).toBe(2);
  expect(scanned).toHaveBeenCalledTimes(2);
});

test("tells as it is made, no timer gone by; then once the changes settle, 300 ms after the last; what reads the same tells nothing (review A's M3)", () => {
  vi.useFakeTimers();
  const { sink, told } = sinkOf();
  const view = viewOf("# A\n", sink);
  expect(headings(told)).toEqual([[1, "A"]]);

  for (let i = 0; i < 5; i++) {
    typed(view, `\n# B${i.toString()}`);
    vi.advanceTimersByTime(100);
  }
  expect(told).toHaveLength(1);
  vi.advanceTimersByTime(199);
  expect(told).toHaveLength(1);
  vi.advanceTimersByTime(1);
  expect(told).toHaveLength(2);
  expect(headings(told)).toEqual([
    [1, "A"],
    [1, "B0"],
    [1, "B1"],
    [1, "B2"],
    [1, "B3"],
    [1, "B4"],
  ]);

  // Text after the headings moves none of them.
  typed(view, "\n\nmore");
  vi.advanceTimersByTime(1_000);
  expect(told).toHaveLength(2);
});

test("text put before every heading, moving them all, tells nothing: their places are the plugin's, mapped (review A's P4S3A-N3)", () => {
  vi.useFakeTimers();
  const { sink, told } = sinkOf();
  const view = viewOf("# A\n\n## B\n", sink);
  view.dispatch({ changes: { from: 0, insert: "Intro\n\n" } });
  vi.advanceTimersByTime(1_000);
  expect(told).toHaveLength(1);
  expect(where(view, 1)).toBe(view.state.doc.line(5).from);
});

/** long is some 200,000 characters of prose, far past the parse a state is made with, a heading at its end. */
const long = `${"Some prose, a line of it to read.\n\n".repeat(6_000)}# Last\n`;

/** sliced has the stub cut the first count slices short, the parse not done; the real one parses after. */
function sliced(count: number) {
  let calls = 0;
  ensured.mockImplementation((state, upto, timeout) =>
    ++calls <= count ? null : language.ensureSyntaxTree(state, upto, timeout)
  );
  return () => calls;
}

const nothing = StateEffect.define<null>();

test.each([
  ["a selection moved", (view: EditorView) => view.dispatch({ selection: { anchor: 1 } })],
  ["an effect alone", (view: EditorView) => view.dispatch({ effects: nothing.of(null) })],
])(
  "completes the parse in slices, %s between two of them going on with the state it gives (review A's I4)",
  (_, between) => {
    vi.useFakeTimers();
    const calls = sliced(3);
    const { sink, told } = sinkOf();
    const view = viewOf(long, sink);
    expect(told).toHaveLength(0);
    between(view);
    // The slices one after another, each a task: no 300 ms of a new round.
    for (let i = 0; i < 5; i++) {
      vi.advanceTimersByTime(1);
    }
    expect(headings(told)?.at(-1)).toEqual([1, "Last"]);
    expect(calls()).toBeGreaterThanOrEqual(4);
  }
);

test("a change between two slices ends the round: the outline is read again once the change settles", () => {
  vi.useFakeTimers();
  sliced(3);
  const { sink, told } = sinkOf();
  const view = viewOf(long, sink);
  typed(view, "# Added\n");
  for (let i = 0; i < 5; i++) {
    vi.advanceTimersByTime(1);
  }
  expect(told).toHaveLength(0);
  vi.advanceTimersByTime(295);
  for (let i = 0; i < 5; i++) {
    vi.advanceTimersByTime(1);
  }
  expect(told).toHaveLength(1);
  expect(headings(told)?.slice(-2)).toEqual([
    [1, "Last"],
    [1, "Added"],
  ]);
});

test("where is a heading's place as the text is now: those told, mapped through the changes since (review A's M2)", () => {
  const { sink, told } = sinkOf();
  const view = viewOf("# One\n\n# Two\n", sink);
  expect(told).toHaveLength(1);
  view.dispatch({ changes: { from: 0, insert: "abcde" } });
  expect(where(view, 1)).toBe(view.state.doc.line(3).from);
  expect(where(view, 0)).toBe(5);
  expect(where(view, 2)).toBeUndefined();
});

test("destroyed, the outline tells no more: its timer gone, and the slice it had next (review A's N5)", () => {
  vi.useFakeTimers();
  const { sink, told } = sinkOf();
  const view = viewOf("# A\n", sink);
  typed(view, "\n# B");
  destroyed(view);
  vi.advanceTimersByTime(1_000);
  expect(told).toHaveLength(1);

  const calls = sliced(1);
  const between = sinkOf();
  const cut = viewOf(long, between.sink);
  expect(calls()).toBe(1);
  destroyed(cut);
  for (let i = 0; i < 5; i++) {
    vi.advanceTimersByTime(1);
  }
  vi.advanceTimersByTime(1_000);
  expect(between.told).toHaveLength(0);
  expect(calls()).toBe(1);
});
