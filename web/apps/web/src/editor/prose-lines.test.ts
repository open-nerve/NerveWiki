import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, expect, test } from "vitest";

import { markdownEditing } from "./markdown";
import { markupMarks, proseLines } from "./prose-lines";

// The editor's lines of code, tables and frontmatter in monospace, its marks faint (M13/P4/S3, decisions 874, 889): of
// the lines in view alone. jsdom draws a few dozen lines: what is decorated is counted in the plugins' sets.

const views: EditorView[] = [];
afterEach(() => {
  for (const view of views.splice(0)) {
    view.destroy();
    view.dom.remove();
  }
});

/** viewOf is an editor on doc with the editor's Markdown, proseLines and markupMarks, in the page. */
function viewOf(doc: string): EditorView {
  const parent = document.createElement("div");
  document.body.append(parent);
  const view = new EditorView({
    state: EditorState.create({ doc, extensions: [markdownEditing(), proseLines, markupMarks] }),
    parent,
  });
  views.push(view);
  return view;
}

/** classed is each line's text and its classes of proseLines', by line: "" for none. */
function classed(view: EditorView): [string, string][] {
  return [...view.contentDOM.querySelectorAll(".cm-line")].map((line) => [
    line.textContent ?? "",
    [...line.classList].filter((name) => name.startsWith("nw-")).join(" "),
  ]);
}

/** marked is what markupMarks decorates: each range's text and its class, in order. */
function marked(view: EditorView): [string, string][] {
  const found: [string, string][] = [];
  const set = view.plugin(markupMarks)?.decorations;
  if (set === undefined) {
    throw new Error("no markupMarks");
  }
  for (const cursor = set.iter(); cursor.value !== null; cursor.next()) {
    found.push([view.state.sliceDoc(cursor.from, cursor.to), String(cursor.value.spec.class)]);
  }
  return found;
}

/** shownLines is how many lines the editor's visible ranges cover. */
function shownLines(view: EditorView): number {
  return view.visibleRanges.reduce(
    (lines, { from, to }) => lines + view.state.doc.lineAt(to).number - view.state.doc.lineAt(from).number + 1,
    0
  );
}

const mono = "nw-mono-line";
const front = "nw-mono-line nw-frontmatter-line";

test("proseLines has the lines of a fence, an indented code block and a table monospaced, in a quote and a list too; the prose's not", () => {
  const view = viewOf(
    [
      "Prose",
      "```go",
      "x := 1",
      "```",
      "",
      "    indented",
      "",
      "| a | b |",
      "|---|---|",
      "| 1 | 2 |",
      "",
      "> ```",
      "> quoted",
      "> ```",
      "",
      "- item",
      "  ```",
      "  listed",
      "  ```",
      "",
      "After",
    ].join("\n")
  );
  expect(classed(view)).toEqual([
    ["Prose", ""],
    ["```go", mono],
    ["x := 1", mono],
    ["```", mono],
    ["", ""],
    ["    indented", mono],
    ["", ""],
    ["| a | b |", mono],
    ["|---|---|", mono],
    ["| 1 | 2 |", mono],
    ["", ""],
    ["> ```", mono],
    ["> quoted", mono],
    ["> ```", mono],
    ["", ""],
    ["- item", ""],
    ["  ```", mono],
    ["  listed", mono],
    ["  ```", mono],
    ["", ""],
    ["After", ""],
  ]);
});

test("proseLines has the frontmatter's lines monospaced and of the frontmatter, closed or being written; nothing begun in it is code", () => {
  // lezer reads the fence in the YAML as one, open to the content's end: the server reads the body after it as prose.
  const closed = viewOf(["---", 'up: "[[Hub]]"', "fence: |", "  ```", "---", "Body"].join("\n"));
  expect(classed(closed)).toEqual([
    ["---", front],
    ['up: "[[Hub]]"', front],
    ["fence: |", front],
    ["  ```", front],
    ["---", front],
    ["Body", ""],
  ]);
  const open = viewOf(["---", "tags: [a, b]", "", "```", "x", "```"].join("\n"));
  expect(classed(open)).toEqual([
    ["---", front],
    ["tags: [a, b]", front],
    ["", ""],
    ["```", mono],
    ["x", mono],
    ["```", mono],
  ]);
});

