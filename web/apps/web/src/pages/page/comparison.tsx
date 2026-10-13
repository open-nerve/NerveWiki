import { useCallback, useEffect, useState } from "react";

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
 * one that cannot load says so, with Try again. Its unchanged stretches
 * are folded; Show unchanged lines, which the keyboard reaches as it does
 * not a fold, shows them all.
 */
export function Comparison({ before, after, label, beforeName, afterName, shown }: ComparisonProps) {
  const t = useT();
  const [diff, setDiff] = useState<Diff>();
  const [failed, setFailed] = useState(false);
  const [unfolded, setUnfolded] = useState(false);
  const load = useCallback(() => import("../../editor/text-diff").then(setDiff, () => setFailed(true)), []);
  useEffect(() => void load(), [load]);
  if (diff !== undefined) {
    return (
      <div className="space-y-2">
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
      </div>
    );
  }
  if (failed) {
    return (
      <div className="space-y-2">
        <Alert>{t("version.diffNotLoaded")}</Alert>
        <Button
          variant="outline"
          onClick={() => {
            setFailed(false);
            void load();
          }}
        >
          {t("status.retry")}
        </Button>
      </div>
    );
  }
  return <Loading />;
}
