import { EditorState, type Extension } from "@codemirror/state";

import type { Translate } from "../i18n/i18n";
import type { MessageKey } from "../i18n/messages/en";

/**
 * The phrases CodeMirror shows or announces in the editor (M4/P6 design
 * 3.5): its search panel, go to line, what the merge view says, the
 * completion's (M6/P7 design 4, 5), and what the upload of files says
 * (M7/P4 design 5.2), by their English text, each with the app's
 * message. $ stands for what is put in: a number, or names.
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
  Completions: "editor.phrase.completions",
  "$ page": "editor.phrase.page",
  "$ pages": "editor.phrase.pages",
  "$ uploaded, not inserted: the text was replaced or can no longer be changed.": "editor.phrase.uploadNotInserted",
  "$ uploaded, not inserted: a name without an extension cannot be embedded.": "editor.phrase.uploadNoLink",
  "Folders are not uploaded: import a folder of notes instead.": "asset.folders",
} as const satisfies Record<string, MessageKey>;

/** editorPhrases is CodeMirror's phrases in the language of t. */
export function editorPhrases(t: Translate): Extension {
  return EditorState.phrases.of(
    Object.fromEntries(Object.entries(phraseKeys).map(([phrase, key]) => [phrase, t(key)]))
  );
}
