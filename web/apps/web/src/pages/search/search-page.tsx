import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState } from "react";
import { Link, useLocation, useSearchParams } from "react-router";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";

import { useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { errorText, fieldErrors } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { Label } from "../../components/ui/label";
import { NativeSelect } from "../../components/ui/native-select";
import { useT } from "../../i18n/i18n";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { idPattern } from "../../lib/ids";
import { readPages } from "../../lib/read-pages";
import { shortId, twinsOf } from "../../lib/twins";
import { ApiError } from "../../services/api";
import type { SearchHit, SearchPage as HitPage } from "../../services/page.service";
import { useNotebooks, useSearch, useStore } from "../../stores/context";
import { LoadMoreButton, readNext, useLoadMore } from "../load-more";
import { useWorkspace } from "../workspace/workspace-layout";
import { pathOf, SearchHitRow, type Twin } from "./search-hit";
import { moreText, statusText } from "./search-status";

/** A space that does not break: said after a notice said again, so that a screen reader reads it again. */
const again = String.fromCharCode(0xa0);

/**
 * unchanging is SWR's options for a search: it is read as it is asked for
 * alone, not again as the window has the focus back, nor on a connection
 * (M8 design 4.13); its key has no event's handler, nor is it read again
 * on the stream's connection (app/event-stream.tsx).
 */
const unchanging = { revalidateIfStale: false, revalidateOnFocus: false, revalidateOnReconnect: false };

/** hitsOf are the hits of the pages read, each once: a rename can list a node on two pages (v0.1 design 13.2, item 19). */
function hitsOf(pages: readonly HitPage[]): SearchHit[] {
  const seen = new Set<string>();
  const hits: SearchHit[] = [];
  for (const hit of pages.flatMap((page) => page.data)) {
    if (!seen.has(hit.node.id)) {
      seen.add(hit.node.id);
      hits.push(hit);
    }
  }
  return hits;
}

/**
 * SearchPage is a workspace's search (M8 design 4.12, 4.13; M8/P5 design
 * 4.3), of the address's q, in its notebook_id or in all the notebooks the
 * account sees: the hits whose names hold every term, then the pages whose
 * contents hold the others, a page of them at a time (search-hit.tsx),
 * More reading the next, or Keep looking after a page that found none as
 * the server looked through as many contents as it does at a time. Each
 * search is read by its entry in the tab's history, its key's: submitted
 * again, it is read again; gone back to, it shows as it was, its pages
 * read with it; it is not read again on events, a refocus nor a
 * connection. Only the search shown stays cached. The status line, there
 * from the start, says what was found, the title described by it: arrived
 * at from another page, the title has the focus; asked again here, the
 * line says it politely. More says what it added. Where to look replaces
 * the address, the focus staying on it. A notebook_id that is no id is not
 * read, and says, as one not found does, that the notebook is not found,
 * with Search the whole workspace; a q the server refuses says why. An
 * address without q says how to search.
 */
export const SearchPage = observer(function SearchPage() {
  const workspace = useWorkspace();
  const notebooks = useNotebooks(workspace);
  const searches = useSearch(workspace);
  const { locale } = useStore().preferences;
  const t = useT();
  const mounted = useMounted();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const ids = { status: useId(), scope: useId() };
  const [params, setParams] = useSearchParams();
  const { key: entry } = useLocation();
  const q = params.get("q") ?? "";
  const asked = q.trim() !== "";
  // An id in capitals is the id, as the server reads it, and as the notebooks list it (M8/P3B review B-2).
  const notebook = params.get("notebook_id")?.toLowerCase() || undefined;
  const readable = notebook === undefined || idPattern.test(notebook);
  const key = asked && readable ? ["search", workspace.id, q, notebook ?? "", entry] : null;
  const serial = key === null ? "" : unstable_serialize(key);
  const swr = useSWRConfig();
  const { mutate: reload } = swr;
  // A read reads as many pages as the list has, by the cache as it reads: what More added, kept as it is read again.
  const cached = () => (key === null ? undefined : swr.cache.get(unstable_serialize(key)));
  const { data, error, mutate } = useSWR(
    key,
    () =>
      readPages(
        (cursor) => searches.search(q, notebook, cursor),
        (cached()?.data as HitPage[] | undefined)?.length ?? 1
      ),
    {
      ...unchanging,
      // The workspace gone, or the account out of it, its shell finds it so; a notebook gone, its list does.
      onError: (refusal: unknown) => {
        if (refusal instanceof ApiError && refusal.code === "workspace.not_found") {
          void reload("workspaces").catch(() => undefined);
        } else if (refusal instanceof ApiError && refusal.code === "notebook.not_found") {
          void reload(["notebooks", workspace.id]).catch(() => undefined);
        }
      },
    }
  );
  useSWR(["notebooks", workspace.id], () => notebooks.load());
  // Only the search shown stays cached: back to it, it shows as it was; back to one before it, it is read again.
  // Dropped as useLeftRevision drops a version (version-page.tsx), with SWR's revalidation: a key no page reads is
  // not read, but its read on its way, or answered within SWR's deduping, is forgotten too; else a page back to it
  // would wait on that read, whose answer the drop made too old to keep, and show Loading for good (P5B review A-I1).
  useEffect(() => {
    void reload(
      (each) => Array.isArray(each) && each[0] === "search" && unstable_serialize(each) !== serial,
      undefined
    ).catch(() => undefined);
  }, [reload, serial]);
  // What More and Keep looking said last, of the search they read: another shown, it is gone.
  const [told, setTold] = useState({ of: "", text: "", again: false });
  // The link of the hit the focus falls to as More goes.
  const focused = useRef<HTMLAnchorElement>(null);
  const hits = hitsOf(data ?? []);
  const cursor = data?.at(-1)?.next_cursor ?? undefined;
  const more = useLoadMore(
    async () => {
      if (key === null) {
        return { added: [], whole: false, last: undefined };
      }
      const of = serial;
      const before = new Set(hits.map((hit) => hit.node.id));
      const read = await readNext(
        swr,
        key,
        cursor,
        (after) => searches.search(q, notebook, after),
        (page) => page.data.map((hit) => hit.node.id)
      );
      // A hit listed before, a rename between the pages, is none added: the focus does not go to it.
      const added = read.added.filter((id) => !before.has(id));
      if (mounted()) {
        const text = moreText(added.length, t, locale);
        setTold((said) => ({ of, text, again: said.of === of && said.text === text && !said.again }));
      }
      return { ...read, added };
    },
    () => focused.current,
    heading,
    serial
  );
  const title = asked ? t("search.titleOf", { q }) : t("search.title");
  useDocumentTitle(title, workspace.name);

  function choose(id: string): void {
    const next = new URLSearchParams(params);
    if (id === "") {
      next.delete("notebook_id");
    } else {
      next.set("notebook_id", id);
    }
    setParams(next, { replace: true });
  }

  const listed = notebooks.list;
  const nameOf = (id: string) => listed?.find((each) => each.id === id)?.name;
  const notebookTwins = twinsOf(listed ?? [], (each) => each.name);
  const named = hits.map((hit) => ({
    id: hit.node.id,
    name: hit.node.name,
    path: pathOf(hit, nameOf(hit.node.notebook_id) ?? ""),
  }));
  const nameTwins = twinsOf(named, (each) => each.name);
  const placeTwins = twinsOf(named, (each) => `${each.name}\n${each.path}`);
  const twinOf = (id: string): Twin => (placeTwins.has(id) ? "id" : nameTwins.has(id) ? "path" : "none");
  const said = data === undefined ? "" : statusText(data, hits.length, t, locale);
  const notFound =
    !readable || (error instanceof ApiError && error.code === "notebook.not_found")
      ? t("search.notebookNotFound")
      : undefined;
  const refused =
    error instanceof ApiError && error.code === "validation_failed"
      ? (fieldErrors(error, t).q ?? errorText(error, t))
      : undefined;
  const lastEmpty = data?.at(-1)?.data.length === 0;
  const now = Date.now();
  return (
    <section className="space-y-4">
      <h1
        ref={heading}
        tabIndex={-1}
        aria-describedby={said === "" ? undefined : ids.status}
        className="text-2xl font-semibold break-words outline-none"
      >
        {asked ? <Heading q={q} /> : title}
      </h1>
      {asked && (
        <div className="space-y-2">
          <Label htmlFor={ids.scope}>{t("search.scope")}</Label>
          <NativeSelect
            id={ids.scope}
            value={notebook ?? ""}
            onChange={(event) => choose(event.target.value)}
            className="block"
          >
            <option value="">{t("search.everyNotebook")}</option>
            {listed?.map((each) => (
              <option key={each.id} value={each.id}>
                {notebookTwins.has(each.id)
                  ? t("search.notebookTwin", { name: each.name, id: shortId(each.id) })
                  : each.name}
              </option>
            ))}
            {notebook !== undefined && nameOf(notebook) === undefined && (
              <option value={notebook}>{listed === undefined ? "…" : t("search.notebookUnknown")}</option>
            )}
          </NativeSelect>
        </div>
      )}
      {/* There before it says anything: a status that comes with its region is not always read. */}
      <output id={ids.status} className="block text-sm empty:sr-only">
        {said}
      </output>
      <output className="sr-only">{told.of === serial ? told.text + (told.again ? again : "") : ""}</output>
      {!asked ? (
        <p className="text-muted-foreground">{t("search.howTo")}</p>
      ) : notFound !== undefined ? (
        <NotFound text={notFound} q={q} left={() => heading.current?.focus()} />
      ) : refused !== undefined ? (
        <Alert>{refused}</Alert>
      ) : data === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : (
        hits.length > 0 && (
          <ul aria-label={t("search.results")} className="divide-y rounded-md border">
            {hits.map((hit) => (
              <SearchHitRow
                key={hit.node.id}
                hit={hit}
                notebook={nameOf(hit.node.notebook_id)}
                twin={twinOf(hit.node.id)}
                now={now}
                linkRef={hit.node.id === more.focusing ? focused : undefined}
              />
            ))}
          </ul>
        )
      )}
      {more.failure !== undefined && <Alert>{errorText(more.failure, t)}</Alert>}
      {data !== undefined && cursor !== undefined && (
        <LoadMoreButton more={more} label={lastEmpty ? t("search.keepLooking") : t("search.more")} />
      )}
    </section>
  );
});

/** Heading is the title of the search of q, q written in a bdi: what was typed, of any direction, does not turn it. */
function Heading({ q }: { q: string }) {
  const t = useT();
  const [before = "", after = ""] = t("search.heading", { q: "{q}" }).split("{q}");
  return (
    <>
      {before}
      <bdi>{q}</bdi>
      {after}
    </>
  );
}

/**
 * NotFound says that the notebook the search is in is not found (text),
 * with Search the whole workspace, the search of q in every notebook;
 * gone with the focus as the search shows, it gives it away (left).
 */
function NotFound({ text, q, left }: { text: string; q: string; left: () => void }) {
  const t = useT();
  const { slug } = useWorkspace();
  const link = useRef<HTMLAnchorElement>(null);
  useFocusLeaving(link, left);
  return (
    <div className="space-y-3">
      <Alert>{text}</Alert>
      <Link
        ref={link}
        to={`/${slug}/search?${new URLSearchParams({ q }).toString()}`}
        className="text-sm underline underline-offset-4"
      >
        {t("search.everywhere")}
      </Link>
    </div>
  );
}
