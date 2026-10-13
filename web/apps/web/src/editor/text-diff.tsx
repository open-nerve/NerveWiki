import { Change, diff, getChunks, goToNextChunk, goToPreviousChunk, unifiedMergeView } from "@codemirror/merge";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";

import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { compareInWorker, compareNow } from "../lib/line-diff-client";
import type { LineDiffResult, TextChange } from "../lib/line-diff";
import { splitBreaks } from "./line-breaks";
import { markdownEditing } from "./markdown";
import { editorPhrases } from "./phrases";
import { editorTheme } from "./theme";

type TextDiffProps = {
  /** The text before, as written. */
  before: string;
  /** The text after, as written. */
  after: string;
  /** The diff's accessible name. */
  label: string;
  /** What the two texts are, for the legend and the side by side's headings. */
  beforeName: string;
  afterName: string;
  /** Whether the unchanged stretches are shown: folded otherwise. */
  unfolded?: boolean;
  /** How a comparison too large for the page's thread runs: in a worker; the tests give their own. */
  compareLarge?: typeof compareInWorker;
};

/** The comparison of the texts a view shows: running, or its result. */
type Compared = { a: string; b: string; result: LineDiffResult };

/**
 * TextDiff shows what changes from one text to another (M8 design 4.3):
 * both as the editor holds them, every line break an LF and no byte order
 * mark, compared line by line (lib/line-diff.ts), on the page's thread
 * when they are small and in a worker otherwise. The changes show in a
 * read-only unified view of the text after, the lines it adds marked with
 * + and those it removes shown struck through with −, a change of a few
 * lines refined to its characters; the unchanged stretches are folded
 * unless unfolded. Above it, how many changes there are, Previous and
 * Next, which move to them, and the legend. Texts too far apart to
 * compare line by line show side by side, read-only, with as much as is
 * known of how far. Texts the same once split say so, or that only their
 * line breaks or byte order mark differ.
 */
export function TextDiff({
  before,
  after,
  label,
  beforeName,
  afterName,
  unfolded = false,
  compareLarge = compareInWorker,
}: TextDiffProps) {
  const t = useT();
  const split = useMemo(() => {
    const a = splitBreaks(before);
    const b = splitBreaks(after);
    return { a: a.text, b: b.text, breaksOnly: a.text === b.text && before !== after };
  }, [before, after]);
  const now = useMemo(() => {
    const result = compareNow(split.a, split.b);
    return result === undefined ? undefined : { a: split.a, b: split.b, result };
  }, [split]);
  const [later, setLater] = useState<Compared>();
  useEffect(() => {
    if (now !== undefined) {
      return undefined;
    }
    const controller = new AbortController();
    compareLarge(split.a, split.b, controller.signal).then(
      (result) => setLater({ a: split.a, b: split.b, result }),
      () => undefined
    );
    return () => controller.abort();
  }, [now, split, compareLarge]);
  const compared = now ?? (later?.a === split.a && later.b === split.b ? later : undefined);

  if (split.a === split.b) {
    return <p className="text-sm text-muted-foreground">{t(split.breaksOnly ? "diff.breaksOnly" : "diff.same")}</p>;
  }
  if (compared === undefined) {
    return (
      <p className="text-sm text-muted-foreground" aria-busy="true">
        {t("diff.comparing")}
      </p>
    );
  }
  const legend = t("diff.legend", { before: beforeName, after: afterName });
  const { result } = compared;
  if (result.kind === "tooLarge") {
    return (
      <div className="space-y-2">
        <p className="text-sm text-muted-foreground">
          {result.added === null || result.removed === null
            ? t("diff.tooLarge")
            : t("diff.tooLargeLines", { added: result.added, removed: result.removed })}
        </p>
        <div className="grid gap-2 md:grid-cols-2">
          <ReadOnlyText text={compared.a} name={beforeName} />
          <ReadOnlyText text={compared.b} name={afterName} />
        </div>
      </div>
    );
  }
  return (
    <UnifiedDiff
      compared={compared}
      changes={result.changes}
      label={label}
      legend={legend}
      lines={{ added: result.added, removed: result.removed }}
      unfolded={unfolded}
    />
  );
}

type UnifiedDiffProps = {
  compared: Compared;
  changes: readonly TextChange[];
  label: string;
  legend: string;
  lines: { added: number; removed: number };
  unfolded: boolean;
};

