import { EditorState } from "@codemirror/state";
import { expect, test } from "vitest";

import { translator } from "../i18n/i18n";
import { editorPhrases, phraseKeys } from "./phrases";

const phrases = Object.entries(phraseKeys);

test.each(phrases)("the English message of %s is CodeMirror's own phrase", (phrase, key) => {
  expect(translator("en")(key)).toBe(phrase);
});

test.each(phrases.filter(([phrase]) => phrase.includes("$")))(
  "the Chinese message of %s keeps the $ CodeMirror fills",
  (_, key) => {
    expect(translator("zh-CN")(key)).toContain("$");
  }
);

test("the editor's state says the phrases in the interface's language", () => {
  const state = EditorState.create({ extensions: editorPhrases(translator("zh-CN")) });
  expect(state.phrase("replace all")).toBe("全部替换");
  expect(state.phrase("replaced $ matches", 3)).toBe("已替换 3 处");
});
