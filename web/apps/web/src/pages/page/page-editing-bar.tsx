import { observer } from "mobx-react-lite";

import { errorText, fieldErrors } from "../../app/problem-messages";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { PageEditing } from "../../stores/page-editing";

type PageEditingBarProps = {
  editing: PageEditing;
  /** Whether the edit is left once the editor's uploads are in (M7/P4 design 5.2). */
  waiting?: boolean;
  save(): void;
  leave(): void;
};

/**
 * PageEditingBar is the edit's status and buttons (M4/P6 design 3.7): what
 * the last save came to, a conflict, that the edit is left once the
 * editor's uploads are in, or that it is unsaved, in an output that
 * screen readers announce (a conflict autosave runs into
 * moves no focus: M5/P5 design 3.6); Save and Done. A content refused says which of
 * its rules it breaks.
 */
export const PageEditingBar = observer(function PageEditingBar({
  editing,
  waiting = false,
  save,
  leave,
}: PageEditingBarProps) {
  const t = useT();
  const failed =
    editing.failure === undefined
      ? undefined
      : (fieldErrors(editing.failure, t).content ??
        errorText(editing.failure, t, { bad_request: "editor.tooSlow", forbidden: "editor.lostAccess" }));
  const status =
    failed !== undefined
      ? failed
      : editing.conflict !== undefined
        ? t("editor.conflicted")
        : editing.busy
          ? t("editor.busy")
          : editing.saving
            ? t("editor.saving")
            : waiting
              ? t("editor.waiting")
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
        <Button onClick={leave}>{t("editor.done")}</Button>
      </div>
    </div>
  );
});
