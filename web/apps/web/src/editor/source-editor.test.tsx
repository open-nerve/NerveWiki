import { openSearchPanel } from "@codemirror/search";
import { EditorView, keymap } from "@codemirror/view";
import { act as reactAct, render, screen } from "@testing-library/react";
import { createRef, StrictMode, Suspense, type ReactNode } from "react";
import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import { I18nProvider } from "../i18n/i18n";
import type { Locale } from "../i18n/locale";
import type { OutlineSink } from "./edit-outline";
import { lockReadOnly } from "./lock-read-only";
import {
  EditorClosed,
  EditorExtensions,
  type EditorContext,
  type EditorControls,
  type EditorExtension,
} from "./registry";
import { SourceEditor, type SourceEditorHandle } from "./source-editor";

afterEach(() => vi.useRealTimers());

const context: EditorContext = {
  workspace: "lab",
  notebook: "n1",
  page: "p1",
  role: "editor",
  linkTargets: () => Promise.resolve([]),
  tags: () => Promise.resolve([]),
  uploadAsset: () => Promise.reject(new Error("no uploads")),
  refuses: () => false,
};

/** The edit's session as the controls tell it, which the test loses; following counts its listeners. */
function sessionState() {
  let lost = false;
  const listeners = new Set<() => void>();
  return {
    session: () => ({ lost }),
    onSessionChange: (listener: () => void) => {
      listeners.add(listener);
      return () => void listeners.delete(listener);
    },
    lose: () => {
      lost = true;
      for (const listener of listeners) {
        listener();
      }
    },
    following: () => listeners.size,
  };
}

/** The editor on content, its handle, its session, and what it told onChange, save, leave and tell; its outline's. */
function editor(
  content: string,
  extensions: readonly EditorExtension[] = [],
  wrap = (node: ReactNode) => node,
  outline?: OutlineSink
) {
  const handle = createRef<SourceEditorHandle>();
  const onChange = vi.fn();
  const save = vi.fn(() => Promise.resolve());
  const leave = vi.fn((_reason: "idle") => Promise.resolve());
  const tell = vi.fn((_text: string) => undefined);
  const session = sessionState();
  const tree = (locale: Locale) =>
    wrap(
      <I18nProvider locale={locale}>
        <EditorExtensions value={extensions}>
          <SourceEditor
            ref={handle}
            content={content}
            context={context}
            outline={outline}
            controls={{
              save,
              saving: () => false,
              session: session.session,
              onSessionChange: session.onSessionChange,
              leave,
              tell,
            }}
            onChange={onChange}
          />
        </EditorExtensions>
      </I18nProvider>
    );
  const { container, rerender, unmount } = render(tree("en"));
  const view = () => {
    const element = container.querySelector<HTMLElement>(".cm-editor");
    const found = element === null ? null : EditorView.findFromDOM(element);
    if (found === null) {
      throw new Error("no editor");
    }
    return found;
  };
  const type = (text: string, at = view().state.doc.length) => view().dispatch({ changes: { from: at, insert: text } });
  return {
    container,
    handle: () => handle.current as SourceEditorHandle,
    view,
    type,
    onChange,
    save,
    leave,
    tell,
    session,
    unmount,
    speak: (locale: Locale) => rerender(tree(locale)),
  };
}

/** An extension that keeps the controls it is given in controls. */
function keeping() {
  const kept: { controls?: EditorControls } = {};
  const extension: EditorExtension = {
    name: "keeping",
    extension: (_, controls) => {
      kept.controls = controls;
      return [];
    },
  };
  return { kept, extension };
}

/** settling is work the test settles: resolved, or rejected with error. */
function settling() {
  const settle: { resolve: () => void; reject: (error: Error) => void } = {
    resolve: () => undefined,
    reject: () => undefined,
  };
  const promise = new Promise<void>((resolve, reject) => {
    settle.resolve = resolve;
    settle.reject = reject;
  });
  return { promise, ...settle };
}

const editable = (view: EditorView) =>
  !view.state.readOnly && view.contentDOM.getAttribute("contenteditable") === "true";

