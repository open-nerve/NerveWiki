import { autocompletion, completionStatus, startCompletion } from "@codemirror/autocomplete";
import { EditorState, type Extension } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import { translator } from "../../i18n/i18n";
import { pageDrag } from "../../lib/file-transfer";
import type { Asset } from "../../services/asset.service";
import { dropped } from "../../test/attachments";
import { assetJSON, assetNode, guide } from "../../test/page-server";
import { readOnly, readOnlyAs } from "../extensions";
import { editorPhrases } from "../phrases";
import type { EditorContext } from "../registry";
import { fakeControls } from "../testing/fake-controls";
import { assetUpload, pastedName } from "./asset-upload";

// The upload of files pasted into the editor or dropped on it (M7/P4 design 5.2).

afterEach(() => {
  pageDrag.on = false;
  document.body.replaceChildren();
});

/** Going is an upload the test answers: with its attachment, or failing. */
type Going = { file: File; answer: (asset: Asset) => void; fail: (error: unknown) => void };

/** attachment is the attachment name uploads as, its link its name, none without an extension. */
const attachment = (name: string) => assetJSON(assetNode(80, name, guide));

/**
 * editing is an editor on doc, the selection from anchor to head, with the
 * upload of files, and phrases and extra extensions when given; the
 * uploads asked for wait for the test's answer, and what the editor told
 * is kept.
 */
function editing(
  doc: string,
  {
    anchor = doc.length,
    head = anchor,
    phrases = [],
    extra = [],
  }: { anchor?: number; head?: number; phrases?: Extension; extra?: Extension } = {}
) {
  const going: Going[] = [];
  const context: EditorContext = {
    workspace: "lab",
    notebook: "n1",
    page: guide.id,
    role: "editor",
    linkTargets: () => Promise.resolve([]),
    tags: () => Promise.resolve([]),
    uploadAsset: (file) =>
      new Promise((answer, fail) => {
        going.push({ file, answer, fail });
      }),
  };
  const { controls, close } = fakeControls();
  const parent = document.createElement("div");
  document.body.append(parent);
  const view = new EditorView({
    parent,
    state: EditorState.create({
      doc,
      selection: { anchor, head },
      extensions: [readOnly.of(readOnlyAs(false)), phrases, extra, assetUpload(context, controls)],
    }),
  });
  return { view, going, controls, close, text: () => view.state.doc.toString() };
}

/** clipboard is a paste's clipboard of files, and of text when given. */
function clipboard(files: File[], text?: string) {
  const transfer = transferOf(dropped(files));
  return {
    ...transfer,
    types: text === undefined ? transfer.types : [...transfer.types, "text/plain"],
    getData: (type: string) => (type === "text/plain" ? (text ?? "") : ""),
  };
}

/** transferOf is a drag's transfer as CodeMirror reads one it takes: no text in it. */
function transferOf<T extends object>(transfer: T): T & { getData: () => string } {
  return { ...transfer, getData: () => "" };
}

/** paste pastes clipboard into view's content, as the browser has it: whether it was taken (its default prevented). */
function paste(view: EditorView, data: unknown): boolean {
  const event = new Event("paste", { bubbles: true, cancelable: true });
  Object.defineProperty(event, "clipboardData", { value: data });
  view.contentDOM.dispatchEvent(event);
  return event.defaultPrevented;
}

/** drag sends a drag's event of type over view's content, at at, with transfer: whether it was taken (its default prevented). */
function drag(view: EditorView, type: "dragover" | "drop", transfer: unknown, at = { x: 1, y: 1 }): boolean {
  const event = new Event(type, { bubbles: true, cancelable: true });
  Object.assign(event, { clientX: at.x, clientY: at.y });
  Object.defineProperty(event, "dataTransfer", { value: transfer });
  view.contentDOM.dispatchEvent(event);
  return event.defaultPrevented;
}

/** aTurn lets a turn of the event loop go: a rejection left unhandled is reported then. */
function aTurn(): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, 0);
  });
}

