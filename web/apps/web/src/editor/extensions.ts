import { Compartment, EditorState, type Extension, type StateEffect } from "@codemirror/state";
import { EditorView } from "@codemirror/view";

import type { EditorContext, EditorControls, EditorExtension } from "./registry";

/** Composed is the registered extensions, built, each in its compartment. */
export type Composed = {
  extension: Extension;
  /**
   * reconfigure is the effect that replaces the extension named name with
   * next, or unloads it with null; undefined for a name not registered.
   * Only what the extension put in the editor's state goes: what it set
   * up through its controls (subscriptions, timers: M5's autosave and idle
   * exit) stays until the state goes.
   */
  reconfigure(name: string, next: Extension | null): StateEffect<unknown> | undefined;
};

/**
 * composeExtensions builds registered for context and controls (M4/P6
 * design 3.4), in their order, each in a compartment of its own, so that
 * one can be replaced or unloaded alone. One whose building throws is
 * left out, the others are not.
 */
export function composeExtensions(
  registered: readonly EditorExtension[],
  context: EditorContext,
  controls: EditorControls
): Composed {
  const compartments = new Map<string, Compartment>();
  const extension = registered.map(({ name, extension: build }) => {
    const compartment = new Compartment();
    compartments.set(name, compartment);
    let built: Extension = [];
    try {
      built = build(context, controls);
    } catch (error) {
      console.error(`The editor's extension ${name} failed`, error);
    }
    return compartment.of(built);
  });
  return {
    extension,
    reconfigure: (name, next) => compartments.get(name)?.reconfigure(next ?? []),
  };
}

/**
 * readOnly is the editor's own compartment for whether its content can be
 * changed (v0.1 design 9.3): what setReadOnly switches.
 */
export const readOnly = new Compartment();

/**
 * readOnlyAs is readOnly's content for on. A content that cannot be
 * changed stays focusable, out of the tab order: the focus in it stays
 * there as it is set read-only.
 */
export function readOnlyAs(on: boolean): Extension {
  return [
    EditorState.readOnly.of(on),
    EditorView.editable.of(!on),
    on ? EditorView.contentAttributes.of({ tabindex: "-1" }) : [],
  ];
}
