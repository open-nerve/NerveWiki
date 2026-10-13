import { useCallback, useEffect, useRef, useState } from "react";

import { useFocusLeaving } from "../../lib/focus-leaving";
import { Loading } from "../../components/loading";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";

type Diff = typeof import("../../editor/text-diff");

type ComparisonProps = {
  /** The text before and the text after, as written. */
  before: string;
  after: string;
  /** The comparison's accessible name. */
  label: string;
  /** What the two texts are, for the legend. */
  beforeName: string;
  afterName: string;
  /** shown is told the text before whose comparison shows (TextDiff's shown). */
  shown?: (before: string | undefined) => void;
};

/**
 * Comparison shows what changes from one text to another (M8 design 4.3):
 * TextDiff (editor/text-diff.tsx), which comes in a chunk of its own with
 * the editor's CodeMirror, as the conflict's diff does (conflict-panel.tsx);
 * one that cannot load says so, and that a reload usually helps (a tab of
 * an older build than the server's), with Try again and Reload; loading
 * again, it says so, Try again aria-disabled, and gone with the focus, the
 * chunk loaded, gives it to the comparison. Its unchanged stretches are
 * folded; Show unchanged lines, which the keyboard reaches as it does not
 * a fold, shows them all.
 */
export function Comparison({ before, after, label, beforeName, afterName, shown }: ComparisonProps) {
  const t = useT();
  const [diff, setDiff] = useState<Diff>();
  // Whether the chunk failed to load, and whether Try again loads it.
  const [failed, setFailed] = useState<"failed" | "loading">();
  const [unfolded, setUnfolded] = useState(false);
  // Where the focus in a failure that goes, the chunk loaded, comes (v0.1 design 13.2, item 17): a group named as the
  // comparison is, not by all its text (M8/P1 fix check 4, R4-2).
  const root = useRef<HTMLDivElement>(null);
  const load = useCallback(() => import("../../editor/text-diff").then(setDiff, () => setFailed("failed")), []);
  useEffect(() => void load(), [load]);
  let shows;
  if (diff !== undefined) {
    shows = (
      <>
        <diff.TextDiff
          before={before}
          after={after}
          label={label}
          beforeName={beforeName}
          afterName={afterName}
          unfolded={unfolded}
          shown={shown}
        />
        <Button variant="outline" aria-pressed={unfolded} onClick={() => setUnfolded(!unfolded)}>
          {t("editor.showUnchanged")}
        </Button>
      </>
    );
  } else if (failed === undefined) {
    shows = <Loading />;
  } else {
    shows = (
      <ChunkNotLoaded
        loading={failed === "loading"}
        retry={() => {
          setFailed("loading");
          void load();
        }}
        left={() => root.current?.focus({ preventScroll: true })}
      />
    );
  }
  return (
    // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- a fieldset groups a form's fields: this groups no field
    <div ref={root} tabIndex={-1} role="group" aria-label={label} className="space-y-2 outline-none">
      {shows}
    </div>
  );
}

/**
 * ChunkNotLoaded says the diff's chunk could not load, and that a reload
 * usually helps, with Try again and Reload. Loading again, it says so, Try
 * again aria-disabled, doing nothing: the focus stays on it (v0.1 design
 * 13.2, item 12). Gone with the focus, it calls left before it leaves the
 * document (item 17).
 */
function ChunkNotLoaded({ loading, retry, left }: { loading: boolean; retry: () => void; left: () => void }) {
  const t = useT();
  const own = useRef<HTMLDivElement>(null);
  useFocusLeaving(own, left);
  return (
    <div ref={own} className="space-y-2" aria-busy={loading || undefined}>
      <Alert>{t(loading ? "status.loading" : "version.diffNotLoaded")}</Alert>
      <div className="flex gap-2">
        <Button
          variant="outline"
          className="aria-disabled:opacity-50"
          aria-disabled={loading || undefined}
          onClick={loading ? undefined : retry}
        >
          {t("status.retry")}
        </Button>
        <Button variant="outline" onClick={() => window.location.reload()}>
          {t("error.reload")}
        </Button>
      </div>
    </div>
  );
}
