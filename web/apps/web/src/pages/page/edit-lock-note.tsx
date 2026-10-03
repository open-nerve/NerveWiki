import { observer } from "mobx-react-lite";
import { useState } from "react";
import useSWR from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { EditLock, TreeNode } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";

type EditLockNoteProps = {
  notebook: Notebook;
  page: TreeNode;
  /** editHere edits the page here, taking the lock over from the account elsewhere: for those who may edit it. */
  editHere?: () => void;
};

/**
 * EditLockNote tells who is editing the page (M5/P3 design 3.10): another
 * member, or the account itself elsewhere (another tab, a token); nothing
 * while no one does. The lock is read by page, again on each of its events
 * and once its lease should have ended, which no event tells. The
 * notebook's admins can release it, once confirmed; the one whose edit
 * ends learns so as they save. The account editing elsewhere may edit here
 * instead (M5/P4 design 3.6): the edit elsewhere is then taken over.
 */
export const EditLockNote = observer(function EditLockNote({ notebook, page, editHere }: EditLockNoteProps) {
  const t = useT();
  const pages = usePageTree(notebook);
  const me = useStore().account?.me;
  // SWR polls by a number: a function of the data it would only ask once polling had started.
  const [expiry, setExpiry] = useState(0);
  const { data, mutate } = useSWR(["edit-lock", page.id], () => pages.editLock(page.id), {
    refreshInterval: expiry,
    onSuccess: (lock) => setExpiry(untilExpiry(lock)),
  });
  const holder = data?.holder;
  if (holder === undefined || holder === null) {
    return null;
  }
  const self = holder.user_id === me?.id;
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border px-4 py-2 text-sm text-muted-foreground">
      <output>{self ? t("page.lockedBySelf") : t("page.lockedBy", { name: holder.display_name })}</output>
      <div className="flex gap-2">
        {self && editHere !== undefined && (
          <Button variant="outline" onClick={editHere}>
            {t("page.editHere")}
          </Button>
        )}
        {notebook.role === "admin" && (
          <ConfirmDialog
            trigger={<Button variant="outline">{t("page.releaseLock")}</Button>}
            title={t("page.releaseLockTitle")}
            description={
              self ? t("page.releaseLockBodySelf") : t("page.releaseLockBody", { name: holder.display_name })
            }
            confirmLabel={t("page.releaseLock")}
            sendingLabel={t("page.releasing")}
            cancelLabel={t("page.cancel")}
            confirm={async () => {
              await pages.releaseEditLock(page.id);
              await mutate();
            }}
          />
        )}
      </div>
    </div>
  );
});

/** untilExpiry is when to read a held lock again: once its lease should have ended; never (0) while no one holds it. */
function untilExpiry(lock: EditLock): number {
  return lock.expires_in ? lock.expires_in * 1000 : 0;
}
