import { EditorView } from "@codemirror/view";

/**
 * editorTheme is the editor's look in the app's colours (M4/P6 design
 * 3.2): it reads the app's CSS variables, which .dark sets anew, so that
 * one theme serves both; a search's matches are the reading view's
 * highlights (reading.css; M13/P4/S3, decision 875): it writes no colour of
 * its own. Lines wrap: Markdown is mostly prose. The content is 20rem high
 * at least. The source editor reads in the reading's font over it
 * (prose-lines.ts); a comparison and a conflict keep it, monospaced.
 */
export const editorTheme = [
  EditorView.lineWrapping,
  EditorView.theme({
    "&": {
      "--nw-editor-mono": "var(--font-mono)",
      color: "var(--foreground)",
      backgroundColor: "var(--background)",
      border: "1px solid var(--border)",
      borderRadius: "12px",
    },
    "&.cm-focused": { outline: "2px solid var(--ring)", outlineOffset: "1px" },
    // The content fills the editor, however short: what is clicked or dropped below its text is the content's.
    ".cm-content": { minHeight: "20rem", padding: "1rem 0", caretColor: "var(--foreground)", lineHeight: "1.6" },
    ".cm-line": { padding: "0 1rem" },
    ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--foreground)" },
    "&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection":
      { backgroundColor: "var(--accent-soft)" },
    ".cm-panels": { backgroundColor: "var(--background)", color: "var(--foreground)" },
    ".cm-panels.cm-panels-top": { borderBottom: "1px solid var(--border)" },
    ".cm-panels.cm-panels-bottom": { borderTop: "1px solid var(--border)" },
    ".cm-searchMatch": { backgroundColor: "var(--nw-mark-background)" },
    ".cm-searchMatch.cm-searchMatch-selected": { backgroundColor: "var(--nw-match-selected)" },
  }),
];
