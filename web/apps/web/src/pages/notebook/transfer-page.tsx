import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState, type RefObject } from "react";
import { useLocation } from "react-router";
import useSWR from "swr";

import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TransferJob } from "../../services/transfer.service";
import { useTransfers } from "../../stores/context";
import type { TransferStore } from "../../stores/transfer.store";
import { watchReader } from "../page/readers-input";
import { ExportDialog } from "./export-dialog";
import { useNotebook } from "./notebook-layout";
import { TransferJobRow } from "./transfer-job-row";

/** focusJob is the route state of the page reached by a page's export: the job its row takes the focus for. */
export type FocusJob = { focusJob: string };

function focusJobOf(state: unknown): string | undefined {
  return typeof state === "object" && state !== null && "focusJob" in state && typeof state.focusJob === "string"
    ? state.focusJob
    : undefined;
}

/**
 * pollInterval is how long the jobs wait to be read again (M7/P5 design
 * 4.2): a second while one is under way; otherwise until a minute before
 * the first download address expires, at least 30 seconds, so that no
 * link points to an address expired; never when none has one.
 */
export function pollInterval(transfers: Pick<TransferStore, "active" | "jobs">, now: number): number {
  if (transfers.active) {
    return 1000;
  }
  const expiries = (transfers.jobs ?? []).flatMap((job) =>
    job.download === null ? [] : [Date.parse(job.download.expires_at)]
  );
  return expiries.length === 0 ? 0 : Math.max(30_000, Math.min(...expiries) - 60_000 - now);
}

/**
 * NotebookTransferPage is the notebook's imports and exports (M7/P5
 * design 4.3): whoever sees the notebook exports it, and sees their jobs;
 * its admins see everyone's.
 */
export const NotebookTransferPage = observer(function NotebookTransferPage() {
  const notebook = useNotebook();
  const rows = useRef(new Map<string, HTMLLIElement>());
  return (
    <div className="space-y-10">
      <ExportSection notebook={notebook} rows={rows} />
      <JobsSection notebook={notebook} rows={rows} />
    </div>
  );
});

type Rows = RefObject<Map<string, HTMLLIElement>>;

/** ExportSection exports the whole notebook: the focus goes to the job's row once it started. */
function ExportSection({ notebook, rows }: { notebook: Notebook; rows: Rows }) {
  const t = useT();
  const started = useRef<string | undefined>(undefined);
  return (
    <section className="max-w-2xl space-y-3">
      <h2 className="text-lg font-semibold">{t("transfer.exportTitle")}</h2>
      <p className="text-sm text-muted-foreground">{t("transfer.exportBody")}</p>
      <ExportDialog
        notebook={notebook}
        trigger={<Button variant="outline">{t("transfer.exportNotebook")}</Button>}
        onStarted={(job) => (started.current = job.id)}
        focusAfter={() => {
          if (started.current !== undefined) {
            rows.current.get(started.current)?.focus();
          }
        }}
      />
    </section>
  );
}

/**
 * JobsSection lists the jobs, the newest first, read again every second
 * while one is under way (pollInterval), and Load more for each page after
 * the first. Reached by a page's export, the job's row takes the focus as
 * it shows, unless the reader did something meanwhile or the focus is
 * elsewhere (v0.1 design 13.2, item 26).
 */
