import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { search, searchKeymap } from "@codemirror/search";
import { Compartment, EditorState, Prec, type Text } from "@codemirror/state";
import { EditorView, keymap, type ViewUpdate } from "@codemirror/view";
import { use, useContext, useId, useImperativeHandle, useLayoutEffect, useRef, type Ref, type RefObject } from "react";

import { useT, type Translate } from "../i18n/i18n";
import { insertLink, toggleStrong } from "./commands";
import { editOutline, where, type OutlineSink } from "./edit-outline";
import { composeExtensions, loadExtensions, readOnly, readOnlyAs, ready } from "./extensions";
import { joinBreaks, lineBreaks, splitBreaks } from "./line-breaks";
import { markdownEditing } from "./markdown";
import { editorPhrases } from "./phrases";
import { markupMarks, proseLines, proseTheme } from "./prose-lines";
import {
  EditorClosed,
  EditorExtensions,
  type EditorContext,
  type EditorControls,
  type ReadyExtension,
} from "./registry";
import { editorTheme } from "./theme";

/** SourceEditorHandle is what the page's edit does with its editor. */
export type SourceEditorHandle = {
  /** text is the content to save: with its byte order mark and each line break as written. */
  text(): string;
  /** version is how many changes the content has had: each adds one. */
  version(): number;
  focus(): void;
  /**
   * whenComposed runs act now, or once the input method's composition
   * ends: never on half a word; drop instead if the editor goes first.
   */
  whenComposed(act: () => void, drop?: () => void): void;
  /** load replaces the content with raw: a new state, whose history does not reach the old content. */
  load(raw: string): void;
  /** hold keeps the content from being changed while on, whatever the extensions set: while the edit is left. */
  hold(on: boolean): void;
  /** working tells whether work of the extensions goes (EditorControls.going). */
  working(): boolean;
  /** settled settles once no work of the extensions goes: what begins as it waits is waited for too. */
  settled(): Promise<void>;
  /**
   * reveal puts the cursor at the start of the line of the outline's index-th heading, scrolled to near the window's
   * top, the editor focused (M13/P4/S3, decision 876): nothing past those told.
   */
  reveal(index: number): void;
};

type SourceEditorProps = {
  /** The content the editor opens on, as written. */
  content: string;
  /** Whether the editor takes the focus as it is made: the user asked to edit. */
  focusOnOpen?: boolean;
  context: EditorContext;
  /** The edit's controls; the editor adds its own: setReadOnly, onChange, onClose, whenComposed and going. */
  controls: Omit<EditorControls, "setReadOnly" | "onChange" | "onClose" | "whenComposed" | "going">;
  /** onChange is told the content's version after each change. */
  onChange(version: number): void;
  /** The page's outline while it is edited, told the editor's headings; none, they are not read. */
  outline?: OutlineSink;
  ref?: Ref<SourceEditorHandle>;
};

/** Live is the latest of the editor's props, which its extensions, made once, read when called. */
type Live = Pick<SourceEditorProps, "context" | "controls" | "onChange" | "outline"> & {
  t: Translate;
  registered: readonly ReadyExtension[];
};

/** How long a composition's end waits for its text, which some browsers give after it. */
const compositionSettles = 50;

/** Where a jump puts its line: at the window's top, 16px below it (decision 876). */
const jumpTo = (at: number) => EditorView.scrollIntoView(at, { y: "start", yMargin: 16 });

/**
 * How often, and how long after it, a jump's line is put back at the window's top as what it passed is measured
 * (EditorHost.reveal): the measuring that moves it is done in a few frames.
 */
const realignments = 4;
const realigning = 1000;

/** What the reader does that ends a jump's realigning: a scroll, a press, a touch, a key. */
const readerActs = ["wheel", "pointerdown", "touchstart", "keydown"] as const;

/** A jump's line kept at the window's top (EditorHost.reveal): where, in which content, how many more times. */
type Aligned = { at: number; doc: Text; left: number; end: () => void };

/** The editor's words in the language of the app: its phrases and its content's label. */
const wording = new Compartment();

/** Why the content may not be changed: an extension set it read-only, or the edit holds it. */
type Lock = "readOnly" | "held";

/** What waits on a composition: act once it ends, or drop if the editor goes first. */
type Waiting = { act: () => void; drop: () => void };

/**
 * EditorHost holds an EditorView, made once, and what goes with it: the
 * changes counted, what waits on a composition, the locks, which a new
 * content keeps, and the extensions' work going, which a new content
 * keeps too until it is done. What waits on a composition as the editor
 * goes, or after, is dropped: a save of the controls rejects (nt-3).
 */
class EditorHost {
  readonly view: EditorView;
  private changes = 0;
  private readonly waiting: Waiting[] = [];
  private settling: ReturnType<typeof setTimeout> | undefined = undefined;
  private readonly locks: Record<Lock, boolean> = { readOnly: false, held: false };
  /** Who the extensions of the state shown have asked to hear of the content's changes. */
  private readonly changed = new Set<() => void>();
  /** What goes with the state shown: its extensions' subscriptions ended, their onClose called. */
  private closing: (() => void)[] = [];
  /** The extensions' work going (EditorControls.going). */
  private readonly work = new Set<Promise<void>>();
  /** The jump whose line is kept at the window's top while what it passed is measured (reveal). */
  private aligned: Aligned | undefined = undefined;
  private destroyed = false;

