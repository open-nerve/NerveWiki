import { useRef, useState, type ReactNode } from "react";

import { useT } from "../i18n/i18n";
import { useStore } from "../stores/context";
import { ConfirmDialog } from "./confirm-dialog";
import { useMounted } from "./mounted";

/**
 * useSignOut is a "Sign out" that asks first while an edit of the login
 * has changes not saved (M12/P2/S1; v0.1 design 13.2, item 21): this
 * tab's, or another tab's that says so within 100 milliseconds
 * (RootStore.unsavedEdits). Asked, Stay keeps the session, stayed giving
 * the focus back; Sign out signs out, which saves what it can first and
 * loses what it cannot. Otherwise it signs out at once. Pressed again
 * while the other tabs are asked, it does nothing. The caller renders
 * dialog.
 */
export function useSignOut(stayed: () => void): { signOut: () => void; dialog: ReactNode } {
  const store = useStore();
  const t = useT();
  const mounted = useMounted();
  const asking = useRef(false);
  const [open, setOpen] = useState(false);
  const [unsaved, setUnsaved] = useState<"here" | "elsewhere">("here");

  async function ask(): Promise<void> {
    if (asking.current) {
      return;
    }
    asking.current = true;
    try {
      const where = await store.unsavedEdits();
      if (!mounted()) {
        return;
      }
      if (where === undefined) {
        await store.auth.signOut();
      } else {
        setUnsaved(where);
        setOpen(true);
      }
    } finally {
      asking.current = false;
    }
  }

  const dialog = (
    <ConfirmDialog
      held={{ open, onOpenChange: setOpen, onClosed: (done) => done || stayed() }}
      title={t("signOut.unsavedTitle")}
      description={t(unsaved === "here" ? "signOut.unsavedHere" : "signOut.unsavedElsewhere")}
      confirmLabel={t("signOut.confirm")}
      sendingLabel={t("signOut.signingOut")}
      cancelLabel={t("editor.stay")}
      confirm={() => store.auth.signOut()}
    />
  );
  return { signOut: () => void ask(), dialog };
}
