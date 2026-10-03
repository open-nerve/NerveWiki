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
  /** Whether the edit is unsaved: leaving asks only then. */
  unsaved: boolean;
  /** stay is called as the user stays: the focus goes back to the edit. */
  stay(): void;
};

/**
 * UnsavedGuard asks before an unsaved edit is left (M4/P6 design 3.7):
 * a move to another page of the app waits on a confirmation, Stay or
 * Leave; closing or reloading the tab has the browser ask. A change of
 * the query or the hash alone is no leaving. A save that goes through
 * while it asks lets the move go on: nothing is left unsaved.
 */
export function UnsavedGuard({ unsaved, stay }: UnsavedGuardProps) {
  const t = useT();
  const stayed = useRef(false);
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) => unsaved && currentLocation.pathname !== nextLocation.pathname
  );
  useEffect(() => {
    if (blocker.state === "blocked" && !unsaved) {
      blocker.proceed();
    }
  }, [blocker, unsaved]);
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
        <AlertDialogTitle>{t("editor.leaveTitle")}</AlertDialogTitle>
        <AlertDialogDescription>{t("editor.leaveDescription")}</AlertDialogDescription>
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