test("under StrictMode one editor is in the page; its content is labelled", () => {
  const { container, view } = editor("# Title\n", [], (node) => <StrictMode>{node}</StrictMode>);
  expect(container.querySelectorAll(".cm-editor")).toHaveLength(1);
  expect(view().contentDOM.getAttribute("aria-label")).toBe("Page content");
});

test("gives back the content with its line breaks and byte order mark; counts the changes", () => {
  const bom = String.fromCodePoint(0xfeff);
  const { handle, type, onChange } = editor(`${bom}a\r\nb\r\n`);

  type("!", 1);
  type("?", 4);
  expect(handle().text()).toBe(`${bom}a!\r\nb?\r\n`);
  expect(onChange.mock.calls).toEqual([[1], [2]]);
  expect(handle().version()).toBe(2);
});

test("a content loaded is a new state: undo does not reach the one before", () => {
  const { handle, view, type } = editor("old\n");
  type("edit");

  handle().load("new\r\n");
  expect(handle().text()).toBe("new\r\n");
  expect(view().state.doc.toString()).toBe("new\n");
  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "z", ctrlKey: true, bubbles: true }));
  expect(handle().text()).toBe("new\r\n");
});

test("what waits on a composition runs at once when there is none", () => {
  const { handle } = editor("text");
  const act = vi.fn();

  handle().whenComposed(act);
  expect(act).toHaveBeenCalledOnce();
});

test("the registered extensions are in; their controls reach the latest save", async () => {
  const saving: EditorExtension = {
    name: "save on change",
    extension: (_, controls) => EditorView.updateListener.of((update) => void (update.docChanged && controls.save())),
  };
  const { type, save } = editor("text", [saving]);

  type("!");
  await Promise.resolve();
  expect(save).toHaveBeenCalledOnce();
});

test("an extension that loads what builds it is waited for, the editor suspended, then composed with the others", async () => {
  let go!: () => void;
  const saving: EditorExtension = {
    name: "save on change, loaded",
    load: () =>
      new Promise((resolve) => {
        go = () =>
          resolve((_, controls) =>
            EditorView.updateListener.of((update) => void (update.docChanged && controls.save()))
          );
      }),
  };
  const save = vi.fn(() => Promise.resolve());
  const session = sessionState();
  const { container } = await reactAct(async () =>
    render(
      <I18nProvider locale="en">
        <EditorExtensions value={[lockReadOnly, saving]}>
          <Suspense fallback={<p>loading</p>}>
            <SourceEditor
              content="text"
              context={context}
              controls={{
                save,
                saving: () => false,
                ...session,
                leave: () => Promise.resolve(),
                tell: () => undefined,
              }}
              onChange={() => undefined}
            />
          </Suspense>
        </EditorExtensions>
      </I18nProvider>
    )
  );
  expect(screen.getByText("loading")).toBeTruthy();
  expect(container.querySelector(".cm-editor")).toBeNull();

  await reactAct(async () => go());
  const element = container.querySelector<HTMLElement>(".cm-editor");
  const view = element === null ? null : EditorView.findFromDOM(element);
  view?.dispatch({ changes: { from: 4, insert: "!" } });
  await Promise.resolve();
  expect(save).toHaveBeenCalledOnce();
});

test("an extension whose load fails is left out; as the page around renders again, the editor stays, its content kept, the load not tried again", async () => {
  const failed = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const broken = vi.fn(() => Promise.reject(new Error("offline")));
  const extensions: readonly EditorExtension[] = [{ name: "broken", load: broken }, lockReadOnly];
  const session = sessionState();
  const tree = (saved: string) => (
    <I18nProvider locale="en">
      <EditorExtensions value={extensions}>
        <p>{saved}</p>
        <Suspense fallback={<p>loading</p>}>
          <SourceEditor
            content="text"
            context={context}
            controls={{
              save: () => Promise.resolve(),
              saving: () => false,
              ...session,
              leave: () => Promise.resolve(),
              tell: () => undefined,
            }}
            onChange={() => undefined}
          />
        </Suspense>
      </EditorExtensions>
    </I18nProvider>
  );
  const { container, rerender } = await reactAct(async () => render(tree("unsaved")));
  const view = () => {
    const element = container.querySelector<HTMLElement>(".cm-editor");
    return element === null ? null : EditorView.findFromDOM(element);
  };
  const made = view();
  expect(made).not.toBeNull();
  made?.dispatch({ changes: { from: 4, insert: " typed" } });

  for (const saved of ["saving", "saved", "unsaved"]) {
    // oxlint-disable-next-line no-await-in-loop -- one render after another
    await reactAct(async () => rerender(tree(saved)));
  }
  expect(view()).toBe(made);
  expect(made?.state.doc.toString()).toBe("text typed");
  expect(broken).toHaveBeenCalledOnce();
  expect(failed).toHaveBeenCalledOnce();
});

