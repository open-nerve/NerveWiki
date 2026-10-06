import {
  acceptCompletion,
  closeCompletion,
  completionStatus,
  startCompletion,
  currentCompletions,
  setSelectedCompletion,
} from "@codemirror/autocomplete";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, expect, test, vi } from "vitest";

import { translator } from "../../i18n/i18n";
import type { LinkTarget, TagCount } from "../../services/linking.service";
import { markdownEditing } from "../markdown";
import { editorPhrases } from "../phrases";
import type { EditorContext } from "../registry";
import { linkCompletion } from "./link-completion";

// The completion of links and tags (M6/P7 design 4, 5, 6), in CodeMirror's
// own editor.

const views: EditorView[] = [];

afterEach(() => {
  for (const view of views.splice(0)) {
    view.destroy();
  }
  document.body.replaceChildren();
});

const targets: LinkTarget[] = [
  { id: "p1", kind: "page", name: "Plans", link: "Plans", aliases: ["Roadmap", "a]b", "C#"] },
  { id: "p2", kind: "page", name: "Q3", link: "Plans/Q3", aliases: ["Third"] },
  { id: "p3", kind: "page", name: "Q3", link: "Archive/Q3", aliases: [] },
  { id: "p4", kind: "page", name: "会议纪要", link: "会议纪要", aliases: [] },
];

const counted: TagCount[] = [
  { tag: "project", count: 3 },
  { tag: "project/alpha", count: 1 },
  { tag: "阅读", count: 2 },
  { tag: "v2", count: 1 },
  // Tags of a frontmatter the body cannot write as #tag.
  { tag: "2026", count: 1 },
  { tag: "/", count: 1 },
  { tag: "数据、分析", count: 1 },
];

/** editing is an editor on doc, the cursor at its end, completing from the data given, which it counts reads of. */
function editing(
  doc: string,
  data: { linkTargets?: () => Promise<readonly LinkTarget[]>; readOnly?: boolean; locale?: "en" | "zh-CN" } = {}
) {
  const reads = { targets: 0, tags: 0 };
  const context: EditorContext = {
    workspace: "lab",
    notebook: "n1",
    page: "p0",
    role: "editor",
    linkTargets: () => {
      reads.targets += 1;
      return data.linkTargets?.() ?? Promise.resolve(targets);
    },
    tags: () => {
      reads.tags += 1;
      return Promise.resolve(counted);
    },
  };
  const view = new EditorView({
    state: EditorState.create({
      doc,
      selection: { anchor: doc.length },
      extensions: [
        markdownEditing(),
        editorPhrases(translator(data.locale ?? "en")),
        EditorState.readOnly.of(data.readOnly ?? false),
        linkCompletion(context, {} as never),
      ],
    }),
    parent: document.body,
  });
  views.push(view);
  return { view, reads };
}

/** type types text at the cursor, as the keyboard does. */
function type(view: EditorView, text: string) {
  const at = view.state.selection.main.head;
  view.dispatch({
    changes: { from: at, insert: text },
    selection: { anchor: at + text.length },
    userEvent: "input.type",
  });
}

/** shown is what the completion lists: each option's text shown, and its detail. */
function shown(view: EditorView) {
  return currentCompletions(view.state).map(({ label, displayLabel, detail }) => [displayLabel ?? label, detail]);
}

/** opened waits for the completion to list options. */
async function opened(view: EditorView) {
  await vi.waitFor(() => expect(completionStatus(view.state)).toBe("active"));
  await vi.waitFor(() => expect(currentCompletions(view.state).length).toBeGreaterThan(0));
}

/** none tells that no completion opens: a while is given for one to. */
async function none(view: EditorView) {
  await new Promise((resolve) => setTimeout(resolve, 250));
  expect(currentCompletions(view.state)).toEqual([]);
}

/** pick accepts the option shown as text, once CodeMirror lets a completion just opened be (interactionDelay). */
async function pick(view: EditorView, text: string) {
  const index = shown(view).findIndex(([each]) => each === text);
  expect(index).toBeGreaterThanOrEqual(0);
  view.dispatch({ effects: setSelectedCompletion(index) });
  await new Promise((resolve) => setTimeout(resolve, 100));
  expect(acceptCompletion(view)).toBe(true);
}

