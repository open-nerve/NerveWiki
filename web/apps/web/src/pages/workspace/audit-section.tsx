import { observer } from "mobx-react-lite";
import { useState } from "react";
import useSWR from "swr";

import { useFollowRole } from "../../app/follow-role";
import { memberWho } from "../../app/member-summary";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
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
 * button, which tries again.
 */
export const AuditSection = observer(function AuditSection({ workspace }: { workspace: Workspace }) {
  const audit = useAudit(workspace);
  const { preferences } = useStore();
  const t = useT();
  const { error, mutate } = useSWR(["notebook-audit", workspace.id], () => audit.load(), {
    onError: useFollowRole(),
  });
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
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="space-y-4">
      <div className="max-w-2xl space-y-1">
        <h2 className="text-lg font-semibold">{t("audit.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("audit.body")}</p>
      </div>
      {audit.events === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : audit.events.length === 0 ? (
        <p className="text-muted-foreground">{t("audit.none")}</p>
      ) : (
        <ol aria-label={t("audit.title")} className="divide-y rounded-md border">
          {audit.events.map((event) => (
            <li key={event.id} className="space-y-1 p-4">
              <p className="break-words">{sentence(event, t)}</p>
              <p className="text-sm text-muted-foreground">
                <time dateTime={event.created_at}>{formatDateTime(event.created_at, preferences.locale)}</time>
              </p>
            </li>
          ))}
        </ol>
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
    </section>
  );
});