test("proseLines decorates the lines anew as the content changes", () => {
  const view = viewOf("Prose\n");
  expect(classed(view)).toEqual([
    ["Prose", ""],
    ["", ""],
  ]);
  view.dispatch({ changes: { from: view.state.doc.length, insert: "```\ncode\n```" } });
  expect(classed(view)).toEqual([
    ["Prose", ""],
    ["```", mono],
    ["code", mono],
    ["```", mono],
  ]);
});

test("proseLines decorates the lines in view alone: of 8,000 lines of code, no more than the editor shows (review C's I2)", () => {
  const view = viewOf(["```", ...Array.from({ length: 8_000 }, (_, i) => `line ${i.toString()}`), "```"].join("\n"));
  const size = view.plugin(proseLines)?.decorations.size ?? 0;
  expect(size).toBeGreaterThan(0);
  expect(size).toBeLessThanOrEqual(shownLines(view));
  expect(shownLines(view)).toBeLessThan(1_000);
});

test("markupMarks fades a wiki link's marks, its page underlined, its section and alias as the prose (decision 889)", () => {
  expect(marked(viewOf("See [[Deploy guide#Steps|guide]] now."))).toEqual([
    ["[[", "nw-md-faint"],
    ["Deploy guide", "nw-md-link"],
    ["#", "nw-md-faint"],
    ["|", "nw-md-faint"],
    ["]]", "nw-md-faint"],
  ]);
  expect(marked(viewOf("![[a.png]] and ![[b.png|100]]"))).toEqual([
    ["![[", "nw-md-faint"],
    ["a.png", "nw-md-link"],
    ["]]", "nw-md-faint"],
    ["![[", "nw-md-faint"],
    ["b.png", "nw-md-link"],
    ["|", "nw-md-faint"],
    ["]]", "nw-md-faint"],
  ]);
  // No page: the section's heading is underlined.
  expect(marked(viewOf("[[#Steps]] and [[#Steps|there]]"))).toEqual([
    ["[[", "nw-md-faint"],
    ["#", "nw-md-faint"],
    ["Steps", "nw-md-link"],
    ["]]", "nw-md-faint"],
    ["[[", "nw-md-faint"],
    ["#", "nw-md-faint"],
    ["Steps", "nw-md-link"],
    ["|", "nw-md-faint"],
    ["]]", "nw-md-faint"],
  ]);
});

test("markupMarks underlines a link's text alone, not an image's nor a reference's; it fades a highlight's marks, not an equality's", () => {
  // The link's brackets and address are faint by the highlighting.
  expect(marked(viewOf("A [text](https://x.test/a) b"))).toEqual([["text", "nw-md-link"]]);
  expect(marked(viewOf("An ![alt](a.png), a [ref] and [](https://x.test)"))).toEqual([]);
  expect(marked(viewOf("A ==mark== and a == b, ==x=="))).toEqual([
    ["==", "nw-md-faint"],
    ["==", "nw-md-faint"],
    ["==", "nw-md-faint"],
    ["==", "nw-md-faint"],
  ]);
});

test("markupMarks marks nothing in inline code, a code block nor the frontmatter (decision 889)", () => {
  const view = viewOf(
    [
      "---",
      'up: "[[Hub]]"',
      "tags: [a, b]",
      "note: ==x== [t](https://x.test)",
      "---",
      "Code `[[a]] ==b==` here.",
      "```",
      "[[c]] ==d== [t](https://x.test)",
      "```",
      "",
      "    [[e]] ==f==",
    ].join("\n")
  );
  expect(marked(view)).toEqual([]);
});

test("markupMarks marks the lines in view alone: of 8,000 lines of a wiki link each, four ranges a line at most", () => {
  const view = viewOf(Array.from({ length: 8_000 }, (_, i) => `[[Page ${i.toString()}]] text`).join("\n"));
  const size = view.plugin(markupMarks)?.decorations.size ?? 0;
  expect(size).toBeGreaterThan(0);
  expect(size).toBeLessThanOrEqual(4 * shownLines(view));
  expect(shownLines(view)).toBeLessThan(1_000);
});