/** settle lets the promises out settle: a turn of the event loop. */
const settle = aTurn;

/** told is what the editor told, in turn: each paste or drop begins anew (""). */
function told(controls: ReturnType<typeof fakeControls>["controls"]): string[] {
  return controls.tell.mock.calls.map(([text]) => text);
}

test("an image the clipboard holds without a name of its own is named as Obsidian names it; others keep theirs", () => {
  const now = new Date(2026, 9, 8, 12, 30, 45);
  const named = (name: string, type: string) => pastedName(new File(["x"], name, { type }), now);
  expect(named("image.png", "image/png")).toBe("Pasted image 20261008123045.png");
  expect(named("", "image/jpeg")).toBe("Pasted image 20261008123045.jpg");
  expect(named("image.webp", "image/webp")).toBe("Pasted image 20261008123045.webp");
  expect(named("", "image/x-icon")).toBe("Pasted image 20261008123045.ico");
  expect(named("", "image/heic")).toBe("Pasted image 20261008123045.heic");
  expect(named("photo.png", "image/png")).toBe("photo.png");
  expect(named("image.png", "")).toBe("image.png");
  expect(named("notes.txt", "text/plain")).toBe("notes.txt");

  // Its local time, wherever the clock is.
  const local = Object.assign(new Date(Date.UTC(2020, 0, 1)), {
    getFullYear: () => 2026,
    getMonth: () => 9,
    getDate: () => 8,
    getHours: () => 12,
    getMinutes: () => 30,
    getSeconds: () => 45,
  });
  expect(pastedName(new File(["x"], "image.png", { type: "image/png" }), local)).toBe(
    "Pasted image 20261008123045.png"
  );
});

test("files pasted upload, and their embeds go where the selection was, in their order, a line each", async () => {
  const { view, going, text } = editing("See it here.", { anchor: 7, head: 11 });
  const photo = new File(["png"], "image.png", { type: "image/png" });
  const sheet = new File(["csv"], "data.csv", { type: "text/csv" });

  expect(paste(view, clipboard([photo, sheet]))).toBe(true);
  // The selection goes, as a paste replaces it.
  expect(text()).toBe("See it .");
  expect(going.map(({ file }) => [file.name.replace(/\d{14}/, "<now>"), file.type])).toEqual([
    ["Pasted image <now>.png", "image/png"],
    ["data.csv", "text/csv"],
  ]);
  // Typed before them meanwhile, and where they were pasted, they go where they were, before what was typed there.
  view.dispatch({ changes: { from: 0, insert: "> " } });
  view.dispatch({ changes: { from: 9, insert: "that " }, userEvent: "input.type" });
  going[1]?.answer(attachment("data.csv"));
  await settle();
  expect(text()).toBe("> See it that .");
  going[0]?.answer(attachment("Pasted image 1.png"));
  await settle();
  expect(text()).toBe("> See it ![[Pasted image 1.png]]\n![[data.csv]]that .");
});

test("a paste with text, or without files, is CodeMirror's; so is one into a read-only editor", () => {
  const { view, going, text } = editing("x");
  const photo = new File(["png"], "cells.png", { type: "image/png" });
  paste(view, clipboard([photo], "a\tb"));
  expect(text()).toBe("xa\tb");
  // A link copied, as its address only: CodeMirror pastes it.
  paste(view, {
    types: ["text/uri-list"],
    items: [],
    getData: (type: string) => (type === "text/uri-list" ? " u" : ""),
  });
  expect(text()).toBe("xa\tb u");
  paste(view, null);
  view.dispatch({ effects: readOnly.reconfigure(readOnlyAs(true)) });
  paste(view, clipboard([photo]));
  expect(going).toEqual([]);
});

