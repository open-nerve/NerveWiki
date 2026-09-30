import { useState } from "react";

import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "../../components/ui/alert-dialog";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { useAccount, useStore } from "../../stores/context";

/**
 * DeactivateDialog confirms deactivating the account (M1/P6 design 3.5).
 * Once the server has deactivated it, every session is over: this browser
 * forgets its session, and the guards take every tab to the sign-in page.
 * A refusal keeps the dialog open with its reason, and the session.
 */
export function DeactivateDialog() {
  const { account } = useAccount();
  const { auth } = useStore();
  const t = useT();
  const [open, setOpen] = useState(false);
  const [failure, setFailure] = useState<unknown>();
  const [sending, setSending] = useState(false);
  const failed = failure === undefined ? undefined : errorText(failure, t);

  async function deactivate() {
    setSending(true);
    setFailure(undefined);
    try {
      await account.deactivate();
    } catch (error) {
      setFailure(error);
      setSending(false);
      return;
    }
    await auth.endSession();
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (!sending) {
          setOpen(next);
          setFailure(undefined);
        }
      }}
    >
      <AlertDialogTrigger asChild>
        <Button variant="destructive">{t("deactivate.open")}</Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogTitle>{t("deactivate.confirmTitle")}</AlertDialogTitle>
        <AlertDialogDescription>{t("deactivate.confirmBody")}</AlertDialogDescription>
        {failed !== undefined && <Alert>{failed}</Alert>}
        <div className="flex justify-end gap-2">
          <AlertDialogCancel asChild>
            <Button variant="outline" disabled={sending}>
              {t("deactivate.cancel")}
            </Button>
          </AlertDialogCancel>
          <Button variant="destructive" disabled={sending} onClick={() => void deactivate()}>
            {sending ? t("deactivate.sending") : t("deactivate.confirm")}
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}
