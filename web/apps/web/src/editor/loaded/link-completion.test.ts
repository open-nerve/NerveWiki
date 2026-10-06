import {
  acceptCompletion,
  completionStatus,
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
  { id: "p1", kind: "page", name: "Plans", link: "Plans", aliases: ["Roadmap"] },
  { id: "p2", kind: "page", name: "Q3", link: "Plans/Q3", aliases: [] },
  { id: "p3", kind: "page", name: "Q3", link: "Archive/Q3", aliases: [] },
  { id: "p4", kind: "page", name: "会议纪要", link: "会议纪要", aliases: [] },
];

const counted: TagCount[] = [
  { tag: "project", count: 3 },
  { tag: "project/alpha", count: 1 },
  { tag: "阅读", count: 2 },
];

/** editing is an editor on doc, the cursor at its end, completing from the data given, which it counts reads of. */
function editing(doc: string, data: { linkTargets?: () => Promise<readonly LinkTarget[]>; readOnly?: boolean } = {}) {
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
        editorPhrases(translator("en")),
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

test("after [[ the notebook's pages are listed by title, a title others share with its link, and the aliases to their page", async () => {
  const { view } = editing("See ");
  type(view, "[[");
  await opened(view);

  expect(shown(view)).toEqual(
    expect.arrayContaining([
      ["Plans", undefined],
      ["Roadmap", "→ Plans"],
      ["Q3", "Plans/Q3"],
      ["Q3", "Archive/Q3"],
      ["会议纪要", undefined],
    ])
  );
  expect(shown(view)).toHaveLength(5);
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
    ["project/alpha", "1 pages"],
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
