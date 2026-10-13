import { observer } from "mobx-react-lite";
import { useId, useRef, useState } from "react";
import { Link, useLocation } from "react-router";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";

import { useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { useT } from "../../i18n/i18n";
import { readPages } from "../../lib/read-pages";
import type { TrashItem, TrashItemPage } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { LoadMoreButton, useLoadMore } from "../load-more";
import { watchReader } from "../page/readers-input";
import { useWorkspace } from "../workspace/workspace-layout";
import { useNotebook } from "./notebook-layout";
import {
  openOf,
  refusalText,
  restoredOf,
  restoredText,
  TrashRow,
  twinsIn,
  useTrashRestore,
  type TrashRestored,
} from "./trash-row";

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
 * restore refused says why above the list.
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
  const { cache } = useSWRConfig();
  // A read reads as many pages as the list has, by the cache as it reads: what More added, the page come back to.
  const cached = () => cache.get(unstable_serialize(key));
  const { data, error, mutate } = useSWR(key, () =>
    readPages((cursor) => pages.trash(cursor), (cached()?.data as TrashItemPage[] | undefined)?.length ?? 1)
  );
  const restore = useTrashRestore(notebook);
  const { state } = useLocation();
  const [said, setSaid] = useState<TrashRestored | undefined>(() => restoredOf(state));
  const [refused, setRefused] = useState<{ error: unknown; item: TrashItem }>();
  // The status line describes the title, where the focus lands: it is heard with it.
  const status = useId();
  // The link of the item the focus falls to as More goes.
  const focused = useRef<HTMLAnchorElement>(null);
  const cursor = data?.at(-1)?.next_cursor ?? undefined;
  const more = useLoadMore(
    async () => {
      const next = await pages.trash(cursor);
      // Added after the page it was read after, which a read meanwhile may have read again.
      const read = await mutate(
        (list) => (list !== undefined && list.at(-1)?.next_cursor === cursor ? [...list, next] : list),
        { revalidate: false }
      );
      if (cached()?.isValidating === true) {
        // A read out as it was added answers what is older than the addition: SWR drops it. Another reads it all.
        void mutate();
      }
      const added = read?.at(-1) === next;
      return {
        added: added ? next.data.map((item) => item.id) : [],
        whole: added && next.next_cursor === null,
        last: read?.at(-1)?.data.at(-1)?.id,
      };
    },
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
        setSaid(restored);
        const at = document.activeElement;
        if (!reader.acted() && (at === from || at === null || at === document.body || at === heading.current)) {
          heading.current?.focus();
        }
      }
    } catch (refusal) {
      if (mounted()) {
        setRefused({ error: refusal, item });
      }
    } finally {
      reader.end();
    }
  }

  const items = data?.flatMap((read) => read.data) ?? [];
  const twinOf = twinsIn(items, locale);
  const home = `/${workspace.slug}/notebooks/${notebook.id}`;
  const open =
    said === undefined ? undefined : openOf({ id: said.id, kind: said.kind, parent: said.parent?.id ?? null }, home);
  const failed = refused === undefined ? undefined : refusalText(refused.error, refused.item, t);
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
        {said !== undefined && open !== undefined && (
          <>
            {restoredText(said, t)}{" "}
            <Link to={open.to} state={open.state} className="underline underline-offset-4">
              {t("trash.open")}
            </Link>
          </>
        )}
      </output>
      {failed !== undefined && <Alert>{failed}</Alert>}
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
