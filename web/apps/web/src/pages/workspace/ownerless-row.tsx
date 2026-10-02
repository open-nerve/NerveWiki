import { observer } from "mobx-react-lite";
import { useState } from "react";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { memberWho } from "../../app/member-summary";
import { Button } from "../../components/ui/button";
import { formatBytes, formatDate } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { OwnerlessNotebook } from "../../services/ownerless.service";
import { useStore } from "../../stores/context";

/** twinKey is what tells two ownerless notebooks apart in the list: the former owner and the name. */
function twinKey(notebook: OwnerlessNotebook): string {
  return `${notebook.former_owner.user_id} ${notebook.name}`;
}

/**
 * twinsOf is the ids of the notebooks of list that another of list has the
 * same name and former owner as: nothing else shown tells them apart, so
 * their rows show the end of their ids (M3 Codex review R6).
 */
export function twinsOf(list: OwnerlessNotebook[]): Set<string> {
  const counts = new Map<string, number>();
  for (const notebook of list) {
    counts.set(twinKey(notebook), (counts.get(twinKey(notebook)) ?? 0) + 1);
  }
  return new Set(list.filter((notebook) => (counts.get(twinKey(notebook)) ?? 0) > 1).map((notebook) => notebook.id));
}

type OwnerlessRowProps = {
  notebook: OwnerlessNotebook;
  /** Whether another notebook listed has its name and former owner: its id tells them apart. */
  twin: boolean;
  /** Takes the notebook over; a refusal is the list's to say. */
  takeOver: () => Promise<void>;
  /** Deletes the notebook. */
  remove: () => Promise<void>;
  /** Where the focus goes once the notebook is deleted, with the row. */
  removed: () => void;
};

/**
 * OwnerlessRow is an ownerless notebook (M3/P5 design 3.3): its name, its
 * former owner, its access, the members it has left, since when it is
 * ownerless, its last activity and its size. Take over goes out at once,
 * one at a time; Delete asks for the notebook's name first, as deleting a
 * notebook does. A twin shows the last six digits of its id, which its
 * controls and the deletion's title name it by too.
 */
export const OwnerlessRow = observer(function OwnerlessRow({
  notebook,
  twin,
  takeOver,
  remove,
  removed,
}: OwnerlessRowProps) {
  const { preferences } = useStore();
  const t = useT();
  const [sending, setSending] = useState(false);
  const { locale } = preferences;
  // Two notebooks may have the same name: the controls name each by its former owner too, and a twin by its id.
  const shortId = twin ? notebook.id.replaceAll("-", "").slice(-6) : undefined;
  const name = shortId === undefined ? notebook.name : t("ownerless.nameWithId", { name: notebook.name, id: shortId });
  const which = { name, former: memberWho(notebook.former_owner, t) };

  async function take() {
    setSending(true);
    try {
      await takeOver();
    } finally {
      setSending(false);
    }
  }

  const details = [
    t(`access.${notebook.workspace_access}`),
    t("ownerless.members", { count: notebook.member_count }),
    t("ownerless.since", { date: formatDate(notebook.ownerless_since, locale) }),
    t("ownerless.activity", { date: formatDate(notebook.last_activity_at, locale) }),
    t("ownerless.size", { size: formatBytes(notebook.size_bytes, locale) }),
  ];
  return (
    <li className="flex flex-wrap items-center justify-between gap-4 p-4">
      <div className="min-w-0 space-y-1">
        <p className="font-medium break-words">{notebook.name}</p>
        {shortId !== undefined && <p className="text-sm text-muted-foreground">{t("ownerless.id", { id: shortId })}</p>}
        <p className="text-sm break-all text-muted-foreground">
          {t("ownerless.formerOwner", { name: memberWho(notebook.former_owner, t) })}
        </p>
        <p className="text-sm text-muted-foreground">{details.join(" · ")}</p>
      </div>
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          disabled={sending}
          aria-label={t("ownerless.takeOverLabel", which)}
          onClick={() => void take()}
        >
          {t("ownerless.takeOver")}
        </Button>
        <ConfirmDialog
          trigger={
            <Button variant="outline" aria-label={t("ownerless.deleteLabel", which)}>
              {t("ownerless.delete")}
            </Button>
          }
          title={t("ownerless.deleteTitle", { name })}
          description={t("ownerless.deleteBody")}
          typedConfirmation={{ label: t("notebookSettings.typeName", { name: notebook.name }), value: notebook.name }}
          confirmLabel={t("notebookSettings.deleteConfirm")}
          sendingLabel={t("notebookSettings.deleting")}
          cancelLabel={t("notebookSettings.cancel")}
          confirm={remove}
          focusAfter={removed}
        />
      </div>
    </li>
  );
});