test("in Chinese the editor says it in Chinese, the names listed as Chinese lists them", async () => {
  document.documentElement.lang = "zh-CN";
  onTestFinished(() => {
    document.documentElement.lang = "en";
  });
  const { view, going, controls } = editing("", { phrases: editorPhrases(translator("zh-CN")) });
  drag(view, "drop", dropped([new File(["r"], "README"), new File(["l"], "LICENSE")], ["notes"]));
  going[0]?.answer(attachment("README"));
  going[1]?.answer(attachment("LICENSE"));
  await settle();
  // What it says of the drop's folders stays with what it says of its files.
  expect(told(controls)).toEqual([
    "文件夹不会上传：请把笔记文件夹打成 zip，在笔记本设置里导入。",
    "文件夹不会上传：请把笔记文件夹打成 zip，在笔记本设置里导入。\nREADME和LICENSE 已上传，未插入：没有扩展名的名称不能嵌入。",
  ]);
});

test("an image copied from a page, as Chromium has it, its HTML with it, is uploaded", () => {
  const { view, going } = editing("x");
  const copied = clipboard([new File(["png"], "image.png", { type: "image/png" })]);
  expect(paste(view, { ...copied, types: ["text/html", "Files"] })).toBe(true);
  expect(going).toHaveLength(1);
});

test("a folder pasted is not uploaded: the editor says to import it, the selection kept when nothing else is", () => {
  const { view, going, controls, text } = editing("hello world", { anchor: 6, head: 11 });
  expect(paste(view, { ...dropped([], ["notes"]), getData: () => "" })).toBe(true);
  expect(told(controls)).toEqual([
    "Folders are not uploaded: zip a folder of notes and import it from the notebook's settings.",
  ]);
  expect(text()).toBe("hello world");
  paste(view, { ...dropped([new File(["a"], "a.png")], ["notes"]), getData: () => "" });
  expect(going.map(({ file }) => file.name)).toEqual(["a.png"]);
  expect(text()).toBe("hello ");
});

test("one that fails inserts nothing, its row saying why; the next goes in its place", async () => {
  const { view, going, controls, text } = editing("");
  paste(view, clipboard([new File(["a"], "a.png", { type: "image/png" }), new File(["b"], "b.png")]));
  going[0]?.fail(new Error("413"));
  going[1]?.answer(attachment("b.png"));
  await settle();
  expect(text()).toBe("![[b.png]]");
  expect(told(controls)).toEqual([""]);
});

test("one that fails while those before it go is left to its row, no rejection unhandled; they go in all the same", async () => {
  const { view, going, controls, text } = editing("");
  paste(view, clipboard([new File(["a"], "a.png"), new File(["b"], "b.md"), new File(["c"], "c.png")]));
  // As a page's file is refused: before the one before it is answered.
  going[1]?.fail({ refused: "page-file" });
  going[2]?.fail(new Error("413"));
  await aTurn();
  going[0]?.answer(attachment("a.png"));
  await Promise.all(controls.going.mock.calls.map(([work]) => work));
  expect(text()).toBe("![[a.png]]");
});

test("those uploaded without an extension, or as the content was replaced, or can no longer be changed, are not inserted: the editor says so of them all as the last is answered", async () => {
  const first = editing("");
  paste(
    first.view,
    clipboard([
      new File(["r"], "README"),
      new File(["l"], "LICENSE"),
      new File(["a"], "a.png"),
      new File(["b"], "b.png"),
    ])
  );
  first.going[0]?.answer(attachment("README"));
  first.going[1]?.answer(attachment("LICENSE"));
  await settle();
  expect(told(first.controls)).toEqual([""]);
  first.view.dispatch({ effects: readOnly.reconfigure(readOnlyAs(true)) });
  first.going[2]?.answer(attachment("a.png"));
  first.going[3]?.answer(attachment("b.png"));
  await settle();
  expect(told(first.controls)).toEqual([
    "",
    "README and LICENSE uploaded, not inserted: a name without an extension cannot be embedded.\n" +
      "a.png and b.png uploaded, not inserted: the text was replaced or can no longer be changed.",
  ]);
  expect(first.text()).toBe("");

  // The next paste begins anew: what was said goes.
  first.view.dispatch({ effects: readOnly.reconfigure(readOnlyAs(false)) });
  paste(first.view, clipboard([new File(["c"], "c.png")]));
  expect(told(first.controls).at(-1)).toBe("");

  const replaced = editing("x");
  paste(replaced.view, clipboard([new File(["a"], "a.png")]));
  replaced.close();
  replaced.going[0]?.answer(attachment("a.png"));
  await settle();
  expect(told(replaced.controls)).toEqual([
    "",
    "a.png uploaded, not inserted: the text was replaced or can no longer be changed.",
  ]);
  expect(replaced.text()).toBe("x");
});