test("Mod+B is the editor's", () => {
  const { view, handle } = editor("word");
  view().dispatch({ selection: { anchor: 0, head: 4 } });

  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "b", ctrlKey: true, bubbles: true }));
  expect(handle().text()).toBe("**word**");
});

test("the registered extensions come after the editor's own: a key both bind is the editor's", () => {
  const run = vi.fn(() => true);
  const { view, handle } = editor("word", [{ name: "bold", extension: () => keymap.of([{ key: "Mod-b", run }]) }]);
  view().dispatch({ selection: { anchor: 0, head: 4 } });

  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "b", ctrlKey: true, bubbles: true }));
  expect(handle().text()).toBe("**word**");
  expect(run).not.toHaveBeenCalled();
});

test("an extension may set the content read-only as it is built; a content loaded keeps it; the edit's hold is apart", () => {
  const errors = vi.spyOn(console, "error");
  let built = 0;
  const locking: EditorExtension = {
    name: "locking",
    extension: (_, controls) => {
      if (built++ === 0) {
        controls.setReadOnly(true);
      }
      return [];
    },
  };
  const { kept, extension } = keeping();
  const { view, handle } = editor("text", [locking, extension]);

  expect(errors).not.toHaveBeenCalled();
  expect(editable(view())).toBe(false);
  handle().load("new");
  expect(editable(view())).toBe(false);
  kept.controls?.setReadOnly(false);
  expect(editable(view())).toBe(true);

  handle().hold(true);
  expect(editable(view())).toBe(false);
  // Read-only, the content stays focusable: the focus in it stays.
  expect(view().contentDOM.getAttribute("tabindex")).toBe("-1");
  kept.controls?.setReadOnly(true);
  kept.controls?.setReadOnly(false);
  expect(editable(view())).toBe(false);
  handle().hold(false);
  expect(editable(view())).toBe(true);
  expect(view().contentDOM.hasAttribute("tabindex")).toBe(false);
});

test("the edit's session reaches the extensions; what one follows of it is let go as a content loads, and as the editor goes", () => {
  const seen: boolean[] = [];
  const following: EditorExtension = {
    name: "following",
    extension: (_, controls) => {
      controls.onSessionChange(() => seen.push(controls.session().lost));
      return [];
    },
  };
  const { handle, session, unmount } = editor("text", [following]);

  expect(session.following()).toBe(1);
  handle().load("new");
  expect(session.following()).toBe(1);
  session.lose();
  expect(seen).toEqual([true]);
  unmount();
  expect(session.following()).toBe(0);
});

test("an extension hears of each change of the content, not of a content loaded; as a content loads and as the editor goes, it hears no more and is told", () => {
  const heard: string[] = [];
  let built = 0;
  const listening: EditorExtension = {
    name: "listening",
    extension: (_, controls) => {
      const state = ++built;
      controls.onChange(() => heard.push(`change ${state.toString()}`));
      controls.onClose(() => heard.push(`close ${state.toString()}`));
      return [];
    },
  };
  const { handle, view, type, unmount } = editor("text", [listening]);

  type("!");
  type("?");
  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "z", ctrlKey: true, bubbles: true }));
  expect(handle().text()).toBe("text");
  handle().load("new");
  type("!");
  unmount();
  expect(heard).toEqual(["change 1", "change 1", "change 1", "close 1", "change 2", "close 2"]);
});

