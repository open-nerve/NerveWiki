import { useEffect, useRef } from "react";
import { useBlocker } from "react-router";

import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
} from "../../components/ui/alert-dialog";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";

type UnsavedGuardProps = {
  /** Whether the edit is unsaved: leaving asks then. */
  unsaved: boolean;
  /** Whether files pasted or dropped into the edit are still uploading: leaving the page in the app asks then too. */
  uploading?: boolean;
  /** stay is called as the user stays: the focus goes back to the edit. */
  stay(): void;
};

/**
 * UnsavedGuard asks before an unsaved edit is left (M4/P6 design 3.7):
 * a move to another page of the app waits on a confirmation, Stay or
 * Leave; closing or reloading the tab has the browser ask. A change of
 * the query or the hash alone is no leaving. A save that goes through
 * while it asks lets the move go on: nothing is left unsaved. While files
 * pasted or dropped still upload, a move to another page asks as well,
 * saying they will not be inserted (M12/P2/S5, 18.1); closing the tab is
 * the AssetStore's to ask about, and signing out asks about neither.
 */
export function UnsavedGuard({ unsaved, uploading = false, stay }: UnsavedGuardProps) {
  const t = useT();
  const stayed = useRef(false);
  const asking = unsaved || uploading;
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) => asking && currentLocation.pathname !== nextLocation.pathname
  );
  useEffect(() => {
    if (blocker.state === "blocked" && !asking) {
      blocker.proceed();
    }
  }, [blocker, asking]);
  useEffect(() => {
    if (!unsaved) {
      return undefined;
    }
    const onBeforeUnload = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [unsaved]);
  return (
    <AlertDialog
      open={blocker.state === "blocked"}
      onOpenChange={(open) => {
        if (!open) {
          stayed.current = true;
          blocker.reset?.();
        }
      }}
    >
      <AlertDialogContent
        onCloseAutoFocus={(event) => {
          if (stayed.current) {
            stayed.current = false;
            event.preventDefault();
            stay();
          }
        }}
      >
        <AlertDialogTitle>{t(unsaved ? "editor.leaveTitle" : "editor.leaveUploadsTitle")}</AlertDialogTitle>
        <AlertDialogDescription>
          {unsaved && uploading
            ? t("page.sentences", {
                first: t("editor.leaveDescription"),
                second: t("editor.leaveUploadsDescription"),
              })
            : t(unsaved ? "editor.leaveDescription" : "editor.leaveUploadsDescription")}
        </AlertDialogDescription>
        <div className="flex justify-end gap-2">
          <AlertDialogCancel asChild>
            <Button variant="outline">{t("editor.stay")}</Button>
          </AlertDialogCancel>
          <Button variant="destructive" onClick={() => blocker.proceed?.()}>
            {t("editor.leave")}
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}