const JobsSection = observer(function JobsSection({ notebook, rows }: { notebook: Notebook; rows: Rows }) {
  const transfers = useTransfers(notebook);
  const t = useT();
  const { error, mutate } = useSWR(["transfer-jobs", notebook.id], () => transfers.load(), {
    refreshInterval: () => pollInterval(transfers, Date.now()),
    // A second's polling is not deduplicated away.
    dedupingInterval: 500,
  });
  const heading = useRef<HTMLHeadingElement>(null);
  const more = useMore(transfers, heading);
  const jobs = transfers.jobs;
  useArriving(rows, jobs);

  function register(id: string, element: HTMLLIElement | null) {
    if (element === null) {
      rows.current.delete(id);
    } else {
      rows.current.set(id, element);
    }
  }

  return (
    <section className="space-y-4">
      <div className="max-w-2xl space-y-1">
        <h2 ref={heading} tabIndex={-1} className="text-lg font-semibold outline-none">
          {t("transfer.jobsTitle")}
        </h2>
        <p className="text-sm text-muted-foreground">{t("transfer.jobsBody")}</p>
      </div>
      {jobs === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : jobs.length === 0 ? (
        <p className="text-muted-foreground">{t("transfer.none")}</p>
      ) : (
        <ol aria-label={t("transfer.jobsTitle")} className="divide-y rounded-md border">
          {jobs.map((job: TransferJob) => (
            <TransferJobRow
              key={job.id}
              notebook={notebook}
              job={job}
              rowRef={(element) => {
                register(job.id, element);
                if (element !== null && job.id === more.focusing) {
                  more.focused.current = element;
                }
              }}
              reread={() => void mutate()}
            />
          ))}
        </ol>
      )}
      {jobs !== undefined && transfers.nextCursor !== null && (
        <div className="flex flex-wrap items-center gap-3">
          <Button
            ref={more.button}
            variant="outline"
            aria-busy={more.reading || undefined}
            aria-disabled={more.reading || undefined}
            onClick={() => void more.read()}
          >
            {t("transfer.more")}
          </Button>
          {more.failure !== undefined && (
            <p role="alert" className="text-sm text-destructive">
              {errorText(more.failure, t)}
            </p>
          )}
        </div>
      )}
    </section>
  );
});

/**
 * useArriving gives the focus to the row of the job the route's state
 * names (a page's export), once, as it shows among jobs: unless the reader
 * did something since the page showed, or the focus is somewhere already.
 */
function useArriving(rows: Rows, jobs: TransferJob[] | undefined): void {
  const pending = useRef(focusJobOf(useLocation().state));
  const reader = useRef<ReturnType<typeof watchReader> | undefined>(undefined);
  useEffect(() => {
    if (pending.current === undefined) {
      return;
    }
    const watching = watchReader();
    reader.current = watching;
    return () => watching.end();
  }, []);
  useEffect(() => {
    const id = pending.current;
    const element = id !== undefined && jobs?.some((job) => job.id === id) === true ? rows.current.get(id) : undefined;
    if (element === undefined) {
      return;
    }
    pending.current = undefined;
    const at = document.activeElement;
    if (reader.current?.acted() !== true && (at === null || at === document.body)) {
      element.focus();
    }
    reader.current?.end();
  }, [jobs, rows]);
}

/**
 * useMore reads the next page of the jobs. As the last is read, Load more
 * goes: the focus falls to the first job it added, or the list's last, or
 * the section's title, unless the reader did something meanwhile or the
 * focus is elsewhere than where Load more was (v0.1 design 13.2, item 26).
 */
function useMore(transfers: TransferStore, heading: RefObject<HTMLHeadingElement | null>) {
  const mounted = useMounted();
  const [reading, setReading] = useState(false);
  const [failure, setFailure] = useState<unknown>(undefined);
  // Where the focus falls as Load more goes: a job's row, or the section's title (null).
  const [focusing, setFocusing] = useState<string | null | undefined>(undefined);
  const busy = useRef(false);
  const button = useRef<HTMLButtonElement>(null);
  const focused = useRef<HTMLLIElement | null>(null);
  useEffect(() => {
    if (focusing === undefined) {
      return;
    }
    const at = document.activeElement;
    if (at === null || at === document.body) {
      (focusing === null ? heading.current : focused.current)?.focus({ preventScroll: true });
    }
  }, [focusing, heading]);

  async function read(): Promise<void> {
    if (busy.current) {
      return;
    }
    busy.current = true;
    setReading(true);
    setFailure(undefined);
    setFocusing(undefined);
    const reader = watchReader({ on: button.current });
    try {
      const added = await transfers.more();
      const at = document.activeElement;
      const held = !reader.acted() && (at === button.current || at === null || at === document.body);
      if (mounted() && held && transfers.nextCursor === null) {
        setFocusing(added[0]?.id ?? transfers.jobs?.at(-1)?.id ?? null);
      }
    } catch (failed) {
      if (mounted()) {
        setFailure(failed);
      }
    } finally {
      reader.end();
      busy.current = false;
      if (mounted()) {
        setReading(false);
      }
    }
  }

  return { reading, failure, focusing, focused, button, read };
}