test("after [[ the notebook's pages are listed by title, a title others share with its link, and the aliases to their page's link", async () => {
  const { view } = editing("See ");
  type(view, "[[");
  await opened(view);

  expect(shown(view)).toEqual(
    expect.arrayContaining([
      ["Plans", undefined],
      ["Roadmap", "→ Plans"],
      ["C#", "→ Plans"],
      ["Q3", "Plans/Q3"],
      ["Third", "→ Plans/Q3"],
      ["Q3", "Archive/Q3"],
      ["会议纪要", undefined],
    ])
  );
  // An alias with a bracket would end the link it is written in.
  expect(shown(view)).toHaveLength(7);
});

test("what is typed after [[ filters by title and by link: a page's folder finds it; Chinese too", async () => {
  const { view } = editing("");
  type(view, "[[Archi");
  await opened(view);
  expect(shown(view)).toEqual([["Q3", "Archive/Q3"]]);

  const chinese = editing("").view;
  type(chinese, "[[会议");
  await opened(chinese);
  expect(shown(chinese)).toEqual([["会议纪要", undefined]]);
});

test("a page picked is written [[link]], an alias [[link|alias]], the cursor after; a ]] after the cursor is not written twice", async () => {
  const { view } = editing("See ");
  type(view, "[[Pla");
  await opened(view);
  await pick(view, "Plans");
  expect(view.state.doc.toString()).toBe("See [[Plans]]");
  expect(view.state.selection.main.head).toBe("See [[Plans]]".length);

  const alias = editing("").view;
  type(alias, "[[Road");
  await opened(alias);
  await pick(alias, "Roadmap");
  expect(alias.state.doc.toString()).toBe("[[Plans|Roadmap]]");

  const closed = editing("[[]]").view;
  closed.dispatch({ selection: { anchor: 2 } });
  type(closed, "Archi");
  await opened(closed);
  await pick(closed, "Q3");
  expect(closed.state.doc.toString()).toBe("[[Archive/Q3]]");
  expect(closed.state.selection.main.head).toBe("[[Archive/Q3]]".length);
});

test("past a link's text, an anchor or an alias written, the completion is over", async () => {
  for (const typed of ["[[Plans|", "[[Plans#", "[[Plans]] ", "[[a]b"]) {
    const { view } = editing("");
    type(view, typed);
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await none(view);
  }
});

test("after a # at a line's start or a space, the notebook's tags are listed with their pages; one picked is written", async () => {
  const { view } = editing("Read ");
  type(view, "#pro");
  await opened(view);
  expect(shown(view)).toEqual([
    ["project", "3 pages"],
    ["project/alpha", "1 page"],
  ]);
  await pick(view, "project/alpha");
  expect(view.state.doc.toString()).toBe("Read #project/alpha");

  const start = editing("").view;
  type(start, "#阅");
  await opened(start);
  expect(shown(start)).toEqual([["阅读", "2 pages"]]);
});

test("a # in a word, after another #, or a heading's, followed by a space, completes nothing", async () => {
  for (const [doc, typed] of [
    ["word", "#"],
    ["", "##"],
    ["", "# "],
    ["", "#pro "],
  ]) {
    const { view } = editing(doc ?? "");
    type(view, typed ?? "");
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await none(view);
  }
});

test("in code nothing completes: inline code, a fenced block, an indented one", async () => {
  for (const [doc, typed] of [
    ["`a ", "[[Pl`"],
    ["```\n", "[[Pl"],
    ["```\n#", "pro"],
    ["    code ", "[[Pl"],
  ]) {
    const { view } = editing(doc ?? "");
    if (doc === "`a ") {
      // The cursor inside the inline code, before its closing backtick.
      type(view, typed ?? "");
      view.dispatch({ selection: { anchor: view.state.doc.length - 1 } });
      type(view, "a");
    } else {
      type(view, typed ?? "");
    }
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await none(view);
  }
});

test("a content that cannot be changed completes nothing", async () => {
  const { view } = editing("", { readOnly: true });
  type(view, "[[Pl");
  await none(view);
});

test("the data is read once a completion: typing on filters what was read; a new [[ reads again", async () => {
  const { view, reads } = editing("");
  type(view, "[[");
  await opened(view);
  type(view, "P");
  type(view, "l");
  await opened(view);
  expect(reads.targets).toBe(1);
  type(view, "]] and ");
  await none(view);
  type(view, "[[");
  await opened(view);
  expect(reads.targets).toBe(2);
});

