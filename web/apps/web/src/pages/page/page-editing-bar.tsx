import { observer } from "mobx-react-lite";

import { errorText, fieldErrors } from "../../app/problem-messages";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { PageEditing } from "../../stores/page-editing";

type PageEditingBarProps = {
  editing: PageEditing;
  /**
   * Whether the edit is left once the editor's uploads are in (M7/P4 design
   * 5.2): with the failure shown as it began to wait, which the leave's own
   * save tries again.
   */
  waiting?: { failure: unknown };
  save(): void;
  leave(): void;
};

/**
 * PageEditingBar is the edit's status and buttons (M4/P6 design 3.7): what
 * the last save came to, a conflict, that the edit is left once the
 * editor's uploads are in (Done busy meanwhile, which saves aside), or
 * that it is unsaved, in an output that screen readers announce (a conflict autosave runs into
 * moves no focus: M5/P5 design 3.6); Save and Done. A content refused says which of
 * its rules it breaks.
 */
export const PageEditingBar = observer(function PageEditingBar({ editing, waiting, save, leave }: PageEditingBarProps) {
  const t = useT();
  // Waiting to leave, the failure shown before is tried again by the leave's own save; one after it shows.
  const failed =
    editing.failure === undefined || editing.failure === waiting?.failure
      ? undefined
      : (fieldErrors(editing.failure, t).content ??
        errorText(editing.failure, t, { bad_request: "editor.tooSlow", forbidden: "editor.lostAccess" }));
  const status =
    failed !== undefined
      ? failed
      : editing.conflict !== undefined
        ? t("editor.conflicted")
        : waiting !== undefined
          ? t("editor.waiting")
          : editing.busy
            ? t("editor.busy")
            : editing.saving
              ? t("editor.saving")
              : editing.unsaved
                ? t("editor.unsaved")
                : editing.saved
                  ? t("page.saved")
                  : "";
  return (
    <div className="flex flex-wrap items-center justify-between gap-2">
      <output className={failed === undefined ? "text-sm text-muted-foreground" : "text-sm text-destructive"}>
        {status}
      </output>
      <div className="flex gap-2">
        <Button variant="outline" onClick={save}>
          {t("page.save")}
        </Button>
        <Button
          aria-busy={waiting !== undefined || undefined}
          aria-disabled={waiting !== undefined || undefined}
          onClick={leave}
        >
          {t("editor.done")}
        </Button>
      </div>
    </div>
  );
});