  constructor(
    parent: HTMLElement,
    raw: string,
    private readonly hint: string,
    private readonly live: RefObject<Live>
  ) {
    // The view comes first: an extension may set it read-only as it is built.
    this.view = new EditorView({ parent });
    this.view.setState(this.stateOf(raw));
  }

  get version(): number {
    return this.changes;
  }

  load(raw: string): void {
    this.close();
    this.view.setState(this.stateOf(raw));
  }

  whenComposed(act: () => void, drop: () => void = () => undefined): void {
    if (this.destroyed) {
      drop();
      return;
    }
    this.waiting.push({ act, drop });
    this.runWaiting();
  }

  /** tell says text unseen, as CodeMirror announces, while the editor is there, and has the edit show it. */
  tell(text: string): void {
    if (!this.destroyed && text !== "") {
      this.view.dispatch({ effects: EditorView.announce.of(text) });
    }
    this.live.current.controls.tell(text);
  }

  going(work: Promise<void>): void {
    this.work.add(work);
    const forget = () => void this.work.delete(work);
    void work.then(forget, forget);
  }

  get working(): boolean {
    return this.work.size > 0;
  }

  async settled(): Promise<void> {
    while (this.work.size > 0) {
      // oxlint-disable-next-line no-await-in-loop -- what began meanwhile, in turn
      await Promise.allSettled(this.work);
    }
  }

  setWording(t: Translate): void {
    this.view.dispatch({ effects: wording.reconfigure(this.wordingOf(t)) });
  }

  lock(lock: Lock, on: boolean): void {
    this.locks[lock] = on;
    // Before its state is made, the state is made locked.
    if (readOnly.get(this.view.state) !== undefined) {
      this.view.dispatch({ effects: readOnly.reconfigure(readOnlyAs(this.locked)) });
    }
  }

  /**
   * reveal puts the cursor at the start of the line at, the line at the window's top, and focuses the editor (decision
   * 876). The lines a jump passes CodeMirror had estimated, not drawn: drawn and measured as it gets there, they moved
   * the line off its place, above the window on a first jump down a long page (M13 closeout, review B's M13CB-I1). So
   * each update of the measuring whose heights changed scrolls to the line once more, in a microtask of its own (no
   * dispatch in an update), the cursor and the content as they were, a few times at most within a second; the reader's
   * scroll, press, touch or key ends it.
   */
  reveal(at: number): void {
    this.aligned?.end();
    this.view.dispatch({ selection: { anchor: at }, effects: jumpTo(at) });
    this.view.focus();
    const end = () => {
      clearTimeout(timer);
      for (const name of readerActs) {
        window.removeEventListener(name, end, true);
      }
      if (this.aligned === aligned) {
        this.aligned = undefined;
      }
    };
    const timer = setTimeout(end, realigning);
    for (const name of readerActs) {
      window.addEventListener(name, end, { capture: true, passive: true });
    }
    const aligned: Aligned = { at, doc: this.view.state.doc, left: realignments, end };
    this.aligned = aligned;
  }

  destroy(): void {
    this.destroyed = true;
    this.aligned?.end();
    clearTimeout(this.settling);
    for (const { drop } of this.waiting.splice(0)) {
      drop();
    }
    this.close();
    this.view.destroy();
  }

  private get locked(): boolean {
    return this.locks.readOnly || this.locks.held;
  }

  private wordingOf(t: Translate) {
    return [
      editorPhrases(t),
      EditorView.contentAttributes.of({ "aria-label": t("editor.label"), "aria-describedby": this.hint }),
    ];
  }

  private close(): void {
    for (const done of this.closing.splice(0)) {
      done();
    }
  }

  /** realign scrolls to a jump's line again as the update measured heights that moved it (reveal). */
  private realign(update: ViewUpdate): void {
    const aligned = this.aligned;
    if (aligned === undefined || !update.heightChanged) {
      return;
    }
    const kept = (state: EditorState) => state.doc === aligned.doc && state.selection.main.head === aligned.at;
    if (!kept(update.state) || aligned.left === 0) {
      aligned.end();
      return;
    }
    aligned.left -= 1;
    queueMicrotask(() => {
      if (this.aligned === aligned && !this.destroyed && kept(this.view.state)) {
        this.view.dispatch({ effects: jumpTo(aligned.at) });
      }
    });
  }

  private runWaiting(): void {
    if (!this.view.composing) {
      for (const { act } of this.waiting.splice(0)) {
        act();
      }
    }
  }

