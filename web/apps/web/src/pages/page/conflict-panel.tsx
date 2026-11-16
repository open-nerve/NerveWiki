import { useCallback, useEffect, useId, useState, type RefObject } from "react";

import { Loading } from "../../components/loading";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Conflict } from "../../stores/page-editing";
import { ChunkNotLoaded } from "./comparison";

type Diff = typeof import("../../editor/conflict-view");

type ConflictPanelProps = {
  conflict: Conflict;
  /** The panel's heading, which takes the focus as the conflict comes and when a save is asked for meanwhile. */
  heading: RefObject<HTMLHeadingElement | null>;
  keep(): void;
  discard(): void;
};

/**
 * ConflictPanel is a save refused because the page changed meanwhile
 * (M4/P6 design 3.8): what the user's text changes of the page as it is
 * now, then Keep mine, which saves it over the page, and Discard mine,
 * which edits the page as it is now. The diff's unchanged stretches are
 * folded; Show unchanged lines, which the keyboard reaches as it does not
 * a fold, shows them all. The diff comes in a chunk of its own; one that
 * cannot load says so, with Try again, and the two buttons still work:
 * the user's text is in the editor whatever happens here. Try again, as
 * the comparison's, stays while the chunk loads, aria-disabled, the focus
 * on it; loaded, the diff shown, the panel's heading takes the focus that
 * left with it (M12/P2/S1, 21.14).
 */
export function ConflictPanel({ conflict, heading, keep, discard }: ConflictPanelProps) {
  const t = useT();
  const title = useId();
  const [diff, setDiff] = useState<Diff>();
  // Whether the chunk failed to load, and whether Try again loads it.
  const [failed, setFailed] = useState<"failed" | "loading">();
  const [unfolded, setUnfolded] = useState(false);
  const load = useCallback(() => import("../../editor/conflict-view").then(setDiff, () => setFailed("failed")), []);
  useEffect(() => void load(), [load]);
  return (
    <section aria-labelledby={title} className="space-y-3 rounded-md border border-destructive/50 p-3">
      <h2 id={title} ref={heading} tabIndex={-1} className="font-medium outline-none">
        {t("editor.conflictTitle")}
      </h2>
      <p className="text-sm text-muted-foreground">{t("editor.conflictDescription")}</p>
      {diff !== undefined ? (
        <div className="space-y-2">
          <diff.ConflictDiff theirs={conflict.theirs} mine={conflict.mine} unfolded={unfolded} />
          <Button variant="outline" aria-pressed={unfolded} onClick={() => setUnfolded(!unfolded)}>
            {t("editor.showUnchanged")}
          </Button>
        </div>
      ) : failed !== undefined ? (
        <ChunkNotLoaded
          loading={failed === "loading"}
          retry={() => {
            setFailed("loading");
            void load();
          }}
          left={() => heading.current?.focus()}
          text={t("editor.conflictDiffNotLoaded")}
          reload={false}
        />
      ) : (
        <Loading />
      )}
      <div className="flex flex-wrap gap-2">
        <Button onClick={keep}>{t("editor.keepMine")}</Button>
        <Button variant="outline" onClick={discard}>
          {t("editor.discardMine")}
        </Button>
      </div>
    </section>
  );
}
