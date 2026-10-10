import { observer } from "mobx-react-lite";
import { useCallback, useId, useRef, useState, type ReactNode } from "react";

import { useFocusLeaving } from "../../app/focus-leaving";
import { errorText } from "../../app/problem-messages";
import { Button } from "../../components/ui/button";
import { formatBytes, formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TransferJob } from "../../services/transfer.service";
import { useAccount, useStore, useTransfers } from "../../stores/context";
import { under } from "../../stores/transfer.store";
import { jobTitle } from "./transfer-names";
import { failureText, TransferReport } from "./transfer-report";

/**
 * TransferJobRow is a job of the notebook (M7/P5 design 4.3): what it
 * does, its state and progress, who started it when not the account, when,
 * and, for an export that succeeded, its size and until when it is kept.
 * An export with an address downloads; a job under way cancels, until its
 * cancel is asked; an ended one shows its report. A cancel refused says
 * why in the row, and the list is read again. The row is named by name,
 * which tells it from the others, and so are its controls (v0.1 design
 * 13.2, item 17); a control gone with the focus gives it to the row. The
 * row takes the focus where the page gives it (its ref).
 */
export const TransferJobRow = observer(function TransferJobRow({
  notebook,
  job,
  name,
  rowRef,
  reread,
}: {
  notebook: Notebook;
  job: TransferJob;
  /** The row's name among the jobs: what it does, who started it, when (transfer-names). */
  name: string;
  rowRef?: (element: HTMLLIElement | null) => void;
  /** Reads the list again. */
  reread: () => void;
}) {
  const transfers = useTransfers(notebook);
  const { me } = useAccount();
  const { instance, preferences } = useStore();
  const t = useT();
  const stateId = useId();
  const detailsId = useId();
  const reportId = useId();
  const [reporting, setReporting] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const busy = useRef(false);
  const [refusal, setRefusal] = useState<unknown>(undefined);
  const row = useRef<HTMLLIElement | null>(null);
  const ref = useCallback(
    (element: HTMLLIElement | null) => {
      row.current = element;
      rowRef?.(element);
    },
    [rowRef]
  );
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
  const refused = refusal === undefined ? undefined : errorText(refusal, t);
  const toRow = () => row.current?.focus();

  async function cancel() {
    if (busy.current) {
      return;
    }
    busy.current = true;
    setRefusal(undefined);
    setCancelling(true);
    try {
      await transfers.cancel(job.id);
    } catch (error) {
      setRefusal(error);
      reread();
    } finally {
      busy.current = false;
      setCancelling(false);
    }
  }

  return (
    <li
      ref={ref}
      tabIndex={-1}
      aria-label={name}
      aria-describedby={`${stateId} ${detailsId}`}
      className="space-y-3 p-4 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <p className="font-medium break-words">{jobTitle(job, t)}</p>
          <p id={stateId} className="text-sm">
            {t(`transfer.state.${job.state}`)}
            {job.state === "running" && job.cancel_requested_at !== null && ` · ${t("transfer.cancelling")}`}
            {job.state === "failed" && ` · ${failureText(job.kind, job.report?.failure ?? "", t)}`}
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
          <p id={detailsId} className="text-sm break-words text-muted-foreground">
            {details.join(" · ")}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          {job.download !== null && (
            <Leaving left={toRow}>
              <a
                href={job.download.url}
                download
                aria-label={t("transfer.downloadOf", { name })}
                className="text-sm underline underline-offset-4"
              >
                {t("transfer.download")}
              </a>
            </Leaving>
          )}
          {under(job) && job.cancel_requested_at === null && (
            <Leaving left={toRow}>
              <Button
                variant="outline"
                aria-label={t("transfer.cancelOf", { name })}
                aria-busy={cancelling || undefined}
                aria-disabled={cancelling || undefined}
                onClick={() => void cancel()}
              >
                {t("transfer.cancel")}
              </Button>
            </Leaving>
          )}
          {job.report !== null && (
            <Button
              variant="ghost"
              aria-label={t("transfer.reportOf", { name })}
              aria-expanded={reporting}
              aria-controls={reporting ? reportId : undefined}
              onClick={() => setReporting(!reporting)}
            >
              {t("transfer.report")}
            </Button>
          )}
        </div>
      </div>
      {refused !== undefined && (
        <p role="alert" className="text-sm text-destructive">
          {refused}
        </p>
      )}
      {reporting && job.report !== null && <TransferReport notebook={notebook} job={job} id={reportId} />}
    </li>
  );
});

/**
 * Leaving holds a control of a row. Gone with the focus (its job
 * cancelled, ended, or its address expired as the list is read again), it
 * calls left, which gives the focus to the row, before it leaves the
 * document: the focus does not fall to the page's start.
 */
function Leaving({ left, children }: { left: () => void; children: ReactNode }) {
  const own = useRef<HTMLSpanElement>(null);
  useFocusLeaving(own, left);
  return (
    <span ref={own} className="contents">
      {children}
    </span>
  );
}
