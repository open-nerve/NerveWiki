import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { search, searchKeymap } from "@codemirror/search";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { useContext, useImperativeHandle, useLayoutEffect, useRef, type Ref, type RefObject } from "react";

import { useT, type Translate } from "../i18n/i18n";
import { insertLink, toggleStrong } from "./commands";
import { composeExtensions, readOnly, readOnlyAs } from "./extensions";
import { joinBreaks, lineBreaks, splitBreaks } from "./line-breaks";
import { markdownEditing } from "./markdown";
import { editorPhrases } from "./phrases";
import { EditorExtensions, type EditorContext, type EditorControls, type EditorExtension } from "./registry";
import { editorTheme } from "./theme";

/** SourceEditorHandle is what the page's edit does with its editor. */
export type SourceEditorHandle = {
  /** text is the content to save: with its byte order mark and each line break as written. */
  text(): string;
  /** version is how many changes the content has had: each adds one. */
  version(): number;
  focus(): void;
  /** whenComposed runs act now, or once the input method's composition ends: never on half a word. */
  whenComposed(act: () => void): void;
  /** load replaces the content with raw: a new state, whose history does not reach the old content. */
  load(raw: string): void;
};

type SourceEditorProps = {
  /** The content the editor opens on, as written. */
  content: string;
  context: EditorContext;
  controls: Omit<EditorControls, "setReadOnly">;
  /** onChange is told the content's version after each change. */
  onChange(version: number): void;
  ref?: Ref<SourceEditorHandle>;
};

/** Live is the latest of the editor's props, which its extensions, made once, read when called. */
type Live = Pick<SourceEditorProps, "context" | "controls" | "onChange"> & {
  t: Translate;
  registered: readonly EditorExtension[];
};

/** How long a composition's end waits for its text, which some browsers give after it. */
const compositionSettles = 50;

const phrases = new Compartment();

/** EditorHost holds an EditorView, made once, and what goes with it: the changes counted, what waits on a composition. */
class EditorHost {
  readonly view: EditorView;
  private changes = 0;
  private readonly waiting: (() => void)[] = [];

  constructor(
    parent: HTMLElement,
    raw: string,
    private readonly live: RefObject<Live>
  ) {
    this.view = new EditorView({ parent, state: this.stateOf(raw) });
  }

  get version(): number {
    return this.changes;
  }

  load(raw: string): void {
    this.view.setState(this.stateOf(raw));
  }

  whenComposed(act: () => void): void {
    this.waiting.push(act);
    this.runWaiting();
  }

  setPhrases(t: Translate): void {
    this.view.dispatch({ effects: phrases.reconfigure(editorPhrases(t)) });
  }

  private runWaiting(): void {
    if (!this.view.composing) {
      for (const act of this.waiting.splice(0)) {
        act();
      }
    }
  }

  private stateOf(raw: string): EditorState {
    const { t, context, registered } = this.live.current;
    const split = splitBreaks(raw);
    const controls: EditorControls = {
      save: () => this.live.current.controls.save(),
      saving: () => this.live.current.controls.saving(),
      setReadOnly: (on) => this.view.dispatch({ effects: readOnly.reconfigure(readOnlyAs(on)) }),
    };
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
        phrases.of(editorPhrases(t)),
        readOnly.of(readOnlyAs(false)),
        EditorView.contentAttributes.of({ "aria-label": t("editor.label") }),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            this.changes += 1;
            this.live.current.onChange(this.changes);
          }
          this.runWaiting();
        }),
        EditorView.domEventObservers({
          compositionend: () => {
            setTimeout(() => this.runWaiting(), compositionSettles);
          },
        }),
        composeExtensions(registered, context, controls).extension,
      ],
    });
  }
}

/**
 * SourceEditor is the Markdown source editor (M4/P6 design 3.5). Its
 * EditorView is made once; a content loaded is a new state. The content
 * goes in and out as written: the line breaks and the byte order mark
 * are kept apart (editor/line-breaks.ts). The registered extensions come
 * after the editor's own, each in its compartment.
 */
export function SourceEditor({ content, context, controls, onChange, ref }: SourceEditorProps) {
  const t = useT();
  const registered = useContext(EditorExtensions);
  const element = useRef<HTMLDivElement>(null);
  const editor = useRef<EditorHost>(null);
  const first = useRef(content);
  const live = useRef<Live>({ t, context, controls, onChange, registered });
  useLayoutEffect(() => {
    live.current = { t, context, controls, onChange, registered };
  });
  useLayoutEffect(() => {
    if (element.current === null) {
      return undefined;
    }
    const made = new EditorHost(element.current, first.current, live);
    editor.current = made;
    return () => {
      made.view.destroy();
      editor.current = null;
    };
  }, []);
  useLayoutEffect(() => editor.current?.setPhrases(t), [t]);
  useImperativeHandle(
    ref,
    () => ({
      text: () => (editor.current === null ? first.current : joinBreaks(editor.current.view.state)),
      version: () => editor.current?.version ?? 0,
      focus: () => editor.current?.view.focus(),
      whenComposed: (act) => (editor.current === null ? act() : editor.current.whenComposed(act)),
      load: (raw) => editor.current?.load(raw),
    }),
    []
  );
  return <div ref={element} />;
}
