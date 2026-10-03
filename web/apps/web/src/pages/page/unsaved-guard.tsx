import { useEffect } from "react";
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

/**
 * UnsavedGuard asks before an unsaved edit is left (M4/P6 design 3.7):
 * a move to another page of the app waits on a confirmation, Stay or
 * Leave; closing or reloading the tab has the browser ask. A change of
 * the query or the hash alone is no leaving. It is there only while the
 * edit is unsaved.
 */
export function UnsavedGuard() {
  const t = useT();
  const blocker = useBlocker(({ currentLocation, nextLocation }) => currentLocation.pathname !== nextLocation.pathname);
  useEffect(() => {
    const onBeforeUnload = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, []);
  return (
    <AlertDialog
      open={blocker.state === "blocked"}
      onOpenChange={(open) => {
        if (!open) {
          blocker.reset?.();
        }
      }}
    >
      <AlertDialogContent>
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
