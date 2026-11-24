import { observer } from "mobx-react-lite";
import { createPortal } from "react-dom";

import { errorText, fieldErrors } from "../../app/problem-messages";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { cn } from "../../lib/cn";
import type { PageEditing } from "../../stores/page-editing";
import { useEditHeadSlots } from "./page-head";

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
 *
 * In the page's head (M13/P4/S1, decisions 866, 894): the status in the right group's first box, where who edited
 * the page last is while it is read, its type and colour the box's (12.5px, faint; a failure in the destructive
 * colour), wrapping in it where it is long; "Saved." with its check before it (styles.css, nw-edit-status); Save
 * before the right column's toggle, Done after it. Nothing of its own takes room in the edit's view.
 */
export const PageEditingBar = observer(function PageEditingBar({ editing, waiting, save, leave }: PageEditingBarProps) {
  const t = useT();
  const { meta, before, after } = useEditHeadSlots();
  // Waiting to leave, the failure shown before is tried again by the leave's own save; one after it shows.
  const failed =
    editing.failure === undefined || editing.failure === waiting?.failure
      ? undefined
      : (fieldErrors(editing.failure, t).content ??
        errorText(editing.failure, t, { bad_request: "editor.tooSlow", forbidden: "editor.lostAccess" }));
  // What the status says, and whether it says the edit is saved: its kind, not its text, draws the check.
  const [status, saved] =
    failed !== undefined
      ? [failed, false]
      : editing.conflict !== undefined
        ? [t("editor.conflicted"), false]
        : waiting !== undefined
          ? [t("editor.waiting"), false]
          : editing.busy
            ? [t("editor.busy"), false]
            : editing.saving
              ? [t("editor.saving"), false]
              : editing.unsaved
                ? [t("editor.unsaved"), false]
                : editing.saved
                  ? [t("page.saved"), true]
                  : ["", false];
  return (
    <>
      {meta !== null &&
        createPortal(
          <output
            data-saved={saved || undefined}
            className={cn("nw-edit-status block [overflow-wrap:anywhere]", failed !== undefined && "text-destructive")}
          >
            {status}
          </output>,
          meta
        )}
      {before !== null &&
        createPortal(
          <Button variant="outline" size="sm" shape="pill" onClick={save}>
            {t("page.save")}
          </Button>,
          before
        )}
      {after !== null &&
        createPortal(
          <Button
            size="sm"
            shape="pill"
            aria-busy={waiting !== undefined || undefined}
            aria-disabled={waiting !== undefined || undefined}
            onClick={leave}
          >
            {t("editor.done")}
          </Button>,
          after
        )}
    </>
  );
});