test("data that cannot be read opens no completion, and says so on the console", async () => {
  const failed = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const { view } = editing("", { linkTargets: () => Promise.reject(new Error("offline")) });
  type(view, "[[Pl");
  await none(view);
  expect(failed).toHaveBeenCalledOnce();
});

test("an input method's composition closes the completion, and none opens while it composes", async () => {
  const { view } = editing("");
  type(view, "[[");
  await opened(view);

  view.contentDOM.dispatchEvent(new CompositionEvent("compositionstart", { bubbles: true }));
  expect(currentCompletions(view.state)).toEqual([]);
  Object.defineProperty(view, "composing", { configurable: true, get: () => true });
  view.dispatch({
    // What it composes would be completed otherwise.
    changes: { from: 2, insert: "Pl" },
    selection: { anchor: 4 },
    userEvent: "input.type.compose",
  });
  await none(view);
});

test("in a link already closed, a pick writes the target alone: its anchor stays, and its display text unless an alias is picked", async () => {
  for (const [doc, at, typed, picked, written] of [
    ["[[|old]]", 2, "Pla", "Plans", "[[Plans|old]]"],
    ["[[#Part]]", 2, "Pla", "Plans", "[[Plans#Part]]"],
    ["[[Plxx]] after", 4, "a", "Plans", "[[Plans]] after"],
    ["[[Ro|old]]", 4, "a", "Roadmap", "[[Plans|Roadmap]]"],
    ["[[Ro#Part]]", 4, "a", "Roadmap", "[[Plans#Part|Roadmap]]"],
  ] as const) {
    const { view } = editing(doc);
    view.dispatch({ selection: { anchor: at } });
    type(view, typed);
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await opened(view);
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await pick(view, picked);
    expect(view.state.doc.toString()).toBe(written);
    // The cursor after the link.
    expect(view.state.selection.main.head).toBe(written.indexOf("]]") + 2);
  }
});

test("in a table an alias is written with \\|, which the table does not split at; a page as elsewhere", async () => {
  const head = "| a | b |\n| - | - |\n| x | ";
  const { view } = editing(head);
  type(view, "[[Road");
  await opened(view);
  await pick(view, "Roadmap");
  expect(view.state.doc.toString()).toBe(`${head}[[Plans\\|Roadmap]]`);

  const page = editing(head).view;
  type(page, "[[Pla");
  await opened(page);
  await pick(page, "Plans");
  expect(page.state.doc.toString()).toBe(`${head}[[Plans]]`);

  const closed = editing(`${head}[[Ro\\|old]] |`).view;
  closed.dispatch({ selection: { anchor: head.length + 4 } });
  type(closed, "a");
  await opened(closed);
  await pick(closed, "Roadmap");
  expect(closed.state.doc.toString()).toBe(`${head}[[Plans\\|Roadmap]] |`);

  // The table's head as its body.
  const header = editingAt("| ‸ | b |\n| - | - |\n| x | y |").view;
  type(header, "[[Road");
  await opened(header);
  await pick(header, "Roadmap");
  expect(header.state.doc.toString()).toBe("| [[Plans\\|Roadmap]] | b |\n| - | - |\n| x | y |");
});

test("an embed's [[ lists the pages, not the aliases: an embed's display text is its size", async () => {
  const { view } = editing("");
  type(view, "![[");
  await opened(view);
  expect(
    shown(view)
      .map(([label]) => label)
      .toSorted()
  ).toEqual(["Plans", "Q3", "Q3", "会议纪要"]);
});

