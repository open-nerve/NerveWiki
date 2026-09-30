import { observer } from "mobx-react-lite";
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
import { formatDate, formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { ApiToken } from "../../services/api-token.service";
import { useApiTokens, useStore } from "../../stores/context";

/**
 * TokenRow is a personal access token of the list: its name, when it was
 * created, when it expires (or expired), when it was last used (to the
 * minute), and the way to revoke it.
 */
export const TokenRow = observer(function TokenRow({ token }: { token: ApiToken }) {
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
    <li className="flex items-start justify-between gap-4 p-4">
      <div className="min-w-0 space-y-1">
        <p className="flex flex-wrap items-center gap-2 font-medium break-words">
          {token.name}
          {expired && (
            <span className="rounded border px-1.5 text-xs font-normal text-muted-foreground">
              {t("tokens.expired")}
            </span>
          )}
        </p>
        <p className="text-sm text-muted-foreground">
          {t("tokens.created", { date: formatDate(token.created_at, locale) })} · {expiry}
        </p>
        <p className="text-sm text-muted-foreground">
          {token.last_used_at === null
            ? t("tokens.neverUsed")
            : t("tokens.lastUsed", { date: formatDateTime(token.last_used_at, locale) })}
        </p>
      </div>
      <RevokeTokenDialog token={token} />
    </li>
  );
});

/**
 * RevokeTokenDialog confirms revoking token; once revoked it leaves the
 * list, which takes the row and the dialog with it. A refusal stays in the
 * dialog.
 */
function RevokeTokenDialog({ token }: { token: ApiToken }) {
  const apiTokens = useApiTokens();
  const t = useT();
  const [open, setOpen] = useState(false);
  const [failure, setFailure] = useState<unknown>();
  const [sending, setSending] = useState(false);
  const failed = failure === undefined ? undefined : errorText(failure, t);

  async function revoke() {
    setSending(true);
    setFailure(undefined);
    try {
      await apiTokens.revoke(token.id);
    } catch (error) {
      setFailure(error);
      setSending(false);
    }
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
        <Button variant="outline" aria-label={t("tokens.revokeLabel", { name: token.name })}>
          {t("tokens.revoke")}
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogTitle>{t("tokens.revokeTitle", { name: token.name })}</AlertDialogTitle>
        <AlertDialogDescription>{t("tokens.revokeBody")}</AlertDialogDescription>
        {failed !== undefined && <Alert>{failed}</Alert>}
        <div className="flex justify-end gap-2">
          <AlertDialogCancel asChild>
            <Button variant="outline" disabled={sending}>
              {t("tokens.cancel")}
            </Button>
          </AlertDialogCancel>
          <Button variant="destructive" disabled={sending} onClick={() => void revoke()}>
            {sending ? t("tokens.revoking") : t("tokens.revoke")}
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}