test("under StrictMode the editor made first, and gone at once, tells its extensions it closed", () => {
  const built: number[] = [];
  const closed: number[] = [];
  const listening: EditorExtension = {
    name: "listening",
    extension: (_, controls) => {
      const state = built.push(built.length + 1);
      controls.onClose(() => closed.push(state));
      return [];
    },
  };
  const { unmount } = editor("text", [listening], (node) => <StrictMode>{node}</StrictMode>);

  expect(built).toEqual([1, 2]);
  expect(closed).toEqual([1]);
  unmount();
  expect(closed).toEqual([1, 2]);
});

test("an extension's leave reaches the edit's, with its reason", async () => {
  const { kept, extension } = keeping();
  const { leave } = editor("text", [extension]);

  await kept.controls?.leave("idle");
  expect(leave.mock.calls).toEqual([["idle"]]);
});

test("lockReadOnly sets the content read-only once the session is lost, a content loaded too; the text stays", () => {
  const { view, handle, session } = editor("text", [lockReadOnly]);

  expect(editable(view())).toBe(true);
  session.lose();
  expect(editable(view())).toBe(false);
  expect(handle().text()).toBe("text");
  handle().load("new");
  expect(editable(view())).toBe(false);
});

test("an extension's save waits for the composition's end, as Mod+S does", async () => {
  const { kept, extension } = keeping();
  const { view, save } = editor("text", [extension]);
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);

  const saved = kept.controls?.save();
  await new Promise((resolve) => setTimeout(resolve, 100));
  expect(save).not.toHaveBeenCalled();
  composing.mockReturnValue(false);
  view().contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await saved;
  expect(save).toHaveBeenCalledOnce();
});

test("an extension's save that waits for a composition rejects with EditorClosed as the editor goes; nothing is saved (nt-3)", async () => {
  const { kept, extension } = keeping();
  const { save, unmount } = editor("text", [extension]);
  vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);

  const saved = kept.controls?.save();
  unmount();
  await expect(saved).rejects.toBeInstanceOf(EditorClosed);
  expect(save).not.toHaveBeenCalled();
});

test("a content loaded keeps what waits on a composition: once it ends, an extension's save goes", async () => {
  const { kept, extension } = keeping();
  const { handle, view, save } = editor("text", [extension]);
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);

  const saved = kept.controls?.save();
  handle().load("new");
  expect(save).not.toHaveBeenCalled();
  composing.mockReturnValue(false);
  view().contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  await saved;
  expect(save).toHaveBeenCalledOnce();
});

test("what the handle has wait on a composition is dropped as the editor goes, its drop told", () => {
  const { handle, unmount } = editor("text");
  vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  const act = vi.fn();
  const drop = vi.fn();

  handle().whenComposed(act, drop);
  unmount();
  expect(drop).toHaveBeenCalledOnce();
  expect(act).not.toHaveBeenCalled();
});

test("what waits on a composition goes with the editor", async () => {
  const { handle, view, unmount } = editor("text");
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  const act = vi.fn();
  handle().whenComposed(act);
  composing.mockReturnValue(false);
  view().contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));

  unmount();
  await vi.advanceTimersByTimeAsync(100);
  expect(act).not.toHaveBeenCalled();
});

test("what an extension has wait on a composition runs once it ends, or is dropped as the editor goes", () => {
  const { kept, extension } = keeping();
  const { view, unmount } = editor("text", [extension]);
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  const act = vi.fn();
  const drop = vi.fn();

  kept.controls?.whenComposed(act, vi.fn());
  expect(act).not.toHaveBeenCalled();
  composing.mockReturnValue(false);
  view().contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true }));
  vi.advanceTimersByTime(100);
  expect(act).toHaveBeenCalledOnce();

  composing.mockReturnValue(true);
  kept.controls?.whenComposed(act, drop);
  unmount();
  expect(drop).toHaveBeenCalledOnce();
  expect(act).toHaveBeenCalledOnce();
});

