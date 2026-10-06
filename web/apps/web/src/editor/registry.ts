import type { Extension } from "@codemirror/state";
import { createContext } from "react";

import type { NotebookRole } from "../services/notebook.service";
import { autosave } from "./autosave";
import { idleExit } from "./idle-exit";
import { lockReadOnly } from "./lock-read-only";

/** EditorContext is the page an editor's extension is built for. */
export type EditorContext = {
  /** The workspace's slug. */
  workspace: string;
  notebook: string;
  page: string;
  role: NotebookRole;
};

/**
 * EditorClosed is why a save that waited for a composition to end did not
 * go: the editor went first (M5/P5 design 3.3).
 */
export class EditorClosed extends Error {
  constructor() {
    super("the editor closed before its composition ended");
    this.name = "EditorClosed";
  }
}

/** EditorControls is what an editor's extension may do to the edit. */
export type EditorControls = {
  /**
   * save saves the content as Mod+S does, once a composition ends; with a
   * conflict open it does nothing, the conflict's panel deciding. It
   * rejects with EditorClosed when the editor goes while it waits.
   */
  save(): Promise<void>;
  /** saving tells whether a save is out. */
  saving(): boolean;
  setReadOnly(readOnly: boolean): void;
  /**
   * session is the state of the edit's session (M5 design 4.9): lost once
   * it may no longer write, its session taken over or unlocked, the page
   * gone or out of reach. It never comes back.
   */
  session(): { lost: boolean };
  /** onSessionChange calls listener after each change of session; it returns the unsubscribe. */
  onSessionChange(listener: () => void): () => void;
  /**
   * onChange calls listener after each change of the content: typing,
   * undoing, each step of a composition; a content loaded is no change.
   * It returns the unsubscribe.
   */
  onChange(listener: () => void): () => void;
  /**
   * onClose calls listener once the extension's state goes: a content
   * loaded, or the editor gone. The extension's subscriptions end then.
   */
  onClose(listener: () => void): void;
  /**
   * leave leaves the edit as Done does, saying why on the reading view
   * (M5/P5 design 3.6): idle, after a long time without input. An edit
   * that cannot be left, its session lost, a conflict open or its save
   * failed, stays; the promise settles all the same.
   */
  leave(reason: "idle"): Promise<void>;
};

/** Build builds an editor's extension for its context and controls. */
export type Build = (context: EditorContext, controls: EditorControls) => Extension;

/** A ReadyExtension builds its extension itself. */
export type ReadyExtension = { name: string; extension: Build };

/**
 * An EditorExtension adds to the source editor (M4 design 8; M4/P6 design
 * 3.4): M5 read-only and autosave, M6 completion, M7 pasted uploads. Its
 * extension is built once per content the editor loads, and goes into a
 * compartment of its own, in the order of the registry, after the
 * editor's own. It builds it itself, holding only types of CodeMirror, or
 * loads what builds it (M6/P7 design 2): a module of the editor's chunk,
 * under editor/loaded/, which may use CodeMirror's values, imported as the
 * editor opens.
 */
export type EditorExtension = ReadyExtension | { name: string; load: () => Promise<Build> };

/**
 * editorExtensions is the registry: M5's read-only while the edit's
 * session is lost first, then its autosave and idle exit (M5/P5 design
 * 3.4, 3.5). The composition root gives it to the editor
 * through EditorExtensions. This module, and the extensions it registers,
 * hold only types of CodeMirror, so that the main chunk does not load it.
 */
export const editorExtensions: readonly EditorExtension[] = [lockReadOnly, autosave, idleExit];

export const EditorExtensions = createContext<readonly EditorExtension[]>([]);
