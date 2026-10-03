import { unifiedMergeView } from "@codemirror/merge";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { useLayoutEffect, useRef } from "react";

import { useT } from "../i18n/i18n";
import { splitBreaks } from "./line-breaks";
import { markdownEditing } from "./markdown";
import { editorTheme } from "./theme";

type ConflictDiffProps = {
  /** The page's content as it is now, as written. */
  theirs: string;
  /** The user's text, as written. */
  mine: string;
};

/**
 * ConflictDiff shows what the user's text changes of the page as it is
 * now (M4/P6 design 3.8): the user's text, read-only, the lines it adds
 * marked and those it would remove shown deleted, the unchanged stretches
 * folded. Both are compared as the editor holds them, every line break an
 * LF: a difference of line breaks alone does not show.
 */
export function ConflictDiff({ theirs, mine }: ConflictDiffProps) {
  const t = useT();
  const element = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    if (element.current === null) {
      return undefined;
    }
    const view = new EditorView({
      parent: element.current,
      state: EditorState.create({
        doc: splitBreaks(mine).text,
        extensions: [
          EditorState.readOnly.of(true),
          EditorView.editable.of(false),
          markdownEditing(),
          editorTheme,
          EditorView.contentAttributes.of({ "aria-label": t("editor.conflictDiff") }),
          unifiedMergeView({
            original: splitBreaks(theirs).text,
            mergeControls: false,
            collapseUnchanged: { margin: 2, minSize: 4 },
          }),
        ],
      }),
    });
    return () => view.destroy();
  }, [theirs, mine, t]);
  return <div ref={element} />;
}
