import type { Ref } from "react";

import { Button } from "../../components/ui/button";
import { useT, type Translate } from "../../i18n/i18n";
import { isolate, Isolated, type Isolating } from "../../i18n/isolated";
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
 * the account's access to it, its notebook being upgraded. Its page
 * focuses it as it shows; with changes not saved, it says to copy them.
 * Back to reading leaves the edit. A card as the conflict's, the name of who
 * took the edit in a bdi (M13/P4/S3, decisions 868, 878).
 */
export function EditLostBanner({ lost, unsaved, back, ref }: EditLostBannerProps) {
  const t = useT();
  const me = useStore().account?.me?.id;
  return (
    <div
      ref={ref}
      role="alert"
      tabIndex={-1}
      className="space-y-3 rounded-control border border-destructive/35 bg-surface px-4 py-3 text-sm outline-none"
    >
      <p className="text-destructive">
        <Isolated {...lostText(lost, t, me)} />
      </p>
      {unsaved && <p>{t("editor.lostUnsaved")}</p>}
      <Button variant="outline" size="sm" shape="pill" onClick={back}>
        {t("editor.backToReading")}
      </Button>
    </div>
  );
}

/** plain is a text of no names. */
const plain = (text: string): Isolating => ({ text, names: [] });

/**
 * lostText is why the edit saves no more, who took it a name in it; me is the account's id, which the lock taken
 * meanwhile may be.
 */
function lostText(lost: EditLost, t: Translate, me: string | undefined): Isolating {
  switch (lost.reason) {
    case "taken_over":
      return plain(t("editor.lostTakenOver"));
    case "unlocked":
      return isolate((name) => t("editor.lostUnlocked", { name: name(lost.by) }));
    case "taken":
      return lost.holder.user_id === me
        ? plain(t("editor.lostTakenSelf"))
        : isolate((name) => t("editor.lostTaken", { name: name(lost.holder.display_name) }));
    case "gone":
      return plain(t("editor.lostGone"));
    case "source":
      return plain(t("editor.lostSource"));
    case "upgrading":
      return plain(t("editor.lostUpgrading"));
    case "no_access":
      return plain(t("editor.lostAccess"));
  }
}
