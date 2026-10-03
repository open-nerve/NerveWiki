import { observer } from "mobx-react-lite";

import { errorText } from "../../app/problem-messages";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { PageEditing } from "../../stores/page-editing";

type PageEditingBarProps = {
  editing: PageEditing;
  save(): void;
  leave(): void;
};

/**
 * PageEditingBar is the edit's status and buttons (M4/P6 design 3.7): what
 * the last save came to, or that the edit is unsaved, in an output that
 * screen readers announce; Save and Done.
 */
export const PageEditingBar = observer(function PageEditingBar({ editing, save, leave }: PageEditingBarProps) {
  const t = useT();
  const failed =
    editing.failure === undefined
      ? undefined
      : errorText(editing.failure, t, { bad_request: "editor.tooSlow", forbidden: "editor.lostAccess" });
  const status = editing.lostAccess
    ? t("editor.lostAccess")
    : failed !== undefined
      ? failed
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
      <output
        className={
          failed === undefined && !editing.lostAccess ? "text-sm text-muted-foreground" : "text-sm text-destructive"
        }
      >
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
