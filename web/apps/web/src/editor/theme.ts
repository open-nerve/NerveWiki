import { EditorView } from "@codemirror/view";

/**
 * editorTheme is the editor's look in the app's colours (M4/P6 design
 * 3.2): it reads the app's CSS variables, which .dark sets anew, so that
 * one theme serves both; a search's matches are the reading view's
 * highlights (reading.css; M13/P4/S3, decision 875): it writes no colour of
 * its own. Lines wrap: Markdown is mostly prose. The content is 20rem high
 * at least. The source editor reads in the reading's font over it
 * (prose-lines.ts); a comparison and a conflict keep it, monospaced.
 *
 * CodeMirror's own look of its panels, fields, buttons and gutters is a
 * light editor's, white and grey in the dark too: the search panel is the
 * app's instead, on the editor's background, its field the app's input,
 * its buttons its small outline buttons, its corners the editor's; a
 * comparison's gutter on its background (M13/P4/S3, decision 910; review
 * B's P4S3B-I1). Variables alone: each theme sets them anew.
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
    ".cm-panels": { backgroundColor: "inherit", color: "var(--foreground)" },
    ".cm-panels.cm-panels-top": {
      borderBottom: "1px solid var(--border)",
      borderTopLeftRadius: "inherit",
      borderTopRightRadius: "inherit",
    },
    ".cm-panels.cm-panels-bottom": {
      borderTop: "1px solid var(--border)",
      borderBottomLeftRadius: "inherit",
      borderBottomRightRadius: "inherit",
    },
    ".cm-panel.cm-search": {
      padding: "6px 2.25rem 6px 10px",
      fontFamily: "var(--font-sans)",
      fontSize: "var(--text-compact)",
    },
    ".cm-panel.cm-search label": { fontSize: "inherit", color: "var(--muted-foreground)" },
    ".cm-panel.cm-search input[type=checkbox]": { accentColor: "var(--accent)" },
    ".cm-panel.cm-search [name=close]": {
      top: "6px",
      right: "8px",
      width: "1.875rem",
      height: "1.875rem",
      borderRadius: "var(--radius-row)",
      color: "var(--muted-foreground)",
      fontSize: "1.125rem",
      lineHeight: "1",
    },
    ".cm-panel.cm-search [name=close]:hover": { backgroundColor: "var(--hover)", color: "var(--foreground)" },
    // The app's input (components/ui/input.tsx), as high as a small button.
    ".cm-textfield": {
      height: "1.875rem",
      padding: "0 0.625rem",
      fontSize: "inherit",
      color: "var(--foreground)",
      backgroundColor: "var(--surface-2)",
      border: "1px solid var(--border)",
      borderRadius: "var(--radius-control)",
      outline: "none",
    },
    ".cm-textfield::placeholder": { color: "var(--faint)" },
    ".cm-textfield:focus-visible": {
      backgroundColor: "var(--surface)",
      borderColor: "var(--ring)",
      boxShadow: "0 0 0 2px var(--ring)",
    },
    // The app's small outline button (components/ui/button.tsx): CodeMirror's gradient none, hovered and pressed the
    // app's layers.
    ".cm-button": {
      height: "1.875rem",
      padding: "0 0.8125rem",
      fontSize: "inherit",
      fontWeight: "500",
      color: "var(--foreground)",
      backgroundColor: "var(--surface)",
      backgroundImage: "none",
      border: "1px solid var(--border)",
      borderRadius: "var(--radius-control)",
    },
    ".dark & .cm-button": { backgroundColor: "var(--surface-2)" },
    ".cm-button:hover": { backgroundImage: "linear-gradient(var(--hover), var(--hover))" },
    ".cm-button:active": { backgroundImage: "linear-gradient(var(--active), var(--active))" },
    ".cm-button:focus-visible": { boxShadow: "0 0 0 2px var(--ring)", outline: "none" },
    // Forced colours draw no ring: a transparent outline, which they show, as the app's controls' (outline-hidden).
    "@media (forced-colors: active)": {
      ".cm-textfield:focus-visible, .cm-button:focus-visible": {
        outline: "2px solid transparent",
        outlineOffset: "2px",
      },
    },
    // Where merge marks the lines changed: on the view's background, not CodeMirror's light grey.
    ".cm-gutters": { backgroundColor: "transparent", color: "var(--muted-foreground)", border: "none" },
    ".cm-searchMatch": { backgroundColor: "var(--nw-mark-background)" },
    ".cm-searchMatch.cm-searchMatch-selected": { backgroundColor: "var(--nw-match-selected)" },
  }),
];
