import { observer } from "mobx-react-lite";
import { useId, useRef, useState } from "react";
import { Link, useLocation } from "react-router";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";

import { useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { writesPages } from "../../app/effective-role";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { useT } from "../../i18n/i18n";
import { readPages } from "../../lib/read-pages";
import type { TrashItem, TrashItemPage } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { LoadMoreButton, readNext, useLoadMore } from "../load-more";
import { openOf } from "../open-node";
import { watchReader } from "../page/readers-input";
import { useWorkspace } from "../workspace/workspace-layout";
import { useNotebook } from "./notebook-layout";
import type { FocusJob } from "./transfer-page";
import {
  namesOf,
  offeredOf,
  refusedText,
  restoredOf,
  restoredText,
  RestoreButton,
  startedOf,
  TrashRow,
  twinsIn,
  useBackgroundRestore,
  useTrashRestore,
  type Refusal,
  type Started,
  type TrashRestored,
} from "./trash-row";

/**
 * Said is what the trash's status line says: an item restored; its
 * restore started in the background; or, the start's answer lost, that
 * none started (none), or that the jobs could not be read (unread).
 */
type Said = { restored: TrashRestored } | { started: Started } | { lost: "none" | "unread"; name: string };

/** saidOf is what the status line says as the trash shows, arrived at from an item's preview. */
function saidOf(state: unknown): Said | undefined {
  const restored = restoredOf(state);
  const started = startedOf(state);
  return restored !== undefined ? { restored } : started !== undefined ? { started } : undefined;
}

/**
 * TrashPage is a notebook's trash (M8 design 4.5, 4.13; M8/P2 design 4.2):
 * what was deleted from it, each item a node deleted with those under it,
 * newer first, while it can be restored, a page of them at a time (its
 * rows, trash-row.tsx), More reading the next; read again (an event, a
 * refocus, a connection, a restore) as many pages as were read, from the
 * first (readPages). Anyone who reads the notebook sees it; a writer
 * restores an item from it, and stays: the status line says where it went,
 * with Open, and the title takes the focus, the line describing it, unless
 * the reader went on meanwhile (v0.1 design 13.2, item 26). Arrived
 * at from an item restored in its preview, it says so the same way. A
 * restore refused says why above the list; refused as too large, with
 * Restore in the background beside it (M8/P4 design 4.2), whose job
 * started the status line says the same way, leading to its progress; its
 * start refused says why instead, still offered when the notebook or the
 * server was busy; its answer lost, the line says whether it started.
 */
export const TrashPage = observer(function TrashPage() {
  const workspace = useWorkspace();
  const notebook = useNotebook();
  const pages = usePageTree(notebook);
  const { locale } = useStore().preferences;
  const t = useT();
  const mounted = useMounted();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const title = t("trash.title");
  useDocumentTitle(title, notebook.name);
  const key = ["trash", notebook.id];
  const swr = useSWRConfig();
  // A read reads as many pages as the list has, by the cache as it reads: what More added, the page come back to.
  const cached = () => swr.cache.get(unstable_serialize(key));
  const { data, error, mutate } = useSWR(key, () =>
    readPages((cursor) => pages.trash(cursor), (cached()?.data as TrashItemPage[] | undefined)?.length ?? 1)
  );
  const restore = useTrashRestore(notebook);
  const start = useBackgroundRestore(notebook);
  const { state } = useLocation();
  const [said, setSaid] = useState<Said | undefined>(() => saidOf(state));
  const [refused, setRefused] = useState<Refusal>();
  // The status line describes the title, where the focus lands: it is heard with it.
  const status = useId();
  // The link of the item the focus falls to as More goes.
  const focused = useRef<HTMLAnchorElement>(null);
  const cursor = data?.at(-1)?.next_cursor ?? undefined;
  const more = useLoadMore(
    async () =>
      readNext(
        swr,
        key,
        cursor,
        (after) => pages.trash(after),
        (read) => read.data.map((item) => item.id)
      ),
    () => focused.current,
    heading
  );

  async function restoreItem(item: TrashItem): Promise<void> {
    setSaid(undefined);
    setRefused(undefined);
    // Where the reader was as they pressed it: the title takes the focus only from there, from nothing, or from
    // itself (the row gone with it), the reader having done nothing meanwhile (v0.1 design 13.2, item 26).
    const from = document.activeElement;
    const reader = watchReader({ on: from === document.body ? null : from });
    try {
      const restored = await restore(item);
      if (mounted()) {
        setSaid({ restored });
        toTitle(from, reader);
      }
    } catch (refusal) {
      if (mounted()) {
        setRefused({ error: refusal, item });
      }
    } finally {
      reader.end();
    }
  }

  // The item refused as too large, its restore in the background: the refusal stays as it is out, and as its answer
  // was lost; refused, it says why instead.
  async function startItem(item: TrashItem): Promise<void> {
    setSaid(undefined);
    const from = document.activeElement;
    const reader = watchReader({ on: from === document.body ? null : from });
    try {
      const started = await start(item);
      if (mounted()) {
        if ("job" in started) {
          setRefused(undefined);
          setSaid({ started: { name: item.name, job: started.job.id } });
          toTitle(from, reader);
        } else {
          setSaid({ lost: started.lost, name: item.name });
        }
      }
    } catch (refusal) {
      if (mounted()) {
        setRefused({ error: refusal, item, start: true });
      }
    } finally {
      reader.end();
    }
  }

  // The title takes the focus from where the reader pressed, from nothing, or from itself (the control gone with it),
  // the reader having done nothing meanwhile (v0.1 design 13.2, item 26).
  function toTitle(from: Element | null, reader: ReturnType<typeof watchReader>): void {
    const at = document.activeElement;
    if (!reader.acted() && (at === from || at === null || at === document.body || at === heading.current)) {
      heading.current?.focus();
    }
  }

  const items = data?.flatMap((read) => read.data) ?? [];
  const twinOf = twinsIn(items, locale);
  const home = `/${workspace.slug}/notebooks/${notebook.id}`;
  const failed = refused === undefined ? undefined : refusedText(refused, t);
  const offered = writesPages(notebook.role) ? offeredOf(refused) : undefined;
  const now = Date.now();
  return (
    <section className="space-y-4">
      <div className="space-y-1">
        <h1
          ref={heading}
          tabIndex={-1}
          aria-describedby={said === undefined ? undefined : status}
          className="text-2xl font-semibold outline-none"
        >
          {title}
        </h1>
        <p className="text-sm text-muted-foreground">{t("trash.hint")}</p>
      </div>
      {/* There before it says anything: a status that comes with its region is not always read. */}
      <output id={status} className="block text-sm empty:sr-only">
        {said !== undefined && <SaidText said={said} home={home} />}
      </output>
      {failed !== undefined && <Alert>{failed}</Alert>}
      {offered !== undefined && (
        <RestoreButton
          label={namesOf(offered, twinOf(offered), t, locale).background}
          text={t("trash.background", { name: offered.name })}
          restore={() => startItem(offered)}
          left={() => heading.current?.focus()}
        />
      )}
      {data === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : items.length === 0 ? (
        <p className="text-muted-foreground">{t("trash.empty")}</p>
      ) : (
        <ul aria-label={title} className="divide-y rounded-md border">
          {items.map((item) => (
            <TrashRow
              key={item.id}
              notebook={notebook}
              item={item}
              twin={twinOf(item)}
              restore={() => restoreItem(item)}
              left={() => heading.current?.focus()}
              linkRef={item.id === more.focusing ? focused : undefined}
              now={now}
            />
          ))}
        </ul>
      )}
      {more.failure !== undefined && <Alert>{errorText(more.failure, t)}</Alert>}
      {cursor !== undefined && <LoadMoreButton more={more} label={t("trash.more")} />}
    </section>
  );
});

/**
 * SaidText is what said says, of the notebook whose home is home: an item
 * restored leading to it; a restore started in the background leading to
 * its progress, its job's row taking the focus there; or, its answer lost,
 * whether it started, the jobs leading there when they could not be read.
 */
function SaidText({ said, home }: { said: Said; home: string }) {
  const t = useT();
  const jobs = `${home}/settings/transfer`;
  if ("restored" in said) {
    const { restored } = said;
    const open = openOf({ id: restored.id, kind: restored.kind, parent: restored.parent?.id ?? null }, home);
    return (
      <>
        {restoredText(restored, t)}{" "}
        <Link to={open.to} state={open.state} className="underline underline-offset-4">
          {t("trash.open")}
        </Link>
      </>
    );
  }
  if ("started" in said) {
    return (
      <>
        {t("trash.backgroundStarted", { name: said.started.name })}{" "}
        <Link
          to={jobs}
          state={{ focusJob: said.started.job } satisfies FocusJob}
          className="underline underline-offset-4"
        >
          {t("transfer.seeProgress")}
        </Link>
      </>
    );
  }
  return said.lost === "none" ? (
    t("trash.backgroundNotStarted", { name: said.name })
  ) : (
    <>
      {t("trash.backgroundUnknown", { name: said.name })}{" "}
      <Link to={jobs} className="underline underline-offset-4">
        {t("transfer.seeJobs")}
      </Link>
    </>
  );
}