test("an embed inserted where the cursor is puts the cursor after it, as a paste does: what is typed, and pasted, next goes after", async () => {
  const { view, going, text } = editing("Notes\n");
  paste(view, clipboard([new File(["1"], "image.png", { type: "image/png" })]));
  going[0]?.answer(attachment("first.png"));
  await settle();
  expect(view.state.selection.main.head).toBe(text().length);
  view.dispatch(view.state.replaceSelection("caption"));
  paste(view, clipboard([new File(["2"], "image.png", { type: "image/png" })]));
  going[1]?.answer(attachment("second.png"));
  await settle();
  expect(text()).toBe("Notes\n![[first.png]]caption![[second.png]]");

  // A selection keeps what it holds.
  view.dispatch({ selection: { anchor: 0, head: 5 } });
  Object.assign(view, { posAtCoords: () => 0 });
  drag(view, "drop", dropped([new File(["3"], "third.png")]));
  going[2]?.answer(attachment("third.png"));
  await settle();
  expect(text()).toBe("![[third.png]]Notes\n![[first.png]]caption![[second.png]]");
  expect(view.state.sliceDoc(view.state.selection.main.from, view.state.selection.main.to)).toBe("Notes");
});

test("files pasted in two places as others go each go where they were pasted, whichever is answered first", async () => {
  const { view, going, text } = editing("ab", { anchor: 1 });
  paste(view, clipboard([new File(["a"], "a.png")]));
  view.dispatch({ selection: { anchor: 0 } });
  paste(view, clipboard([new File(["b"], "b.png")]));
  going[1]?.answer(attachment("b.png"));
  await settle();
  going[0]?.answer(attachment("a.png"));
  await settle();
  expect(text()).toBe("![[b.png]]a![[a.png]]b");
});

test("files pasted elsewhere as others go keep their own places, whichever is answered first", async () => {
  const { view, going, text } = editing("abc", { anchor: 1 });
  paste(view, clipboard([new File(["a"], "a.png")]));
  view.dispatch({ selection: { anchor: 0 } });
  paste(view, clipboard([new File(["b"], "b.png")]));
  view.dispatch({ selection: { anchor: 3 } });
  paste(view, clipboard([new File(["c"], "c.png")]));
  going[0]?.answer(attachment("a.png"));
  await settle();
  going[1]?.answer(attachment("b.png"));
  going[2]?.answer(attachment("c.png"));
  await settle();
  expect(text()).toBe("![[b.png]]a![[a.png]]bc![[c.png]]");
});

test("files pasted where others go still go after them, whichever is answered first", async () => {
  const { view, going, text } = editing("");
  paste(view, clipboard([new File(["a"], "a.png"), new File(["b"], "b.png")]));
  paste(view, clipboard([new File(["c"], "c.png")]));
  paste(view, clipboard([new File(["d"], "d.png")]));
  going[0]?.answer(attachment("a.png"));
  await settle();
  expect(text()).toBe("![[a.png]]");
  going[3]?.answer(attachment("d.png"));
  await settle();
  expect(text()).toBe("![[a.png]]![[d.png]]");
  going[2]?.answer(attachment("c.png"));
  await settle();
  expect(text()).toBe("![[a.png]]![[c.png]]![[d.png]]");
  going[1]?.answer(attachment("b.png"));
  await settle();
  expect(text()).toBe("![[a.png]]\n![[b.png]]![[c.png]]![[d.png]]");
});