test("an escaped [[, raw HTML, an autolink, and a # in a link being written (in a table, the cell's) complete nothing; two backslashes escape none", async () => {
  const table = "| a | b |\n| - | - |\n| ‸";
  for (const [doc, typed] of [
    ["‸", "\\[[Pl"],
    ["<!-- ‸", "#pro"],
    ["<div>\n‸", "[[Pl"],
    ["a <!-- ‸ -->", "[[Pl"],
    ["<https://x.test/‸>", "[[Pl"],
    ["‸", "[[Plans #pro"],
    ["‸", "[[Plans|see #pro"],
    // The last [[ is the one being written, in a table in the cell, which an escaped '|' does not end.
    ["‸", "[[a]] [[b #pro"],
    [table, "[[x #pro"],
    [table, "[[x\\|y #pro"],
    // Nor does one after two backslashes, as the server reads it.
    [table, "[[x \\\\| #pro"],
  ]) {
    const { view } = editingAt(doc ?? "");
    type(view, typed ?? "");
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await none(view);
  }
  for (const [doc, typed] of [
    ["‸", "\\\\[[Pl"],
    ["‸", "\\[[x #pro"],
    // Code's [[, on a line after the first; a link's address's; another cell's.
    ["x\n‸", "`[[` #pro"],
    ["‸", "[t]([[x) #pro"],
    [table, "[[x | #pro"],
    // A table's row without a '|' is one cell.
    ["| a | b |\n| - | - |\n| x | y |\n‸", "row #pro"],
  ]) {
    const { view } = editingAt(doc ?? "");
    type(view, typed ?? "");
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await opened(view);
  }
  // An escaped '!' makes no embed: the link has its aliases.
  const link = editing("").view;
  type(link, "\\![[Road");
  await opened(link);
  expect(shown(link).map(([label]) => label)).toContain("Roadmap");
});

/** editingAt is an editor on doc, the cursor where ‸ is (taken out). */
function editingAt(doc: string, data: Parameters<typeof editing>[1] = {}) {
  const at = doc.indexOf("‸");
  const editor = editing(doc.replace("‸", ""), data);
  editor.view.dispatch({ selection: { anchor: at } });
  return editor;
}

test("in a frontmatter, as the server finds it, a link completes in quotes, as a property link is written; a tag not at all", async () => {
  for (const [doc, typed, written] of [
    ["---\nup: ‸\n---", '"[[Pla', '---\nup: "[[Plans]]\n---'],
    ["---\nup: ‸\n---", "'[[Pla", "---\nup: '[[Plans]]\n---"],
    // What the editor parses as Markdown's code is YAML there: a value indented after a blank line, a block's fence.
    ["---\nmeta:\n\n    up: ‸\n---", '"[[Pla', '---\nmeta:\n\n    up: "[[Plans]]\n---'],
    ["---\nnote: |\n  ```\nup: ‸\n---", '"[[Pla', '---\nnote: |\n  ```\nup: "[[Plans]]\n---'],
    // One being written, not closed yet.
    ["---\nup: ‸", '"[[Pla', '---\nup: "[[Plans]]'],
  ] as const) {
    const { view } = editingAt(doc);
    type(view, typed);
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await opened(view);
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await pick(view, "Plans");
    expect(view.state.doc.toString()).toBe(written);
  }
  for (const [doc, typed] of [
    ["---\nup: ‸\n---", "[[Pla"],
    // "..." closes nothing: the frontmatter goes on to the "---".
    ["---\nup: x\n...\n‸\n---", "[[Pla"],
    ["---\nup: ‸\n---", "#pro"],
    ["---\n- ‸\n---", "#pro"],
    // One being written, as far as an empty line: "..." closes nothing.
    ["---\nup: ‸", "[[Pla"],
    ["---\ntags: ‸\nup: x", "#pro"],
    ["---\nup: x\n...\n‸", "[[Pla"],
    // Code in one being written is the body's code until it is closed.
    ["---\n```js\nconst a = ‸", '"[[Pla'],
  ]) {
    const { view } = editingAt(doc ?? "");
    type(view, typed ?? "");
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await none(view);
  }
});

test("in a frontmatter a table the editor finds in a block's text is none, closed or being written: an alias is written with |, as YAML takes it", async () => {
  for (const [doc, written] of [
    [
      "---\nnote: |\n  intro\n\n  | a | b |\n  | - | - |\nup: ‸\n\nx: y\n---",
      '---\nnote: |\n  intro\n\n  | a | b |\n  | - | - |\nup: "[[Plans|Roadmap]]\n\nx: y\n---',
    ],
    ["---\nnote: >-\n  x | y\n  :-|-:\n  z\nup: ‸", '---\nnote: >-\n  x | y\n  :-|-:\n  z\nup: "[[Plans|Roadmap]]'],
  ]) {
    const { view } = editingAt(doc ?? "");
    type(view, '"[[Road');
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await opened(view);
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await pick(view, "Roadmap");
    expect(view.state.doc.toString()).toBe(written);
  }
});