test("an extension's wait on a composition after the editor went is dropped", () => {
  const { kept, extension } = keeping();
  const { unmount } = editor("text", [extension]);
  const controls = kept.controls;
  unmount();
  const act = vi.fn();
  const drop = vi.fn();

  controls?.whenComposed(act, drop);
  expect(drop).toHaveBeenCalledOnce();
  expect(act).not.toHaveBeenCalled();
});

test("what an extension tells is announced and reaches the edit; nothing told announces nothing, and reaches it to clear what was", () => {
  const { kept, extension } = keeping();
  const { tell } = editor("text", [extension]);
  const announce = vi.spyOn(EditorView.announce, "of");
  onTestFinished(() => announce.mockRestore());

  kept.controls?.tell("Said.");
  kept.controls?.tell("");
  expect(announce.mock.calls).toEqual([["Said."]]);
  expect(tell.mock.calls).toEqual([["Said."], [""]]);
});

test("an extension's work keeps the editor working until it settles, failed or not, what begins meanwhile too, a content loaded or not", async () => {
  const { kept, extension } = keeping();
  const { handle } = editor("text", [extension]);
  expect(handle().working()).toBe(false);
  const first = settling();
  const second = settling();
  kept.controls?.going(first.promise);
  expect(handle().working()).toBe(true);
  let settled = false;
  void handle()
    .settled()
    .finally(() => {
      settled = true;
    });

  reactAct(() => handle().load("other"));
  kept.controls?.going(second.promise);
  first.resolve();
  await new Promise((resolve) => {
    setTimeout(resolve, 0);
  });
  expect(settled).toBe(false);
  second.reject(new Error("failed"));
  await vi.waitFor(() => expect(settled).toBe(true));
  expect(handle().working()).toBe(false);
});

test("Tab indents; the content refers to the line that says how to move out; its words follow the app's language", () => {
  const { view, speak } = editor("text");

  const hint = document.getElementById(view().contentDOM.getAttribute("aria-describedby") ?? "");
  expect(hint?.textContent).toBe("Tab indents. To move out of the editor, press Esc, then Tab.");
  view().contentDOM.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab", bubbles: true }));
  expect(view().state.doc.toString()).toBe("  text");
  speak("zh-CN");
  expect(view().contentDOM.getAttribute("aria-label")).toBe("页面正文");
  expect(screen.getByText("Tab 键缩进。要离开编辑区，先按 Esc，再按 Tab。")).toBe(hint);
});

// The source editor as the page reads (M13/P4/S3, decisions 874, 889): its prose in the reading's font, its code,
// tables and frontmatter in lines of their own, its marks faint. jsdom cascades the editor's style modules in the order
// CodeMirror mounts them; it reads no var(), answering what is written.
test("the editor's look wins over the editor's theme: its prose's font, its content's line 1.85, its lines' padding 26px, its caret amber", () => {
  const { container } = editor("Text\n");
  const content = container.querySelector(".cm-content");
  const line = container.querySelector(".cm-line");
  const scroller = container.querySelector(".cm-scroller");
  if (content === null || line === null || scroller === null) {
    throw new Error("no content");
  }
  // A comparison's monospace, which the editor's theme has (decision 909), is not the page's editor's.
  expect(getComputedStyle(scroller).fontFamily).toBe("var(--font-sans)");
  expect(getComputedStyle(content).lineHeight).toBe("1.85");
  expect(getComputedStyle(line).paddingLeft).toBe("26px");
  // The caret is the browser's: there is no drawn one (review B's R1-M1).
  expect(getComputedStyle(content).caretColor).toBe("var(--accent)");
  expect(container.querySelector(".cm-cursor")).toBeNull();
});

test("the editor has a fence's lines monospaced and a wiki link's marks faint", () => {
  const { container } = editor("See [[Hub]].\n\n```\ncode\n```\n");
  const lines = [...container.querySelectorAll(".cm-line")];
  expect(lines.map((line) => line.classList.contains("nw-mono-line"))).toEqual([false, false, true, true, true, false]);
  expect([...container.querySelectorAll(".nw-md-faint")].map((mark) => mark.textContent)).toEqual(["[[", "]]"]);
  expect([...container.querySelectorAll(".nw-md-link")].map((mark) => mark.textContent)).toEqual(["Hub"]);
});