/**
 * UnifiedDiff is the text after, read-only, with what changed from the
 * text before: @codemirror/merge's unified view, given the changes the
 * comparison found (diffConfig.override) rather than computing its own.
 */
function UnifiedDiff({ compared, changes, label, legend, lines, unfolded }: UnifiedDiffProps) {
  const t = useT();
  const element = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView>(undefined);
  const [chunks, setChunks] = useState(0);
  useLayoutEffect(() => {
    if (element.current === null) {
      return undefined;
    }
    const found = changes.map((c) => new Change(c.fromA, c.toA, c.fromB, c.toB));
    // The view compares the texts the comparison did: any other pair, which it never asks for, is compared afresh.
    const override = (a: string, b: string) =>
      a === compared.a && b === compared.b ? found : diff(a, b, { scanLimit: 500 });
    const editor = new EditorView({
      parent: element.current,
      state: EditorState.create({
        doc: compared.b,
        extensions: [
          EditorState.readOnly.of(true),
          EditorView.editable.of(false),
          markdownEditing(),
          editorTheme,
          diffTheme,
          editorPhrases(t),
          EditorView.contentAttributes.of({ "aria-label": label }),
          unifiedMergeView({
            original: compared.a,
            mergeControls: false,
            diffConfig: { override },
            ...(unfolded ? {} : { collapseUnchanged: { margin: 2, minSize: 4 } }),
          }),
        ],
      }),
    });
    view.current = editor;
    setChunks(getChunks(editor.state)?.chunks.length ?? 0);
    return () => {
      view.current = undefined;
      editor.destroy();
    };
  }, [compared, changes, label, t, unfolded]);
  const move = (command: typeof goToNextChunk) => {
    const editor = view.current;
    if (editor !== undefined) {
      command({ state: editor.state, dispatch: (tr) => editor.dispatch(tr) });
    }
  };
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span>{t("diff.changes", { count: chunks })}</span>
        <span className="text-muted-foreground">{t("diff.lines", { added: lines.added, removed: lines.removed })}</span>
        <span className="text-muted-foreground">{legend}</span>
        <span className="ms-auto flex gap-1">
          <Button variant="outline" disabled={chunks === 0} onClick={() => move(goToPreviousChunk)}>
            {t("diff.previous")}
          </Button>
          <Button variant="outline" disabled={chunks === 0} onClick={() => move(goToNextChunk)}>
            {t("diff.next")}
          </Button>
        </span>
      </div>
      <div ref={element} />
    </div>
  );
}

/** ReadOnlyText is a text in a read-only editor of its own, under its name. */
function ReadOnlyText({ text, name }: { text: string; name: string }) {
  const t = useT();
  const element = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    if (element.current === null) {
      return undefined;
    }
    const editor = new EditorView({
      parent: element.current,
      state: EditorState.create({
        doc: text,
        extensions: [
          EditorState.readOnly.of(true),
          EditorView.editable.of(false),
          markdownEditing(),
          editorTheme,
          editorPhrases(t),
          EditorView.contentAttributes.of({ "aria-label": name }),
        ],
      }),
    });
    return () => editor.destroy();
  }, [text, name, t]);
  return (
    <section className="min-w-0 space-y-1">
      <h3 className="text-sm font-medium">{name}</h3>
      <div ref={element} className="max-h-[70vh] overflow-auto rounded-md border" />
    </section>
  );
}

/**
 * The diff's marks besides its colours (M8 design 4.3): + before a line
 * added, − before a line removed and the line struck through; the folded
 * stretches in the app's colours, light or dark, merge's own being those
 * of a light editor.
 */
const diffTheme = EditorView.theme({
  "& .cm-collapsedLines": { color: "var(--muted-foreground)", background: "var(--accent)" },
  "& .cm-changedLine, & .cm-deletedLine": { position: "relative", paddingInlineStart: "1.25em" },
  "& .cm-changedLine::before, & .cm-deletedLine::before": {
    position: "absolute",
    insetInlineStart: "0.25em",
    color: "var(--muted-foreground)",
  },
  "& .cm-changedLine::before": { content: '"+"' },
  "& .cm-deletedLine::before": { content: '"−"' },
  "& .cm-deletedLine del": { textDecoration: "line-through" },
});