test("the edit waits for the embeds to go in: each paste's or drop's work goes until its last is", async () => {
  const { view, going, controls, text } = editing("");
  paste(view, clipboard([new File(["a"], "a.png"), new File(["b"], "b.png")]));
  const [[work]] = controls.going.mock.calls as [[Promise<void>]];
  let done = false;
  void work.finally(() => {
    done = true;
  });
  going[0]?.answer(attachment("a.png"));
  await settle();
  expect(done).toBe(false);
  going[1]?.fail(new Error("413"));
  await work;
  expect(text()).toBe("![[a.png]]");
});

test("an embed waits for the input method's composition to end; one whose editor goes first is not inserted, the editor says", async () => {
  const { view, going, controls, text } = editing("");
  const waiting: { act: () => void; drop: () => void }[] = [];
  controls.whenComposed = (act, drop = () => undefined) => void waiting.push({ act, drop });
  paste(view, clipboard([new File(["a"], "a.png"), new File(["b"], "b.png")]));
  going[0]?.answer(attachment("a.png"));
  going[1]?.answer(attachment("b.png"));
  await settle();
  expect(text()).toBe("");
  waiting.shift()?.act();
  await settle();
  expect(text()).toBe("![[a.png]]");
  waiting.shift()?.drop();
  await settle();
  expect(text()).toBe("![[a.png]]");
  expect(told(controls)).toEqual([
    "",
    "b.png uploaded, not inserted: the text was replaced or can no longer be changed.",
  ]);
});

test("files dragged from outside may drop, and upload where they drop, by their own names; a folder is not uploaded: the editor says to import it", async () => {
  const { view, going, controls, text } = editing("ab");
  const files = dropped([new File(["a"], "image.png")], ["notes"]);
  expect(drag(view, "dragover", files)).toBe(true);
  expect(files.dropEffect).toBe("copy");

  // jsdom lays nothing out: where the drop is, the test says.
  const posAtCoords = vi.fn(({ x, y }: { x: number; y: number }) => (x === 30 && y === 40 ? 1 : 0) as number | null);
  Object.assign(view, { posAtCoords });
  // Where it would drop shows as it is dragged.
  drag(view, "dragover", files, { x: 30, y: 40 });
  expect(view.scrollDOM.querySelector(".cm-dropCursor")).not.toBeNull();
  expect(drag(view, "drop", files, { x: 30, y: 40 })).toBe(true);
  expect(told(controls)).toEqual([
    "Folders are not uploaded: zip a folder of notes and import it from the notebook's settings.",
  ]);
  expect(going.map(({ file }) => file.name)).toEqual(["image.png"]);
  going[0]?.answer(attachment("image.png"));
  await settle();
  expect(text()).toBe("a![[image.png]]b");

  // Off the text, where no position is, it goes where the cursor is.
  posAtCoords.mockReturnValue(null);
  view.dispatch({ selection: { anchor: 0 } });
  drag(view, "drop", dropped([new File(["c"], "c.png")]));
  going[1]?.answer(attachment("c.png"));
  await settle();
  expect(text()).toBe("![[c.png]]a![[image.png]]b");

  // Dragged from another page, a file goes with its address and its HTML: it is uploaded all the same.
  drag(view, "drop", {
    ...dropped([new File(["d"], "d.png")]),
    types: ["text/plain", "text/uri-list", "text/html", "Files"],
  });
  expect(going.map(({ file }) => file.name)).toEqual(["image.png", "c.png", "d.png"]);
});