  private stateOf(raw: string): EditorState {
    const { t, context, registered } = this.live.current;
    const split = splitBreaks(raw);
    const controls: EditorControls = {
      save: () =>
        new Promise((resolve, reject) =>
          this.whenComposed(
            () => this.live.current.controls.save().then(resolve, reject),
            () => reject(new EditorClosed())
          )
        ),
      saving: () => this.live.current.controls.saving(),
      setReadOnly: (on) => this.lock("readOnly", on),
      session: () => this.live.current.controls.session(),
      onSessionChange: (listener) => {
        const off = this.live.current.controls.onSessionChange(listener);
        this.closing.push(off);
        return off;
      },
      onChange: (listener) => {
        this.changed.add(listener);
        const off = () => void this.changed.delete(listener);
        this.closing.push(off);
        return off;
      },
      onClose: (listener) => void this.closing.push(listener),
      leave: (reason) => this.live.current.controls.leave(reason),
      whenComposed: (act, drop) => this.whenComposed(act, drop),
      tell: (text) => this.tell(text),
      going: (work) => this.going(work),
    };
    const composed = composeExtensions(registered, context, controls);
    return EditorState.create({
      doc: split.text,
      extensions: [
        lineBreaks(split),
        history(),
        markdownEditing(),
        search({ top: true }),
        keymap.of([
          { key: "Mod-b", run: toggleStrong },
          { key: "Mod-k", run: insertLink },
          indentWithTab,
          ...defaultKeymap,
          ...historyKeymap,
          ...searchKeymap,
        ]),
        editorTheme,
        // The editor's look as the page reads, over the editor's theme, which a comparison keeps (decision 874).
        Prec.high(proseTheme),
        proseLines,
        markupMarks,
        editOutline(() => this.live.current.outline),
        wording.of(this.wordingOf(t)),
        readOnly.of(readOnlyAs(this.locked)),
        EditorView.updateListener.of((update) => {
          this.realign(update);
          if (update.docChanged) {
            this.changes += 1;
            this.live.current.onChange(this.changes);
            for (const listener of this.changed) {
              listener();
            }
          }
          this.runWaiting();
        }),
        EditorView.domEventObservers({
          compositionend: () => {
            clearTimeout(this.settling);
            this.settling = setTimeout(() => this.runWaiting(), compositionSettles);
          },
        }),
        composed.extension,
      ],
    });
  }
}

/**
 * SourceEditor is the Markdown source editor (M4/P6 design 3.5). Its
 * prose is in the reading's font, its code, tables and frontmatter in
 * monospaced lines, its marks faint (M13/P4/S3, decisions 874, 889); its
 * headings are told to the page's outline, which leads to their lines
 * (reveal, decision 876). Its EditorView is made once; a content loaded
 * is a new state. The content
 * goes in and out as written: the line breaks and the byte order mark
 * are kept apart (editor/line-breaks.ts). The registered extensions come
 * after the editor's own, each in its compartment. Tab indents: a line
 * under the editor, which its content refers to, says how to move out.
 * An extension that loads what builds it is waited for, suspended.
 */
export function SourceEditor({
  content,
  focusOnOpen = false,
  context,
  controls,
  onChange,
  outline,
  ref,
}: SourceEditorProps) {
  const t = useT();
  const extensions = useContext(EditorExtensions);
  const registered = ready(extensions) ? extensions : use(loadExtensions(extensions));
  const hint = useId();
  const element = useRef<HTMLDivElement>(null);
  const editor = useRef<EditorHost>(null);
  const first = useRef({ content, focusOnOpen });
  const live = useRef<Live>({ t, context, controls, onChange, outline, registered });
  useLayoutEffect(() => {
    live.current = { t, context, controls, onChange, outline, registered };
  });
  useLayoutEffect(() => {
    if (element.current === null) {
      return undefined;
    }
    const made = new EditorHost(element.current, first.current.content, hint, live);
    editor.current = made;
    if (first.current.focusOnOpen) {
      made.view.focus();
    }
    return () => {
      made.destroy();
      editor.current = null;
    };
  }, [hint]);
  useLayoutEffect(() => editor.current?.setWording(t), [t]);
  useImperativeHandle(
    ref,
    () => ({
      text: () => (editor.current === null ? first.current.content : joinBreaks(editor.current.view.state)),
      version: () => editor.current?.version ?? 0,
      focus: () => editor.current?.view.focus(),
      whenComposed: (act, drop) => (editor.current === null ? act() : editor.current.whenComposed(act, drop)),
      load: (raw) => editor.current?.load(raw),
      hold: (on) => editor.current?.lock("held", on),
      working: () => editor.current?.working === true,
      settled: () => editor.current?.settled() ?? Promise.resolve(),
      reveal: (index) => {
        const host = editor.current;
        const at = host === null ? undefined : where(host.view, index);
        if (host === null || at === undefined) {
          return;
        }
        // The line's start: what is typed at its very start comes before the place mapped, and a heading in a quote or
        // a list item starts after its "> " or "- " (review A's R1-N3).
        host.reveal(host.view.state.doc.lineAt(at).from);
      },
    }),
    []
  );
  return (
    <div className="space-y-1">
      <div ref={element} />
      <p id={hint} className="text-xs text-muted-foreground">
        {t("editor.tabHint")}
      </p>
    </div>
  );
}