test("what the server reads as no frontmatter completes as the body does: one not closed after its first blank line, one opened by '--- ', after one, an empty one too", async () => {
  for (const [doc, typed] of [
    ["---\nintro\n\n‸", "[[Pla"],
    ["---\nintro\n\n‸", "#pro"],
    // The first blank line ends it, one of spaces and tabs too; an empty one has the body after it.
    ["---\nIntro\n\nmore ‸\n\nend", "[[Pla"],
    ["---\nintro\n\t\n‸", "[[Pla"],
    ["---\n---\n‸", "#pro"],
    ["---\nup: x\n...\n \n‸", "[[Pla"],
    ["--- \nup: ‸\n---", "[[Pla"],
    ["---\nup: x\n---\n‸", "#pro"],
  ]) {
    const { view } = editingAt(doc ?? "");
    type(view, typed ?? "");
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await opened(view);
  }
});

test("a tag the body cannot write is not listed; a tag's characters go on: digits, a slash, after a full-width space", async () => {
  const { view } = editing("");
  type(view, "#");
  await opened(view);
  expect(
    shown(view)
      .map(([label]) => label)
      .toSorted()
  ).toEqual(["project", "project/alpha", "v2", "阅读"]);

  for (const typed of ["#v2", "\u3000#project/al"]) {
    const each = editing("").view;
    type(each, typed);
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await opened(each);
  }
});

test("a long run of backslashes before what is typed costs a time as long as it: a link's completion, a # in it", async () => {
  const { view } = editing("\\".repeat(1 << 17));
  const started = performance.now();
  // What follows the run is not [[: a pattern for the run would try each of its places.
  type(view, "a[[Pla");
  await opened(view);
  type(view, " #pro");
  await none(view);
  expect(performance.now() - started).toBeLessThan(2_000);
});

test("a link's completion ends at a character its target cannot hold, typed after it opened", async () => {
  const { view } = editing("");
  type(view, "[[C");
  await opened(view);
  expect(shown(view).map(([label]) => label)).toContain("C#");
  type(view, "#");
  await none(view);
});

test("the completions of one [[ read once, its completion closed and opened again (an input method's composition); after a while, anew", async () => {
  const { view, reads } = editing("");
  type(view, "[[");
  await opened(view);
  closeCompletion(view);
  type(view, "P");
  await opened(view);
  expect(reads.targets).toBe(1);

  const meanwhile = vi.spyOn(Date, "now").mockReturnValue(Date.now() + 9_000);
  closeCompletion(view);
  type(view, "l");
  await opened(view);
  expect(reads.targets).toBe(1);
  meanwhile.mockRestore();

  const later = vi.spyOn(Date, "now").mockReturnValue(Date.now() + 11_000);
  closeCompletion(view);
  type(view, "a");
  await opened(view);
  expect(reads.targets).toBe(2);
  later.mockRestore();
});

test("the completion speaks the editor's language: its list's name, and how many pages a tag has", async () => {
  const { view } = editing("", { locale: "zh-CN" });
  type(view, "#阅");
  await opened(view);
  expect(shown(view)).toEqual([["阅读", "2 页"]]);
  expect(view.dom.querySelector("[role=listbox]")?.getAttribute("aria-label")).toBe("补全");
});

test("what matched shows in a page's title, though it matched its link: a title others share as well", async () => {
  const { view } = editing("");
  type(view, "[[Q3");
  await opened(view);
  const matched = [...view.dom.querySelectorAll("[role=option] .cm-completionLabel")].map((label) =>
    [...label.querySelectorAll(".cm-completionMatchedText")].map((each) => each.textContent).join("")
  );
  expect(matched).toEqual(["Q3", "Q3"]);
});

test("a query that matches nothing reads once as it goes on: the [[ or the # it is of is the same", async () => {
  const { view, reads } = editing("");
  type(view, "[[zz");
  await none(view);
  for (const key of ["z", "y", " ", "x"]) {
    type(view, key);
    // oxlint-disable-next-line no-await-in-loop -- one key after another
    await new Promise((resolve) => setTimeout(resolve, 60));
  }
  await none(view);
  expect(reads.targets).toBe(1);

  type(view, "\n#zz");
  await none(view);
  type(view, "z");
  type(view, "y");
  await none(view);
  expect(reads.tags).toBe(1);
});

test("a screen reader hears a pause between an option's text and its detail, which shows none", async () => {
  const { view } = editing("");
  type(view, "#proj");
  await opened(view);
  const options = [...view.dom.querySelectorAll("[role=option]")].map((option) => option.textContent);
  expect(options).toEqual(["project, 3 pages", "project/alpha, 1 page"]);
  expect(view.dom.querySelector(".nw-completion-pause")?.textContent).toBe(", ");
});

