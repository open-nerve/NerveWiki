import { ConfirmDialog } from "../../app/confirm-dialog";
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
  return (
    <ConfirmDialog
      trigger={<Button variant="destructive">{t("deactivate.open")}</Button>}
      title={t("deactivate.confirmTitle")}
      description={t("deactivate.confirmBody")}
      confirmLabel={t("deactivate.confirm")}
      sendingLabel={t("deactivate.sending")}
      cancelLabel={t("deactivate.cancel")}
      confirm={async () => {
        await account.deactivate();
        await auth.endSession();
      }}
    />
  );
}
