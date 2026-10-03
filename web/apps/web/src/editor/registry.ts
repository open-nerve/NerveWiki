import type { Extension } from "@codemirror/state";
import { createContext } from "react";

import type { NotebookRole } from "../services/notebook.service";
import { lockReadOnly } from "./lock-read-only";

/** EditorContext is the page an editor's extension is built for. */
export type EditorContext = {
  /** The workspace's slug. */
  workspace: string;
  notebook: string;
  page: string;
  role: NotebookRole;
};

/** EditorControls is what an editor's extension may do to the edit. */
export type EditorControls = {
  /** save saves the content, as Mod+S does. */
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
};

/**
 * An EditorExtension adds to the source editor (M4 design 8; M4/P6 design
 * 3.4): M5 read-only and autosave, M6 completion, M7 pasted uploads. Its
 * extension is built once per content the editor loads, and goes into a
 * compartment of its own, in the order of the registry, after the
 * editor's own.
 */
export type EditorExtension = {
  name: string;
  extension(context: EditorContext, controls: EditorControls): Extension;
};

/**
 * editorExtensions is the registry: M5's read-only while the edit's
 * session is lost first. The composition root gives it to the editor
 * through EditorExtensions. This module, and the extensions it registers,
 * hold only types of CodeMirror, so that the main chunk does not load it.
 */
export const editorExtensions: readonly EditorExtension[] = [lockReadOnly];

export const EditorExtensions = createContext<readonly EditorExtension[]>([]);
