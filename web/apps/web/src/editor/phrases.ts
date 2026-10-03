import { EditorState, type Extension } from "@codemirror/state";

import type { Translate } from "../i18n/i18n";
import type { MessageKey } from "../i18n/messages/en";

/**
 * The phrases CodeMirror shows or announces in the editor (M4/P6 design
 * 3.5): its search panel, go to line, and what the merge view says, by
 * their English text, each with the app's message. $ stands for the
 * number CodeMirror puts in.
 */
export const phraseKeys = {
  Find: "editor.phrase.find",
  Replace: "editor.phrase.replace",
  next: "editor.phrase.next",
  previous: "editor.phrase.previous",
  all: "editor.phrase.all",
  "match case": "editor.phrase.matchCase",
  regexp: "editor.phrase.regexp",
  "by word": "editor.phrase.byWord",
  replace: "editor.phrase.replaceOne",
  "replace all": "editor.phrase.replaceAll",
  close: "editor.phrase.close",
  "current match": "editor.phrase.currentMatch",
  "on line": "editor.phrase.onLine",
  "replaced $ matches": "editor.phrase.replacedMatches",
  "replaced match on line $": "editor.phrase.replacedOnLine",
  "Go to line": "editor.phrase.goToLine",
  go: "editor.phrase.go",
  "Control character": "editor.phrase.controlCharacter",
  "Selection deleted": "editor.phrase.selectionDeleted",
  "$ unchanged lines": "editor.phrase.unchangedLines",
} as const satisfies Record<string, MessageKey>;

/** editorPhrases is CodeMirror's phrases in the language of t. */
export function editorPhrases(t: Translate): Extension {
  return EditorState.phrases.of(
    Object.fromEntries(Object.entries(phraseKeys).map(([phrase, key]) => [phrase, t(key)]))
  );
}
