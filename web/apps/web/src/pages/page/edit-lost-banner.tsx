import type { Ref } from "react";

import { Button } from "../../components/ui/button";
import { useT, type Translate } from "../../i18n/i18n";
import { useStore } from "../../stores/context";
import type { EditLost } from "../../stores/edit-session";

type EditLostBannerProps = {
  lost: EditLost;
  /** Whether the edit has changes not saved, which go once it is left. */
  unsaved: boolean;
  /** back is Back to reading. */
  back(): void;
  /** The banner, which its page focuses as it shows. */
  ref?: Ref<HTMLDivElement>;
};

/**
 * EditLostBanner says why the edit saves no more (M5 design 4.9; M5/P4
 * design 3.8), above the editor, now read-only: taken over elsewhere,
 * unlocked by an admin, the lock taken while it lapsed, the page gone,
 * the account's access to it. Its page focuses it as it shows; with
 * changes not saved, it says to copy them. Back to reading leaves the
 * edit.
 */
export function EditLostBanner({ lost, unsaved, back, ref }: EditLostBannerProps) {
  const t = useT();
  const me = useStore().account?.me?.id;
  return (
    <div
      ref={ref}
      role="alert"
      tabIndex={-1}
      className="space-y-3 rounded-md border border-destructive/50 px-4 py-3 text-sm outline-none"
    >
      <p className="text-destructive">{lostText(lost, t, me)}</p>
      {unsaved && <p>{t("editor.lostUnsaved")}</p>}
      <Button variant="outline" onClick={back}>
        {t("editor.backToReading")}
      </Button>
    </div>
  );
}

/** lostText is why the edit saves no more; me is the account's id, which the lock taken meanwhile may be. */
function lostText(lost: EditLost, t: Translate, me: string | undefined): string {
  switch (lost.reason) {
    case "taken_over":
      return t("editor.lostTakenOver");
    case "unlocked":
      return t("editor.lostUnlocked", { name: lost.by });
    case "taken":
      return lost.holder.user_id === me
        ? t("editor.lostTakenSelf")
        : t("editor.lostTaken", { name: lost.holder.display_name });
    case "gone":
      return t("editor.lostGone");
    case "no_access":
      return t("editor.lostAccess");
  }
}
