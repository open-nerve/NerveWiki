import { observer } from "mobx-react-lite";
import { useId, useState, type Ref } from "react";

import { errorText } from "../../app/problem-messages";
import { Button } from "../../components/ui/button";
import { formatBytes, formatDateTime } from "../../i18n/format";
import { useT, type Translate } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TransferJob } from "../../services/transfer.service";
import { useAccount, useStore, useTransfers } from "../../stores/context";
import { failureText, TransferReport } from "./transfer-report";

/** title is what a job does: an export of the whole notebook or of a page, or an import. */
function title(job: TransferJob, t: Translate): string {
  if (job.kind === "import") {
    return t("transfer.importOf", { name: job.name });
  }
  return job.root_id === null ? t("transfer.exportOfNotebook") : t("transfer.exportOfPage", { name: job.name });
}

/** under tells whether a job is under way: queued or running. */
function under(job: TransferJob): boolean {
  return job.state === "queued" || job.state === "running";
}

/**
 * TransferJobRow is a job of the notebook (M7/P5 design 4.3): what it
 * does, its state and progress, who started it when not the account, when,
 * and, for an export that succeeded, its size and until when it is kept.
 * An export with an address downloads; a job under way cancels; an ended
 * one shows its report. A cancel refused says why in the row, and the list
 * is read again. The row takes the focus where the page gives it (its ref).
 */
export const TransferJobRow = observer(function TransferJobRow({
  notebook,
  job,
  rowRef,
  reread,
}: {
  notebook: Notebook;
  job: TransferJob;
  rowRef?: Ref<HTMLLIElement>;
  /** Reads the list again. */
  reread: () => void;
}) {
  const transfers = useTransfers(notebook);
  const { me } = useAccount();
  const { instance, preferences } = useStore();
  const t = useT();
  const titleId = useId();
  const reportId = useId();
  const [reporting, setReporting] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [refusal, setRefusal] = useState<unknown>(undefined);
  const { locale } = preferences;
  const { done, total } = job.progress;
  const ttl = instance.info?.export_ttl_seconds;
  const details = [
    job.created_by.user_id === me.id ? undefined : t("transfer.startedBy", { name: job.created_by.display_name }),
    t("transfer.startedAt", { time: formatDateTime(job.created_at, locale) }),
    job.finished_at === null ? undefined : t("transfer.endedAt", { time: formatDateTime(job.finished_at, locale) }),
    job.result_bytes === null ? undefined : formatBytes(job.result_bytes, locale),
    job.kind !== "export" || job.state !== "succeeded" || job.finished_at === null || ttl === undefined
      ? undefined
      : t("transfer.keptUntil", {
          time: formatDateTime(new Date(Date.parse(job.finished_at) + ttl * 1000).toISOString(), locale),
        }),
  ].filter((detail) => detail !== undefined);

  async function cancel() {
    setRefusal(undefined);
    setCancelling(true);
    try {
      await transfers.cancel(job.id);
    } catch (error) {
      setRefusal(error);
      reread();
    } finally {
      setCancelling(false);
    }
  }

  return (
    <li
      ref={rowRef}
      tabIndex={-1}
      aria-labelledby={titleId}
      className="space-y-3 p-4 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <p id={titleId} className="font-medium break-words">
            {title(job, t)}
          </p>
          <p className="text-sm">
            {t(`transfer.state.${job.state}`)}
            {job.state === "running" && job.cancel_requested_at !== null && ` · ${t("transfer.cancelling")}`}
            {job.state === "failed" && ` · ${failureText(job.report?.failure ?? "", t)}`}
          </p>
          {under(job) &&
            (total > 0 ? (
              <progress
                className="w-48"
                max={total}
                value={done}
                aria-label={t("transfer.progress", { done, total })}
              />
            ) : (
              <progress className="w-48" aria-label={t(`transfer.state.${job.state}`)} />
            ))}
          <p className="text-sm break-words text-muted-foreground">{details.join(" · ")}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          {job.download !== null && (
            <a href={job.download.url} download className="text-sm underline underline-offset-4">
              {t("transfer.download")}
            </a>
          )}
          {under(job) && (
            <Button variant="outline" disabled={cancelling} onClick={() => void cancel()}>
              {t("transfer.cancel")}
            </Button>
          )}
          {job.report !== null && (
            <Button
              variant="ghost"
              aria-expanded={reporting}
              aria-controls={reporting ? reportId : undefined}
              onClick={() => setReporting(!reporting)}
            >
              {t("transfer.report")}
            </Button>
          )}
        </div>
      </div>
      {refusal !== undefined && (
        <p role="alert" className="text-sm text-destructive">
          {errorText(refusal, t)}
        </p>
      )}
      {reporting && job.report !== null && <TransferReport notebook={notebook} job={job} id={reportId} />}
    </li>
  );
});
