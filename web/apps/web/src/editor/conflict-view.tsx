import { useT } from "../i18n/i18n";
import { TextDiff } from "./text-diff";

type ConflictDiffProps = {
  /** The page's content as it is now, as written. */
  theirs: string;
  /** The user's text, as written. */
  mine: string;
  /** Whether the unchanged stretches are shown: folded otherwise. */
  unfolded: boolean;
};

/**
 * ConflictDiff shows what the user's text changes of the page as it is
 * now (M4/P6 design 3.8): the user's text, read-only, with what it adds
 * and what it would remove, in the versions' diff (M8 design 4.3, 13.2
 * item 25), which compares them as the editor holds them: a difference of
 * line breaks alone does not show.
 */
export function ConflictDiff({ theirs, mine, unfolded }: ConflictDiffProps) {
  const t = useT();
  return (
    <TextDiff
      before={theirs}
      after={mine}
      label={t("editor.conflictDiff")}
      beforeName={t("editor.conflictTheirs")}
      afterName={t("editor.conflictMine")}
      unfolded={unfolded}
    />
  );
}
