import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
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
 *
 * Save shows only when it is needed (M13/P4/S3, decisions 877, 885, 895): with something unsaved (a failure shown is
 * of something unsaved: Save tries again), and, once it shows, as long as the focus stays in the head's right group,
 * so that the keys on their way to it find it there however the autosave falls (held); pressed with nothing to save,
 * it does nothing. The focus coming into the group shows no Save that was hidden. Hidden, it keeps its room (decision
 * 894: the head does not move as it comes and goes, which the mockup's has not, decision 896).
 */
export const PageEditingBar = observer(function PageEditingBar({ editing, waiting, save, leave }: PageEditingBarProps) {
  const t = useT();
  const { group, meta, before, after } = useEditHeadSlots();
  // Whether a Save showing is kept (decision 895): from the focus in the head's right group with something to save
  // until it leaves the group, so that the keys on their way to Save find it there however the autosave falls (review
  // A's R1-M2); the focus coming into the group shows no Save that was hidden (review A's R2-M1).
  const [held, setHeld] = useState(false);
  useEffect(() => {
    if (group === null) {
      return undefined;
    }
    const into = () => {
      if (editing.unsaved) {
        setHeld(true);
      }
    };
    const out = (event: FocusEvent) => {
      const to = event.relatedTarget;
      // A window losing the focus keeps the active element where it was: it is not left (design section 1 (g)).
      if (!(to instanceof Node && group.contains(to)) && !group.contains(document.activeElement)) {
        setHeld(false);
      }
    };
    group.addEventListener("focusin", into);
    group.addEventListener("focusout", out);
    return () => {
      group.removeEventListener("focusin", into);
      group.removeEventListener("focusout", out);
    };
  }, [group, editing]);
  // Something to save coming as the focus is in the group already (a link an upload puts in as Done waits): kept too.
  const { unsaved } = editing;
  useEffect(() => {
    if (unsaved && group?.contains(document.activeElement) === true) {
      setHeld(true);
    }
  }, [group, unsaved]);
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
      {before !== null && createPortal(<SaveButton save={save} shown={unsaved || held} />, before)}
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

/**
 * SaveButton is the edit's Save, always there: not shown, it is invisible, keeping its room, not to be focused and not
 * in the accessibility tree (decisions 877, 894), so that the head's row stays as it is.
 */
function SaveButton({ save, shown }: { save: () => void; shown: boolean }) {
  const t = useT();
  return (
    <Button variant="outline" size="sm" shape="pill" className={cn(!shown && "invisible")} onClick={save}>
      {t("page.save")}
    </Button>
  );
}