test("a drag of no files, of pages' files only, or over a read-only editor, is CodeMirror's", async () => {
  const { view, going, text } = editing("x");
  expect(drag(view, "dragover", transferOf({ types: ["text/plain"], items: [] }))).toBe(false);
  // Pages' files: CodeMirror reads their text in.
  drag(view, "drop", transferOf(dropped([new File(["# a"], "a.md"), new File(["# b"], "b.MD")])));
  await vi.waitFor(() => expect(text()).toContain("# a"));
  // Files it cannot read, as Firefox's drag of an image from another page may be: CodeMirror's, which takes its text.
  drag(view, "drop", { types: ["Files", "text/plain"], items: [], getData: () => "TXT" });
  view.dispatch({ effects: readOnly.reconfigure(readOnlyAs(true)) });
  // Read-only, it shows no place to drop.
  Object.assign(view, { posAtCoords: () => 1 });
  expect(drag(view, "dragover", transferOf(dropped([new File(["a"], "a.png")])))).toBe(false);
  expect(view.scrollDOM.querySelector(".cm-dropCursor")).toBeNull();
  drag(view, "drop", transferOf(dropped([new File(["a"], "a.png")])));
  expect(going).toEqual([]);
  expect(text()).toMatch(/^TXT/);
});

test("a drag started in the page, which may carry a file (Chromium's image), is CodeMirror's", () => {
  const { view, going } = editing("x");
  pageDrag.on = true;
  expect(drag(view, "dragover", transferOf(dropped([new File(["a"], "a.png")])))).toBe(false);
  drag(view, "drop", transferOf(dropped([new File(["a"], "a.png")])));
  expect(going).toEqual([]);
});

test("a drop of pages' files with others is the editor's: each goes up, a page's file refused on its row", () => {
  const { view, going } = editing("x");
  expect(drag(view, "drop", transferOf(dropped([new File(["# a"], "a.md"), new File(["b"], "b.png")])))).toBe(true);
  expect(going.map(({ file }) => file.name)).toEqual(["a.md", "b.png"]);
});

test("a drop of folders only uploads nothing; one of folders and pages' files is CodeMirror's, the editor saying to import the folders", async () => {
  const { view, going, controls, text } = editing("x");
  expect(drag(view, "drop", dropped([], ["notes"]))).toBe(true);
  expect(controls.tell).toHaveBeenCalledOnce();
  // CodeMirror takes it, its default prevented too.
  drag(view, "drop", transferOf(dropped([new File(["# a"], "a.md")], ["notes"])));
  expect(told(controls)).toEqual([
    "Folders are not uploaded: zip a folder of notes and import it from the notebook's settings.",
    "Folders are not uploaded: zip a folder of notes and import it from the notebook's settings.",
  ]);
  await vi.waitFor(() => expect(text()).toContain("# a"));
  expect(going).toEqual([]);
});

test("a drop of pages' files only, no folder, says nothing", async () => {
  const { view, controls, text } = editing("x");
  drag(view, "drop", transferOf(dropped([new File(["# a"], "a.md")])));
  await vi.waitFor(() => expect(text()).toContain("# a"));
  expect(controls.tell).not.toHaveBeenCalled();
});

test("what a drop with folders says of files not inserted keeps its folders' line", async () => {
  const { view, going, controls } = editing("");
  drag(view, "drop", dropped([new File(["a"], "a.png")], ["notes"]));
  view.dispatch({ effects: readOnly.reconfigure(readOnlyAs(true)) });
  going[0]?.answer(attachment("a.png"));
  await settle();
  expect(told(controls).at(-1)).toBe(
    "Folders are not uploaded: zip a folder of notes and import it from the notebook's settings.\n" +
      "a.png uploaded, not inserted: the text was replaced or can no longer be changed."
  );
});

test("an embed inserted away from an open completion leaves it open", async () => {
  const { view, going, text } = editing("top\n\nSee [[Pl", {
    anchor: 0,
    extra: autocompletion({ override: [(context) => ({ from: context.pos - 2, options: [{ label: "Plans" }] })] }),
  });
  paste(view, clipboard([new File(["png"], "chart.png", { type: "image/png" })]));
  view.dispatch({ selection: { anchor: view.state.doc.length } });
  startCompletion(view);
  await vi.waitFor(() => expect(completionStatus(view.state)).toBe("active"));

  going[0]?.answer(attachment("chart.png"));
  await settle();
  expect(text()).toBe("![[chart.png]]top\n\nSee [[Pl");
  expect(completionStatus(view.state)).toBe("active");
});
