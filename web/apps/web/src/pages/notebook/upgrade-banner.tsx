import { Link, useMatch } from "react-router";

import { useT, type PlainKey } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import { useWorkspace } from "../workspace/workspace-layout";

type UpgradeState = NonNullable<Notebook["upgrade"]>["state"];

/** What the banner, and the notebook's settings, say of each state of the upgrade (M11/P1 design 4.4). */
export const upgradeStateText: Record<UpgradeState, PlainKey> = {
  upgrading: "upgrade.bannerUpgrading",
  reviewing: "upgrade.bannerReviewing",
  rolling_back: "upgrade.bannerRollingBack",
};

/**
 * UpgradeBanner says, above each page of the notebook, that its upgrade
 * under way keeps it read only, by the upgrade's state (M11/P1 design
 * 4.4): to whoever reads the notebook, from the notebook as the list
 * holds it, which the mode event and each connection read again. Its
 * admins also get the upgrade's page, but on it, and, while the upgrade
 * may be abandoned, that any of them can (decision 246): not while it
 * rolls back. A status (output), not an alert: it is not read out again on
 * each page, and it takes no focus (v0.1 design 13.2, item 26). No MCP
 * address nor prompt (decision 184).
 */
export function UpgradeBanner({ notebook }: { notebook: Notebook }) {
  const t = useT();
  const { slug } = useWorkspace();
  // On the upgrade's page, which the link would lead to, there is none (M11 design review C-N6).
  const onItsPage = useMatch("/:slug/notebooks/:id/upgrade") !== null;
  const upgrade = notebook.upgrade;
  if (upgrade === null) {
    return null;
  }
  const admin = notebook.role === "admin";
  return (
    <output className="mb-6 block space-y-1 rounded-md border border-primary/40 bg-muted/50 px-4 py-3 text-sm">
      <span className="block">{t(upgradeStateText[upgrade.state])}</span>
      {admin && (upgrade.state !== "rolling_back" || !onItsPage) && (
        <span className="block">
          {upgrade.state !== "rolling_back" && <>{t("upgrade.bannerAnyAdmin")} </>}
          {!onItsPage && (
            <Link to={`/${slug}/notebooks/${notebook.id}/upgrade`} className="font-medium underline underline-offset-4">
              {t("upgrade.view")}
            </Link>
          )}
        </span>
      )}
    </output>
  );
}