/** looks is an element's type as jsdom computes it: its size and weight. */
const looks = (element: Element) => [getComputedStyle(element).fontSize, getComputedStyle(element).fontWeight];

test("the editor draws the frontmatter as plain text: the line lezer reads there as a heading is not enlarged nor bold (decision 889)", () => {
  // lezer reads the YAML's last line over the closing "---" as a setext heading, as it does the body's over a "---".
  const { container } = editor("---\ntags: [a, b]\n---\n\nBody\n---\n");
  const lines = container.querySelectorAll(".cm-line");
  const heading = lines[4]?.querySelector("span");
  const yaml = lines[1];
  if (heading === null || heading === undefined || yaml === undefined) {
    throw new Error("no lines");
  }
  // jsdom resolves the highlighting's 1.065em against the prose's 15.5px.
  expect(looks(heading)).toEqual(["16.5075px", "630"]);
  const spans = [...yaml.querySelectorAll("span")];
  expect(spans.map((span) => span.textContent).join("")).toBe("tags: [a, b]");
  expect(spans.map(looks)).toEqual(spans.map(() => looks(yaml)));
  expect(looks(yaml)).toEqual(["13.5px", "400"]);
});

// Inline code in the code's type and size, 13.5px as a code line's (review B's P4S3B-M2): of the prose's 15.5px
// 0.871em, of a monospaced line's 1em; a comparison, which sets no such size, has its text's.
test("the editor's inline code is as large as its code lines: 0.871em in the prose, 1em in a table's line", () => {
  const { container } = editor("Text `code` here.\n\n| `cell` |\n|---|\n");
  const content = container.querySelector(".cm-content");
  const table = container.querySelector(".nw-mono-line");
  const codes = [...container.querySelectorAll(".cm-line span")].filter((span) =>
    ["code", "cell"].includes(span.textContent ?? "")
  );
  if (content === null || table === null) {
    throw new Error("no content");
  }
  // Of the heading's size out of a heading: --nw-heading is 1 (review A's P4S3A-F1-M1).
  expect(codes.map((code) => getComputedStyle(code).fontSize)).toEqual([
    "calc(var(--nw-editor-code, 1em) * var(--nw-heading, 1))",
    "calc(var(--nw-editor-code, 1em) * var(--nw-heading, 1))",
  ]);
  expect(getComputedStyle(content).getPropertyValue("--nw-editor-code")).toBe("0.871em");
  expect(getComputedStyle(table).getPropertyValue("--nw-editor-code")).toBe("1em");
});

// In a heading, inline code is as much larger as the heading is, as the reading view's (review A's P4S3A-F1-M1): the
// heading's rule sets --nw-heading on the code's own span, which has both, monospace's rule after it.
test("the editor's inline code in a heading grows with the heading: 1.29 in an h1, 1.065 in an h2, of the code's size", () => {
  const { container } = editor("# Using `kubectl` now\n\n## Two `x`\n\n### Three `y`\n");
  const codes = [...container.querySelectorAll(".cm-line span")].filter((span) =>
    ["kubectl", "x", "y"].includes(span.textContent ?? "")
  );
  expect(
    codes.map((code) => [getComputedStyle(code).fontSize, getComputedStyle(code).getPropertyValue("--nw-heading")])
  ).toEqual([
    ["calc(var(--nw-editor-code, 1em) * var(--nw-heading, 1))", "1.29"],
    ["calc(var(--nw-editor-code, 1em) * var(--nw-heading, 1))", "1.065"],
    ["calc(var(--nw-editor-code, 1em) * var(--nw-heading, 1))", ""],
  ]);
});

