import { ScrollText } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useState } from "react";
import useSWR from "swr";

import { useFollowRole } from "../../app/follow-role";
import { memberWho } from "../../app/member-summary";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { SettingsCard, SettingsSection } from "../../app/settings-page";
import { EmptyState } from "../../components/empty-state";
import { Button } from "../../components/ui/button";
import { formatDateTime } from "../../i18n/format";
import { useT, type Translate } from "../../i18n/i18n";
import type { NotebookAuditEvent } from "../../services/ownerless.service";
import type { Workspace } from "../../services/workspace.service";
import { useAudit, useStore } from "../../stores/context";

/**
 * sentence says what an audit event records, each account by its name and
 * address: two accounts may have the same name.
 */
function sentence(event: NotebookAuditEvent, t: Translate): string {
  const notebook = event.notebook_name;
  const former = memberWho(event.former_owner, t);
  switch (event.action) {
    case "taken_over":
      return t("audit.taken_over", { actor: memberWho(event.actor, t), notebook, former });
    case "deleted":
      return t("audit.deleted", { actor: memberWho(event.actor, t), notebook, former });
    case "returned":
      return t("audit.returned", { notebook, former });
  }
}

/**
 * AuditSection is what was done with a workspace's ownerless notebooks,
 * the newest first (M3/P5 design 3.3): the first page, then Load more for
 * each page after it. A page that cannot be loaded says why beside the
 * button, which tries again; one refused as forbidden follows the role,
 * as the first page does (v0.1 design 13.2, item 18). A group of the
 * ownerless notebooks' page, its events a card's list (M13/P3/S2).
 */
export const AuditSection = observer(function AuditSection({ workspace }: { workspace: Workspace }) {
  const audit = useAudit(workspace);
  const { preferences } = useStore();
  const t = useT();
  const followRole = useFollowRole();
  const { error, mutate } = useSWR(["notebook-audit", workspace.id], () => audit.load(), { onError: followRole });
  const [loading, setLoading] = useState(false);
  const [failure, setFailure] = useState<unknown>();
  const failed = failure === undefined ? undefined : errorText(failure, t);

  async function more() {
    setFailure(undefined);
    setLoading(true);
    try {
      await audit.more();
    } catch (refusal) {
      setFailure(refusal);
      followRole(refusal);
    } finally {
      setLoading(false);
    }
  }

  return (
    <SettingsSection title={t("audit.title")}>
      <p className="max-w-2xl px-0.5 text-sm text-muted-foreground">{t("audit.body")}</p>
      {audit.events === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : audit.events.length === 0 ? (
        <SettingsCard>
          <EmptyState icon={ScrollText} size="card">
            {t("audit.none")}
          </EmptyState>
        </SettingsCard>
      ) : (
        <SettingsCard ordered label={t("audit.title")}>
          {audit.events.map((event) => (
            <li key={event.id} className="space-y-1 px-4.5 py-3">
              <p className="text-sm break-words">{sentence(event, t)}</p>
              <p className="text-sm text-muted-foreground">
                <time dateTime={event.created_at}>{formatDateTime(event.created_at, preferences.locale)}</time>
              </p>
            </li>
          ))}
        </SettingsCard>
      )}
      {audit.events !== undefined && audit.nextCursor !== null && (
        <div className="flex flex-wrap items-center gap-3">
          <Button variant="outline" disabled={loading} onClick={() => void more()}>
            {t("audit.more")}
          </Button>
          {failed !== undefined && (
            <p role="alert" className="text-sm text-destructive">
              {failed}
            </p>
          )}
        </div>
      )}
    </SettingsSection>
  );
});