test("a tag picked with the cursor in a tag is written over the rest of it", async () => {
  const { view } = editing("See #Start");
  view.dispatch({ selection: { anchor: "See #".length } });
  type(view, "pro");
  await opened(view);
  await pick(view, "project");
  expect(view.state.doc.toString()).toBe("See #project");
  expect(view.state.selection.main.head).toBe("See #project".length);
});

test("what matched shows in a title whose link holds it before its end (.md after a path)", async () => {
  const { view } = editing("", {
    linkTargets: () =>
      Promise.resolve([{ id: "p9", kind: "page", name: "Notes.md", link: "Notes.md.md", aliases: [] }]),
  });
  type(view, "[[Notes");
  await opened(view);
  const label = view.dom.querySelector("[role=option] .cm-completionLabel");
  expect(label?.textContent).toBe("Notes.md");
  expect(label?.querySelector(".cm-completionMatchedText")?.textContent).toBe("Notes");
});

test("in a link already closed: a table's \\| stays, a '^' is the target's, a display text holding '|' goes with an alias picked", async () => {
  for (const [doc, picked, written] of [
    ["| a | b |\n| - | - |\n| x | [[Pl‸\\|old]] |", "Plans", "| a | b |\n| - | - |\n| x | [[Plans\\|old]] |"],
    ["[[Pl‸^b]]", "Plans", "[[Plans]]"],
    ["[[Ro‸#a|b|c]]", "Roadmap", "[[Plans#a|Roadmap]]"],
  ] as const) {
    const { view } = editingAt(doc);
    type(view, "a");
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await opened(view);
    // oxlint-disable-next-line no-await-in-loop -- one editor after another
    await pick(view, picked);
    expect(view.state.doc.toString()).toBe(written);
  }
});

/** aliasesTargets is a page whose aliases hold a '|' and a line's end. */
function aliasesTargets(): Promise<LinkTarget[]> {
  return Promise.resolve([{ id: "p1", kind: "page", name: "Plans", link: "Plans", aliases: ["x|y", "x\ny", "xz"] }]);
}

test("an alias with a line's end is not listed; in a table, nor one with a '|'", async () => {
  const linkTargets = aliasesTargets;
  const { view } = editing("", { linkTargets });
  type(view, "[[x");
  await opened(view);
  expect(shown(view).map(([label]) => label)).toEqual(["x|y", "xz"]);

  const table = editing("| a | b |\n| - | - |\n| x | ", { linkTargets }).view;
  type(table, "[[x");
  await opened(table);
  expect(shown(table).map(([label]) => label)).toEqual(["xz"]);
});

test("a failed read is not remembered: the next key reads again; a completion asked for reads anew", async () => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
  let calls = 0;
  const { view, reads } = editing("", {
    linkTargets: () => (++calls === 1 ? Promise.reject(new Error("offline")) : Promise.resolve(targets)),
  });
  type(view, "[[Pl");
  await none(view);
  type(view, "a");
  await opened(view);
  expect(reads.targets).toBe(2);

  startCompletion(view);
  await opened(view);
  expect(reads.targets).toBe(3);

  const tagged = editing("");
  type(tagged.view, "#pro");
  await opened(tagged.view);
  startCompletion(tagged.view);
  await opened(tagged.view);
  expect(tagged.reads.tags).toBe(2);
});

test("what matched shows in a title as the link's last: a path's folder of the same name is not the title", async () => {
  const { view } = editing("", {
    linkTargets: () => Promise.resolve([{ id: "p9", kind: "page", name: "Notes", link: "Notes/Notes", aliases: [] }]),
  });
  type(view, "[[Notes/N");
  await opened(view);
  const label = view.dom.querySelector("[role=option] .cm-completionLabel");
  expect(label?.querySelector(".cm-completionMatchedText")?.textContent).toBe("N");
});

test("an option without a detail has no pause", async () => {
  const { view } = editing("");
  type(view, "[[Pla");
  await opened(view);
  const plans = [...view.dom.querySelectorAll("[role=option]")].find((option) => option.textContent === "Plans");
  expect(plans).toBeDefined();
  expect(plans?.querySelector(".nw-completion-pause")).toBeNull();
});
