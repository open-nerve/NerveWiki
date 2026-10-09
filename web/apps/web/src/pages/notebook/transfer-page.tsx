import { observer } from "mobx-react-lite";
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";
import { useLocation } from "react-router";
import useSWR from "swr";

import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import { useAccount, useStore, useTransfers } from "../../stores/context";
import type { TransferStore } from "../../stores/transfer.store";
import { watchReader } from "../page/readers-input";
import { ExportDialog } from "./export-dialog";
import { useNotebook } from "./notebook-layout";
import { TransferJobRow } from "./transfer-job-row";
import { jobNames } from "./transfer-names";

/** focusJob is the route state of the page reached by a page's export: the job its row takes the focus for. */
export type FocusJob = { focusJob: string };

function focusJobOf(state: unknown): string | undefined {
  return typeof state === "object" && state !== null && "focusJob" in state && typeof state.focusJob === "string"
    ? state.focusJob
    : undefined;
}

/** The longest the jobs wait to be read again while one has an address: an address lives two hours at most. */
const longestWait = 60 * 60_000;

/**
 * pollInterval is how long the jobs wait to be read again (M7/P5 design
 * 4.2): a second while one is under way; otherwise until a minute before
 * the first download address expires, at least 30 seconds and at most an
 * hour, so that no link points to an address expired; never when none has
 * one.
 */
export function pollInterval(transfers: Pick<TransferStore, "active" | "jobs">, now: number): number {
  if (transfers.active) {
    return 1000;
  }
  const expiries = (transfers.jobs ?? [])
    .flatMap((job) => (job.download === null ? [] : [Date.parse(job.download.expires_at)]))
    .filter((expiry) => Number.isFinite(expiry));
  return expiries.length === 0 ? 0 : Math.min(longestWait, Math.max(30_000, Math.min(...expiries) - 60_000 - now));
}

/**
 * NotebookTransferPage is the notebook's imports and exports (M7/P5
 * design 4.3): whoever sees the notebook exports it, and sees their jobs;
 * its admins see everyone's.
 */
export const NotebookTransferPage = observer(function NotebookTransferPage() {
  const notebook = useNotebook();
  const rows = useRef(new Map<string, HTMLLIElement>());
  const heading = useRef<HTMLHeadingElement>(null);
  return (
    <div className="space-y-10">
      <ExportSection notebook={notebook} rows={rows} heading={heading} />
      <JobsSection notebook={notebook} rows={rows} heading={heading} />
    </div>
  );
});

type Rows = RefObject<Map<string, HTMLLIElement>>;

/**
 * ExportSection exports the whole notebook: the focus goes to the job's
 * row once it started, or to the jobs' title while they are not shown.
 */
function ExportSection({
  notebook,
  rows,
  heading,
}: {
  notebook: Notebook;
  rows: Rows;
  heading: RefObject<HTMLHeadingElement | null>;
}) {
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
            (rows.current.get(started.current) ?? heading.current)?.focus();
          }
        }}
      />
    </section>
  );
}

/**
 * JobsSection lists the jobs, the newest first, read again every second
 * while one is under way (pollInterval), and Load more for each page after
 * the first. Until the list is read, or while it cannot be, it says so
 * (NotLoaded); a read again that fails says so above the jobs held, which
 * stop changing meanwhile. Reached by a page's export, the job's row takes
 * the focus as it shows, unless the reader did something meanwhile or the
 * focus is elsewhere (v0.1 design 13.2, item 26).
 */
