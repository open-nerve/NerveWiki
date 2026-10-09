import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { afterEach, expect, test } from "vitest";

import { pageDrag } from "../../lib/file-transfer";
import type { Asset } from "../../services/asset.service";
import { dropped } from "../../test/attachments";
import { assetJSON, assetNode, guide } from "../../test/page-server";
import { readOnly, readOnlyAs } from "../extensions";
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
 * upload of files; the uploads asked for wait for the test's answer, and
 * what the editor told is kept.
 */
function editing(doc: string, { anchor = doc.length, head = anchor }: { anchor?: number; head?: number } = {}) {
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
      extensions: [readOnly.of(readOnlyAs(false)), assetUpload(context, controls)],
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

/** drag sends a drag's event of type over view's content with transfer: whether it was taken (its default prevented). */
function drag(view: EditorView, type: "dragover" | "drop", transfer: unknown): boolean {
  const event = new Event(type, { bubbles: true, cancelable: true });
  Object.assign(event, { clientX: 1, clientY: 1 });
  Object.defineProperty(event, "dataTransfer", { value: transfer });
  view.contentDOM.dispatchEvent(event);
  return event.defaultPrevented;
}

/** settle lets the promises out settle. */
async function settle() {
  for (let i = 0; i < 5; i++) {
    // oxlint-disable-next-line no-await-in-loop -- a turn at a time
    await Promise.resolve();
  }
}

test("an image the clipboard holds without a name of its own is named as Obsidian names it; others keep theirs", () => {
  const now = new Date(2026, 9, 8, 12, 30, 45);
  const named = (name: string, type: string) => pastedName(new File(["x"], name, { type }), now);
  expect(named("image.png", "image/png")).toBe("Pasted image 20261008123045.png");
  expect(named("", "image/jpeg")).toBe("Pasted image 20261008123045.jpg");
  expect(named("image.webp", "image/webp")).toBe("Pasted image 20261008123045.webp");
  expect(named("", "image/x-icon")).toBe("Pasted image 20261008123045.xicon");
  expect(named("photo.png", "image/png")).toBe("photo.png");
  expect(named("image.png", "")).toBe("image.png");
  expect(named("notes.txt", "text/plain")).toBe("notes.txt");
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
  paste(view, clipboard([]));
  paste(view, null);
  view.dispatch({ effects: readOnly.reconfigure(readOnlyAs(true)) });
  paste(view, clipboard([photo]));
  expect(going).toEqual([]);
});

test("one that fails inserts nothing, its row saying why; the next goes in its place", async () => {
  const { view, going, controls, text } = editing("");
  paste(view, clipboard([new File(["a"], "a.png", { type: "image/png" }), new File(["b"], "b.png")]));
  going[0]?.fail(new Error("413"));
  going[1]?.answer(attachment("b.png"));
  await settle();
  expect(text()).toBe("![[b.png]]");
  expect(controls.tell).not.toHaveBeenCalled();
});

test("one uploaded without an extension, or as the content was replaced, or can no longer be changed, is not inserted: the editor says so", async () => {
  const first = editing("");
  paste(first.view, clipboard([new File(["r"], "README"), new File(["a"], "a.png")]));
  first.going[0]?.answer(attachment("README"));
  await settle();
  expect(first.controls.tell).toHaveBeenLastCalledWith(
    "README is in the page's attachments, not inserted: a name without an extension cannot be embedded."
  );
  first.view.dispatch({ effects: readOnly.reconfigure(readOnlyAs(true)) });
  first.going[1]?.answer(attachment("a.png"));
  await settle();
  expect(first.controls.tell).toHaveBeenLastCalledWith(
    "a.png is in the page's attachments, not inserted: the text was replaced or can no longer be changed."
  );
  expect(first.text()).toBe("");

  const replaced = editing("x");
  paste(replaced.view, clipboard([new File(["a"], "a.png")]));
  replaced.close();
  replaced.going[0]?.answer(attachment("a.png"));
  await settle();
  expect(replaced.controls.tell).toHaveBeenCalledOnce();
  expect(replaced.text()).toBe("x");
});

test("an embed waits for the input method's composition to end; one whose editor goes first is not inserted", async () => {
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
  expect(controls.tell).toHaveBeenCalledOnce();
});

test("files dragged from outside may drop, and upload where they drop; a folder is not uploaded: the editor says to import it", async () => {
  const { view, going, controls, text } = editing("ab");
  const files = dropped([new File(["a"], "a.png")], ["notes"]);
  expect(drag(view, "dragover", files)).toBe(true);
  expect(files.dropEffect).toBe("copy");

  // jsdom lays nothing out: where the drop is, the test says.
  Object.assign(view, { posAtCoords: () => 1 });
  expect(drag(view, "drop", files)).toBe(true);
  expect(controls.tell).toHaveBeenCalledWith("Folders are not uploaded: import a folder of notes instead.");
  expect(going.map(({ file }) => file.name)).toEqual(["a.png"]);
  going[0]?.answer(attachment("a.png"));
  await settle();
  expect(text()).toBe("a![[a.png]]b");

  // Off the text, where no position is, it goes where the cursor is.
  Object.assign(view, { posAtCoords: () => null });
  view.dispatch({ selection: { anchor: 0 } });
  drag(view, "drop", dropped([new File(["c"], "c.png")]));
  going[1]?.answer(attachment("c.png"));
  await settle();
  expect(text()).toBe("![[c.png]]a![[a.png]]b");
});

test("a drag of no files, of pages' files only, or over a read-only editor, is CodeMirror's", () => {
  const { view, going } = editing("x");
  expect(drag(view, "dragover", transferOf({ types: ["text/plain"], items: [] }))).toBe(false);
  drag(view, "drop", transferOf(dropped([new File(["# a"], "a.md"), new File(["# b"], "b.MD")])));
  view.dispatch({ effects: readOnly.reconfigure(readOnlyAs(true)) });
  expect(drag(view, "dragover", transferOf(dropped([new File(["a"], "a.png")])))).toBe(false);
  drag(view, "drop", transferOf(dropped([new File(["a"], "a.png")])));
  expect(going).toEqual([]);
});

test("a drag started in the page, which may carry a file (Chromium's image), is CodeMirror's", () => {
  const { view, going } = editing("x");
  pageDrag.on = true;
  expect(drag(view, "dragover", transferOf(dropped([new File(["a"], "a.png")])))).toBe(false);
  drag(view, "drop", transferOf(dropped([new File(["a"], "a.png")])));
  expect(going).toEqual([]);
});

test("a drop of folders only uploads nothing", () => {
  const { view, going, controls } = editing("x");
  expect(drag(view, "drop", dropped([], ["notes"]))).toBe(true);
  expect(controls.tell).toHaveBeenCalledOnce();
  expect(going).toEqual([]);
});
