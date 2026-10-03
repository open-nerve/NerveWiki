import { EditorView } from "@codemirror/view";

/**
 * editorTheme is the editor's look in the app's colours (M4/P6 design
 * 3.2): it reads the app's CSS variables, which .dark sets anew, so that
 * one theme serves both. The colours the app has none of, a link's and a
 * search match's, are the editor's own, set anew under .dark too. Lines
 * wrap: Markdown is mostly prose.
 */
export const editorTheme = [
  EditorView.lineWrapping,
  EditorView.theme({
    "&": {
      "--nw-editor-mono": "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
      "--nw-editor-link": "oklch(0.55 0.15 255)",
      "--nw-editor-match": "oklch(0.9 0.12 95 / 60%)",
      "--nw-editor-match-selected": "oklch(0.8 0.15 75 / 70%)",
      color: "var(--foreground)",
      backgroundColor: "var(--background)",
      border: "1px solid var(--border)",
      borderRadius: "var(--radius)",
      minHeight: "20rem",
    },
    ".dark &": {
      "--nw-editor-link": "oklch(0.75 0.12 255)",
      "--nw-editor-match": "oklch(0.45 0.1 95 / 60%)",
      "--nw-editor-match-selected": "oklch(0.5 0.12 75 / 70%)",
    },
    "&.cm-focused": { outline: "2px solid var(--ring)", outlineOffset: "1px" },
    ".cm-content": { padding: "1rem 0", caretColor: "var(--foreground)", lineHeight: "1.6" },
    ".cm-line": { padding: "0 1rem" },
    ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--foreground)" },
    "&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection":
      { backgroundColor: "var(--accent)" },
    ".cm-panels": { backgroundColor: "var(--background)", color: "var(--foreground)" },
    ".cm-panels.cm-panels-top": { borderBottom: "1px solid var(--border)" },
    ".cm-panels.cm-panels-bottom": { borderTop: "1px solid var(--border)" },
    ".cm-searchMatch": { backgroundColor: "var(--nw-editor-match)" },
    ".cm-searchMatch.cm-searchMatch-selected": { backgroundColor: "var(--nw-editor-match-selected)" },
  }),
];
