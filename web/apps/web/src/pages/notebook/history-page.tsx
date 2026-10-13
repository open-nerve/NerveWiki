import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";

import { useDocumentTitle } from "../../app/document-title";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { idPattern } from "../../lib/ids";
import { readPages } from "../../lib/read-pages";
import { ApiError } from "../../services/api";
import type { ChangesetPage, LogFilter } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import { LoadMoreButton, useLoadMore } from "../load-more";
import { useWorkspace } from "../workspace/workspace-layout";
import { twinsIn } from "./changeset-text";
import { HistoryFilters } from "./history-filters";
import { HistoryRow } from "./history-row";
import { useNotebook } from "./notebook-layout";

/** filterOf is the filter of a history's address: ?actor=&client=&page=, "" for each left out. */
function filterOf(params: URLSearchParams): LogFilter {
  return { actor: params.get("actor") ?? "", client: params.get("client") ?? "", page: params.get("page") ?? "" };
}

/** paramsOf is the query of a history's address that has filter: those of it that narrow it alone. */
function paramsOf({ actor, client, page }: LogFilter): URLSearchParams {
  const params = new URLSearchParams();
  for (const [name, value] of Object.entries({ actor, client, page })) {
    if (value !== "") {
      params.set(name, value);
    }
  }
  return params;
}

/** A space that does not break: said after a notice said again, so that a screen reader reads it again. */
const again = String.fromCharCode(0xa0);

/**
 * HistoryPage is a notebook's history (M8 design 4.7, 4.13; M8/P3 design
 * 4.2): the changesets its writes made, newer first, a page of them at a
 * time (its rows, history-row.tsx), More reading the next; read again (an
 * event, a refocus, a connection) as many pages as were read, from the
 * first (readPages). Anyone who reads the notebook sees it. Its filters
 * are its address's (history-filters.tsx), each choice replacing it
 * without a step of the history; what a choice lists is said politely,
 * the focus left on what chose it. An address whose account or page is no
 * id is not read, and says so as a client the server does not take does,
 * or a page not of the notebook: with Clear filters.
 */
export const HistoryPage = observer(function HistoryPage() {
  const workspace = useWorkspace();
  const notebook = useNotebook();
  const pages = usePageTree(notebook);
  const { locale } = useStore().preferences;
  const t = useT();
  const heading = useRef<HTMLHeadingElement>(null);
  const title = t("history.title");
  useDocumentTitle(title, notebook.name);
  const [params, setParams] = useSearchParams();
  const filter = filterOf(params);
  // An account or a page in the address that is no id names none: not read, as the server would refuse it.
  const readable = [filter.actor, filter.page].every((id) => id === "" || idPattern.test(id));
  const key = readable ? ["changesets", notebook.id, filter.actor, filter.client, filter.page] : null;
  const { cache } = useSWRConfig();
  // A read reads as many pages as the list has, by the cache as it reads: what More added, the page come back to.
  const cached = () => (key === null ? undefined : cache.get(unstable_serialize(key)));
  const { data, error, mutate } = useSWR(key, () =>
    readPages(
      (cursor) => pages.changesets(filter, cursor),
      (cached()?.data as ChangesetPage[] | undefined)?.length ?? 1
    )
  );
  // The filters chosen whose list is still to be said, and what was said last.
  const [asked, setAsked] = useState<string>();
  const [said, setSaid] = useState({ text: "", again: false });
  // The link of the changeset the focus falls to as More goes.
  const focused = useRef<HTMLAnchorElement>(null);
  const cursor = data?.at(-1)?.next_cursor ?? undefined;
  const changesets = data?.flatMap((read) => read.data) ?? [];
  const serial = key === null ? "" : unstable_serialize(key);
  useEffect(() => {
    if (asked !== serial || data === undefined) {
      return;
    }
    const count = new Intl.NumberFormat(locale).format(data.reduce((sum, read) => sum + read.data.length, 0));
    const text =
      count === "0"
        ? t("history.empty")
        : (data.at(-1)?.next_cursor ?? null) === null
          ? t("history.listed", { count })
          : t("history.listedMore", { count });
    setSaid((before) => ({ text, again: before.text === text && !before.again }));
    setAsked(undefined);
  }, [asked, serial, data, locale, t]);
  const more = useLoadMore(
    async () => {
      const next = await pages.changesets(filter, cursor);
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
        added: added ? next.data.map((changeset) => changeset.id) : [],
        whole: added && next.next_cursor === null,
        last: read?.at(-1)?.data.at(-1)?.id,
      };
    },
    () => focused.current,
    heading
  );

  function choose(next: LogFilter): void {
    const nextKey = ["changesets", notebook.id, next.actor, next.client, next.page];
    setAsked(unstable_serialize(nextKey));
    setParams(paramsOf(next), { replace: true });
  }

  const home = `/${workspace.slug}/notebooks/${notebook.id}`;
  const href = (id: string) => `${home}/history/${id}`;
  const twins = twinsIn(
    changesets.flatMap((changeset) => (changeset.reverts === null ? [changeset] : [changeset, changeset.reverts])),
    locale
  );
  // What keeps the list from being read: a filter that is no id, or one the server does not take.
  const refused =
    !readable || (error instanceof ApiError && error.code === "validation_failed")
      ? t("history.badFilters")
      : error instanceof ApiError && error.code === "page.not_found"
        ? t("history.pageNotFound")
        : undefined;
  const now = Date.now();
  return (
    <section className="space-y-4">
      <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold outline-none">
        {title}
      </h1>
      <HistoryFilters
        notebook={notebook}
        filter={filter}
        read={changesets}
        choose={choose}
        left={() => heading.current?.focus()}
      />
      <output className="sr-only">{said.text + (said.again ? again : "")}</output>
      {refused !== undefined ? (
        <Refused
          text={refused}
          clear={() => choose({ actor: "", client: "", page: "" })}
          left={() => heading.current?.focus()}
        />
      ) : data === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : changesets.length === 0 ? (
        <p className="text-muted-foreground">{t("history.empty")}</p>
      ) : (
        <ul aria-label={t("history.list")} className="divide-y rounded-md border">
          {changesets.map((changeset) => (
            <HistoryRow
              key={changeset.id}
              changeset={changeset}
              href={href}
              twin={twins.has(changeset.id)}
              revertsTwin={changeset.reverts !== null && twins.has(changeset.reverts.id)}
              only={
                changeset.created_by.user_id === filter.actor
                  ? undefined
                  : () => choose({ ...filter, actor: changeset.created_by.user_id })
              }
              left={() => heading.current?.focus()}
              linkRef={changeset.id === more.focusing ? focused : undefined}
              now={now}
            />
          ))}
        </ul>
      )}
      {more.failure !== undefined && <Alert>{errorText(more.failure, t)}</Alert>}
      {refused === undefined && cursor !== undefined && <LoadMoreButton more={more} label={t("history.more")} />}
    </section>
  );
});

/**
 * Refused says why the filters keep the history from being read (text),
 * with Clear filters (clear), which, gone with the focus as the list
 * shows, gives it away (left).
 */
function Refused({ text, clear, left }: { text: string; clear: () => void; left: () => void }) {
  const t = useT();
  const button = useRef<HTMLButtonElement>(null);
  useFocusLeaving(button, left);
  return (
    <div className="space-y-3">
      <Alert>{text}</Alert>
      <Button ref={button} variant="outline" onClick={clear}>
        {t("history.clearFilters")}
      </Button>
    </div>
  );
}
