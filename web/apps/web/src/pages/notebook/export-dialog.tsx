import { observer } from "mobx-react-lite";
import type { ReactElement } from "react";
import useSWR, { useSWRConfig } from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import type { HeldDialog } from "../../app/held-dialog";
import { formatDuration } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import type { TransferJob } from "../../services/transfer.service";
import { useStore, useTransfers } from "../../stores/context";

/**
 * How the dialog opens: by its trigger, the focus going to focusAfter once
 * the export started; or held by its caller, a menu's item, which moves
 * the focus itself.
 */
type Opening =
  | { trigger: ReactElement; focusAfter: () => void; held?: never }
  | { held: HeldDialog; trigger?: never; focusAfter?: never };

type ExportDialogProps = Opening & {
  notebook: Notebook;
  /** The page exported with its subtree; the whole notebook when absent. */
  page?: TreeNode;
  /** Told the job as the export starts, before the dialog closes. */
  onStarted: (job: TransferJob) => void;
};

/**
 * ExportDialog asks to export the notebook, or a page with its subtree
 * (M7/P5 design 4.5): it says what the zip holds and how long a succeeded
 * export is kept, the instance's export TTL, which it reads. Confirmed,
 * it starts the export, whose job goes first in the notebook's jobs, read
 * again then: they are read every second while it runs. A refusal stays
 * in the dialog: an export of the account's under way, a queue full, the
 * storage full, the notebook or the page gone.
 */
export const ExportDialog = observer(function ExportDialog({
  notebook,
  page,
  onStarted,
  ...opening
}: ExportDialogProps) {
  const transfers = useTransfers(notebook);
  const { instance, preferences } = useStore();
  const t = useT();
  const { mutate } = useSWRConfig();
  useSWR("instance", () => instance.load());
  const ttl = instance.info?.export_ttl_seconds;
  return (
    <ConfirmDialog
      {...opening}
      title={
        page === undefined
          ? t("transfer.exportNotebookTitle", { name: notebook.name })
          : t("transfer.exportPageTitle", { name: page.name })
      }
      description={
        ttl === undefined
          ? t("transfer.exportDescriptionNoTtl")
          : t("transfer.exportDescription", { ttl: formatDuration(ttl, preferences.locale) })
      }
      confirmLabel={t("transfer.exportConfirm")}
      sendingLabel={t("transfer.exporting")}
      cancelLabel={t("transfer.dialogCancel")}
      tone="default"
      confirm={async () => {
        const job = await transfers.start(page?.id ?? null);
        // The jobs are read again: a list whose read failed is polled no more (SWR skips its polling while the key
        // holds an error) until it is read.
        void mutate(["transfer-jobs", notebook.id]);
        onStarted(job);
      }}
      texts={{
        server_busy: "transfer.queueFull",
        "page.not_found": "transfer.pageGone",
        "transfer.busy": "transfer.exportBusy",
      }}
    />
  );
});