/** forcedOutlines is the outline and its offset forced colours give element, by their rules: none, out of them. */
function forcedOutlines(element: Element): string[][] {
  const found: string[][] = [];
  for (const style of document.querySelectorAll("style")) {
    for (const media of style.sheet?.cssRules ?? []) {
      if (media instanceof CSSMediaRule && media.media.mediaText === "(forced-colors: active)") {
        for (const rule of media.cssRules) {
          const focused = rule instanceof CSSStyleRule ? rule.selectorText.split(/,\s*/) : [];
          if (
            rule instanceof CSSStyleRule &&
            focused.some((selector) => selector.endsWith(":focus-visible") && element.matches(selector.slice(0, -14)))
          ) {
            found.push([rule.style.outline, rule.style.outlineOffset]);
          }
        }
      }
    }
  }
  return found;
}

// The search panel's field as wide in every engine, its controls' focus shown in forced colours too, by a transparent
// outline they draw (review B's P4S3B-F1-N1, review C's P4S3C-F1-N2; decision 910). jsdom matches no forced colours:
// their rules are read.
test("the search panel's fields are 10.5rem wide; its field and buttons focused have, in forced colours, a transparent outline", () => {
  const { view } = editor("Text\n");
  openSearchPanel(view());
  const field = view().dom.querySelector(".cm-panel.cm-search .cm-textfield");
  const button = view().dom.querySelector(".cm-panel.cm-search .cm-button");
  if (field === null || button === null) {
    throw new Error("no search panel");
  }
  // jsdom's 10.5rem, at 16px the rem.
  expect(getComputedStyle(field).width).toBe("168px");
  expect(getComputedStyle(field).outline).toBe("none");
  expect([forcedOutlines(field), forcedOutlines(button)]).toEqual([
    [["2px solid transparent", "2px"]],
    [["2px solid transparent", "2px"]],
  ]);
});

/** coloured is the text of the line's spans of each colour jsdom computes, by colour, in order. */
function coloured(line: Element): Record<string, string> {
  const by: Record<string, string> = {};
  const walker = document.createTreeWalker(line, NodeFilter.SHOW_TEXT);
  for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
    const colour = node.parentElement === null ? "" : getComputedStyle(node.parentElement).color;
    by[colour] = (by[colour] ?? "") + (node.textContent ?? "");
  }
  return by;
}

// In a quote, the Markdown's marks and an address are faint as out of one, its text secondary (review B's P4S3B-N1).
test("in a quote the marks of emphasis, a link's and code's, and an address are faint; its text secondary", () => {
  const { container } = editor("> a **b** [x](https://x.test) `c`\n");
  const line = container.querySelector(".cm-line");
  if (line === null) {
    throw new Error("no line");
  }
  expect(coloured(line)).toEqual({
    "var(--faint)": ">****[](https://x.test)``",
    "var(--muted-foreground)": " a b x c",
  });
});

// The outline while the page is edited leads to a heading's line (M13/P4/S3, decision 876).
test.each([
  ["# One\n\nText\n\n## Two\n", 5, "## Two"],
  // In a quote, the cursor at the line's start, before its mark (review A's R1-N3).
  ["# One\n\nText\n\n> ## Quoted\n", 5, "> ## Quoted"],
])(
  "reveal puts the cursor at the start of the heading's line, scrolled to near the top, the editor focused: %j",
  (content, line, text) => {
    const told: string[][] = [];
    const sink: OutlineSink = { upTo: 10, told: (listed) => void told.push(listed.map((heading) => heading.text)) };
    const scrolled = vi.spyOn(EditorView, "scrollIntoView");
    const { view, handle } = editor(content, [], undefined, sink);
    expect(told.at(-1)?.length).toBe(2);
    const dispatched = vi.spyOn(view(), "dispatch");

    handle().reveal(1);
    const { head } = view().state.selection.main;
    expect(head).toBe(view().state.doc.line(line).from);
    expect(view().state.doc.lineAt(head).text).toBe(text);
    expect(document.activeElement).toBe(view().contentDOM);
    expect(scrolled).toHaveBeenCalledWith(head, { y: "start", yMargin: 16 });
    const effect = scrolled.mock.results.at(-1)?.value as unknown;
    expect(dispatched.mock.calls.at(-1)?.[0]).toMatchObject({ effects: effect });
    // Beyond those listed, nothing.
    handle().reveal(2);
    expect(view().state.selection.main.head).toBe(head);
  }
);
