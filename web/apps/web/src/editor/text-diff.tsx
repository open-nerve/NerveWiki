import {
  Change,
  diff,
  getChunks,
  getOriginalDoc,
  goToNextChunk,
  goToPreviousChunk,
  unifiedMergeView,
} from "@codemirror/merge";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";

import { useT } from "../i18n/i18n";
import { compareInWorker, compareNow } from "../lib/line-diff-client";
import { lineDiffLimits, type LineDiffResult } from "../lib/line-diff";
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
  /** shown is told the text before whose comparison shows, as it changes: undefined while none of it does. */
  shown?: (before: string | undefined) => void;
};

/** The comparison of the texts a view shows, the attempt at it that answered: its result, or that it failed. */
type Compared = { a: string; b: string; attempt: number; result: LineDiffResult | { kind: "failed" } };

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
 * known of how far; so do changes the view would join into a block of
 * more lines removed than the comparison's bound. Texts the same once
 * split say so, or that only their line breaks or byte order mark differ.
 * A comparison that fails says so, with Try again; failed again, that a
 * reload of the page usually helps, with Reload. While the comparison of
 * newer texts runs, the one before stays shown, marked busy, and saying it
 * compares; shown hears of the texts' when theirs shows.
 */
export function TextDiff({
  before,
  after,
  label,
  beforeName,
  afterName,
  unfolded = false,
  compareLarge = compareInWorker,
  shown,
}: TextDiffProps) {
  const t = useT();
  const split = useMemo(() => {
    const a = splitBreaks(before);
    const b = splitBreaks(after);
    return { a: a.text, b: b.text, breaksOnly: a.text === b.text && before !== after };
  }, [before, after]);
  const [attempt, setAttempt] = useState(0);
  const now = useMemo(() => {
    const result = compareNow(split.a, split.b);
    return result === undefined ? undefined : { a: split.a, b: split.b, attempt: 0, result };
  }, [split]);
  const [later, setLater] = useState<Compared>();
  if (later !== undefined && (now !== undefined || split.a === split.b)) {
    // No worker's comparison is shown: the last one, and its texts, are let go.
    setLater(undefined);
  }
  useEffect(() => {
    if (now !== undefined || split.a === split.b) {
      return undefined;
    }
    const controller = new AbortController();
    const answer = (result: Compared["result"]) => {
      if (!controller.signal.aborted) {
        setLater({ a: split.a, b: split.b, attempt, result });
      }
    };
    compareLarge(split.a, split.b, controller.signal).then(answer, () => answer({ kind: "failed" }));
    return () => controller.abort();
  }, [now, split, compareLarge, attempt]);
  // The comparison of other texts stays shown while this one runs (M8/P1 review A, M4), unless it failed.
  const stale = later !== undefined && (later.a !== split.a || later.b !== split.b || later.attempt !== attempt);
  const compared = now ?? (stale && later.result.kind === "failed" ? undefined : later);
  const state = useMemo(() => {
    if (compared?.result.kind !== "changes") {
      return undefined;
    }
    const found = compared.result.changes.map((c) => new Change(c.fromA, c.toA, c.fromB, c.toB));
    // The view compares the texts the comparison did: any other pair, which it never asks for, is compared afresh.
    const override = (a: string, b: string) =>
      a === compared.a && b === compared.b ? found : diff(a, b, { scanLimit: 500 });
    return EditorState.create({
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
    });
  }, [compared, label, t, unfolded]);
  const busy = now === undefined && stale;
  // A comparison of these texts, or none needed: what a restore sends goes with what shows (M8/P1 fix check 1, N1).
  const shownBefore =
    split.a === split.b || (compared !== undefined && compared.result.kind !== "failed" && !busy) ? before : undefined;
  useEffect(() => shown?.(shownBefore), [shown, shownBefore]);

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
  const { result } = compared;
  if (result.kind === "failed") {
    // Failed again on Try again: a worker of an older build than the server's, which a reload replaces.
    const again = compared.attempt > 0;
    return (
      <div className="space-y-2">
        <p role="alert" className="rounded-md border border-destructive/50 px-4 py-3 text-sm text-destructive">
          {t(again ? "diff.failedAgain" : "diff.failed")}
        </p>
        <div className="flex gap-2">
          <button type="button" className={buttonClass} onClick={() => setAttempt(attempt + 1)}>
            {t("status.retry")}
          </button>
          {again && (
            <button type="button" className={buttonClass} onClick={() => window.location.reload()}>
              {t("error.reload")}
            </button>
          )}
        </div>
      </div>
    );
  }
  if (result.kind === "tooLarge" || state === undefined || removedLines(state) > lineDiffLimits.maxChangedLines) {
    return (
      <div className="space-y-2" aria-busy={busy || undefined}>
        {busy && <p className="text-sm text-muted-foreground">{t("diff.comparing")}</p>}
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
      state={state}
      legend={t("diff.legend", { before: beforeName, after: afterName })}
      lines={{ added: result.added, removed: result.removed }}
      busy={busy}
    />
  );
}

/**
 * removedLines is the most lines a change of the view removes: it shows
 * them at once, in one block, having joined the changes that only an
 * empty line parts, so that it may hold more lines than the comparison
 * counted (M8/P1 review A, M5).
 */
function removedLines(state: EditorState): number {
  const original = getOriginalDoc(state);
  let most = 0;
  for (const chunk of getChunks(state)?.chunks ?? []) {
    if (chunk.toA > chunk.fromA) {
      most = Math.max(most, original.lineAt(chunk.endA).number - original.lineAt(chunk.fromA).number + 1);
    }
  }
  return most;
}

type UnifiedDiffProps = {
  /** The text after with what changed from the text before, as the view's state. */
  state: EditorState;
  legend: string;
  lines: { added: number; removed: number };
  /** Whether a newer comparison runs: the view says so, dimmed. */
  busy: boolean;
};

/**
 * UnifiedDiff is the text after, read-only, with what changed from the
 * text before: @codemirror/merge's unified view, given the changes the
 * comparison found (diffConfig.override) rather than computing its own.
 */
function UnifiedDiff({ state, legend, lines, busy }: UnifiedDiffProps) {
  const t = useT();
  const element = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView>(undefined);
  const chunks = getChunks(state)?.chunks.length ?? 0;
  useLayoutEffect(() => {
    if (element.current === null) {
      return undefined;
    }
    const editor = new EditorView({ parent: element.current, state });
    view.current = editor;
    return () => {
      view.current = undefined;
      editor.destroy();
    };
  }, [state]);
  const move = (command: typeof goToNextChunk) => {
    const editor = view.current;
    if (editor !== undefined) {
      command({ state: editor.state, dispatch: (tr) => editor.dispatch(tr) });
    }
  };
  return (
    <div className="space-y-2" aria-busy={busy || undefined}>
      <div className="flex flex-wrap items-center gap-2 text-sm">
        {busy && <span className="text-muted-foreground">{t("diff.comparing")}</span>}
        <span>{t("diff.changes", { count: chunks })}</span>
        <span className="text-muted-foreground">{t("diff.lines", { added: lines.added, removed: lines.removed })}</span>
        <span className="text-muted-foreground">{legend}</span>
        <span className="ms-auto flex gap-1">
          <button type="button" className={buttonClass} disabled={chunks === 0} onClick={() => move(goToPreviousChunk)}>
            {t("diff.previous")}
          </button>
          <button type="button" className={buttonClass} disabled={chunks === 0} onClick={() => move(goToNextChunk)}>
            {t("diff.next")}
          </button>
        </span>
      </div>
      <div ref={element} className={busy ? "opacity-60" : undefined} />
    </div>
  );
}

/**
 * The look of the diff's buttons: the app's outline button's (components/ui/button.tsx), which the editor does not
 * import (v0.1 design 13.2, item 6).
 */
const buttonClass =
  "inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-md border bg-background px-4 py-2 text-sm font-medium whitespace-nowrap transition-colors outline-none hover:bg-accent hover:text-accent-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50";

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