const JobsSection = observer(function JobsSection({
  notebook,
  rows,
  heading,
}: {
  notebook: Notebook;
  rows: Rows;
  heading: RefObject<HTMLHeadingElement | null>;
}) {
  const transfers = useTransfers(notebook);
  const { me } = useAccount();
  const { preferences } = useStore();
  const t = useT();
  const { error, mutate } = useSWR(["transfer-jobs", notebook.id], () => transfers.load(), {
    // Made anew each render, which SWR's polling (2.5.1) takes as a new interval and is timed again by: once no job
    // was under way (0, no timer), an export started here polls again as it shows.
    refreshInterval: () => pollInterval(transfers, Date.now()),
    // A second's polling is not deduplicated away.
    dedupingInterval: 500,
  });
  const more = useMore(transfers, rows, heading);
  const jobs = transfers.loaded ? transfers.jobs : undefined;
  useArriving(rows, jobs);
  const reread = useCallback(() => void mutate(), [mutate]);
  const rowRef = useRowRefs(rows);
  const names = jobs === undefined ? undefined : jobNames(jobs, t, preferences.locale, me.id);

  return (
    <section className="space-y-4">
      <div className="max-w-2xl space-y-1">
        <h2 ref={heading} tabIndex={-1} className="text-lg font-semibold outline-none">
          {t("transfer.jobsTitle")}
        </h2>
        <p className="text-sm text-muted-foreground">{t("transfer.jobsBody")}</p>
      </div>
      {jobs === undefined || names === undefined ? (
        <NotLoaded error={error} retry={reread} />
      ) : (
        <>
          {error !== undefined && <NotLoaded error={error} retry={reread} />}
          {jobs.length === 0 ? (
            <p className="text-muted-foreground">{t("transfer.none")}</p>
          ) : (
            <ol aria-label={t("transfer.jobsTitle")} className="divide-y rounded-md border">
              {jobs.map((job) => (
                <TransferJobRow
                  key={job.id}
                  notebook={notebook}
                  job={job}
                  name={names.get(job.id) ?? job.name}
                  rowRef={rowRef(job.id)}
                  reread={reread}
                />
              ))}
            </ol>
          )}
        </>
      )}
      {jobs !== undefined && transfers.nextCursor !== null && (
        <div className="flex flex-wrap items-center gap-3">
          <MoreButton more={more} />
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

/** useRowRefs gives each job's row a ref of its own, the same each render, which keeps rows by job id. */
function useRowRefs(rows: Rows): (id: string) => (element: HTMLLIElement | null) => void {
  const refs = useRef(new Map<string, (element: HTMLLIElement | null) => void>());
  return (id) => {
    let ref = refs.current.get(id);
    if (ref === undefined) {
      ref = (element) => {
        if (element === null) {
          rows.current.delete(id);
        } else {
          rows.current.set(id, element);
        }
      };
      refs.current.set(id, ref);
    }
    return ref;
  };
}

/**
 * useArriving gives the focus to the row of the job the route's state
 * names (a page's export), once, as the list shows it: unless the reader
 * did something since the page showed, or the focus is somewhere already.
 * A list read without it gives up.
 */
function useArriving(rows: Rows, jobs: { id: string }[] | undefined): void {
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
    if (id === undefined || jobs === undefined) {
      return;
    }
    pending.current = undefined;
    const acted = reader.current?.acted() === true;
    reader.current?.end();
    const element = jobs.some((job) => job.id === id) ? rows.current.get(id) : undefined;
    const at = document.activeElement;
    if (element !== undefined && !acted && (at === null || at === document.body)) {
      element.focus();
    }
  }, [jobs, rows]);
}

/**
 * MoreButton is Load more. Gone with the focus, the last page read, it
 * gives the focus to the section's title before it leaves the document:
 * not to the page's start.
 */
function MoreButton({ more }: { more: ReturnType<typeof useMore> }) {
  const t = useT();
  const leaving = useRef(more.left);
  useEffect(() => {
    leaving.current = more.left;
  });
  const { button } = more;
  useLayoutEffect(() => {
    const element = button.current;
    return () => {
      if (element?.contains(document.activeElement)) {
        leaving.current();
      }
    };
  }, [button]);
  return (
    <Button
      ref={button}
      variant="outline"
      aria-busy={more.reading || undefined}
      aria-disabled={more.reading || undefined}
      onClick={() => void more.read()}
    >
      {t("transfer.more")}
    </Button>
  );
}

/**
 * useMore reads the next page of the jobs. As the last is read, Load more
 * goes: the focus falls to the first job it added, or the list's last, or
 * the section's title, unless the reader did something meanwhile or the
 * focus is elsewhere than where Load more was (v0.1 design 13.2, item 26);
 * Load more gone with the focus gave it to the title (left), which is
 * where Load more was.
 */
function useMore(transfers: TransferStore, rows: Rows, heading: RefObject<HTMLHeadingElement | null>) {
  const mounted = useMounted();
  const [reading, setReading] = useState(false);
  const [failure, setFailure] = useState<unknown>(undefined);
  // Where the focus falls as Load more goes: a job's row, or the section's title (null).
  const [focusing, setFocusing] = useState<string | null | undefined>(undefined);
  const busy = useRef(false);
  const button = useRef<HTMLButtonElement>(null);
  // The title, once Load more gone with the focus gave it there.
  const gaveTo = useRef<Element | null>(null);
  useEffect(() => {
    if (focusing === undefined) {
      return;
    }
    const at = document.activeElement;
    if (at === null || at === document.body || at === gaveTo.current) {
      (focusing === null ? heading.current : rows.current.get(focusing))?.focus({ preventScroll: true });
    }
  }, [focusing, heading, rows]);

  function left(): void {
    gaveTo.current = heading.current;
    heading.current?.focus({ preventScroll: true });
  }

  async function read(): Promise<void> {
    if (busy.current) {
      return;
    }
    busy.current = true;
    setReading(true);
    setFailure(undefined);
    setFocusing(undefined);
    gaveTo.current = null;
    const reader = watchReader({ on: button.current });
    try {
      const added = await transfers.more();
      const at = document.activeElement;
      const held =
        !reader.acted() && (at === button.current || at === null || at === document.body || at === gaveTo.current);
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

  return { reading, failure, button, read, left };
}
