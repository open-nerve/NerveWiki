import { observer } from "mobx-react-lite";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { badgeClass, listRowClass } from "../../app/settings-page";
import { Button } from "../../components/ui/button";
import { formatDate, formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { ApiToken } from "../../services/api-token.service";
import { useApiTokens, useStore } from "../../stores/context";

/**
 * TokenRow is a personal access token of the list: its name, when it was
 * created, when it expires (or expired), when it was last used (to the
 * minute), and the way to revoke it; revoked, it leaves the list, and the
 * focus goes where revoked puts it. When it was created and when it expires
 * are each kept whole: a narrow row breaks their line between them, never
 * inside a date (review B's fix check, N1).
 */
export const TokenRow = observer(function TokenRow({ token, revoked }: { token: ApiToken; revoked: () => void }) {
  const { preferences } = useStore();
  const t = useT();
  const locale = preferences.locale;
  const expired = token.expires_at !== null && Date.parse(token.expires_at) <= Date.now();
  let expiry = t("tokens.neverExpires");
  if (token.expires_at !== null) {
    const date = formatDate(token.expires_at, locale);
    expiry = expired ? t("tokens.expiredOn", { date }) : t("tokens.expires", { date });
  }
  return (
    <li className={listRowClass}>
      <div className="min-w-0 space-y-1">
        <p className="flex flex-wrap items-center gap-2 text-sm font-medium break-words">
          {token.name}
          {expired && <span className={badgeClass}>{t("tokens.expired")}</span>}
        </p>
        <p className="text-sm text-muted-foreground">
          <span className="whitespace-nowrap">
            {t("tokens.created", { date: formatDate(token.created_at, locale) })}
          </span>{" "}
          · <span className="whitespace-nowrap">{expiry}</span>
        </p>
        <p className="text-sm text-muted-foreground">
          {token.last_used_at === null
            ? t("tokens.neverUsed")
            : t("tokens.lastUsed", { date: formatDateTime(token.last_used_at, locale) })}
        </p>
      </div>
      <RevokeTokenDialog token={token} revoked={revoked} />
    </li>
  );
});

/** RevokeTokenDialog confirms revoking token; once revoked it leaves the list, which takes the row with it. */
function RevokeTokenDialog({ token, revoked }: { token: ApiToken; revoked: () => void }) {
  const apiTokens = useApiTokens();
  const t = useT();
  return (
    <ConfirmDialog
      trigger={
        <Button variant="outline" size="sm" shape="pill" aria-label={t("tokens.revokeLabel", { name: token.name })}>
          {t("tokens.revoke")}
        </Button>
      }
      title={t("tokens.revokeTitle", { name: token.name })}
      description={t("tokens.revokeBody")}
      confirmLabel={t("tokens.revoke")}
      sendingLabel={t("tokens.revoking")}
      cancelLabel={t("tokens.cancel")}
      confirm={() => apiTokens.revoke(token.id)}
      focusAfter={revoked}
    />
  );
}
