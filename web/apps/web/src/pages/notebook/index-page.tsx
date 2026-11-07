import { observer } from "mobx-react-lite";
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";
import { Link, useLocation, useSearchParams } from "react-router";
import useSWR, { SWRConfig, useSWRConfig, type Unloader } from "swr";
import useSWRInfinite from "swr/infinite";

import { useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { onErrorRetry } from "../../app/retry";
import { Alert } from "../../components/ui/alert";
import { useT } from "../../i18n/i18n";
import type { IndexFilter, NotebookIndex } from "../../services/linking.service";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree, useStore } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { LoadMoreButton, useLoadMore } from "../load-more";
import { nodeTarget } from "../node-target";
import { useWorkspace } from "../workspace/workspace-layout";
import { IndexFilters } from "./index-filters";
import { IndexRows } from "./index-rows";
import { useNotebook } from "./notebook-layout";

type Owner = {
  alive: boolean;
  restarting: boolean;
  controllers: Set<AbortController>;
  unload?: Unloader;
  restart?: () => void;
};
type PageKey = readonly ["notebook-index", string | undefined, string, string, string, string, string, string | null];
const unchanging = { revalidateIfStale: false, revalidateOnFocus: false, revalidateOnReconnect: false };

/** Directory filters stay mounted while only their opening's result scope is replaced. */
export const IndexPage = observer(function IndexPage() {
  const notebook = useNotebook();
  const workspace = useWorkspace();
  const store = useStore();
  const pages = usePageTree(notebook);
  const location = useLocation();
  const [params, setParams] = useSearchParams();
  const filter = { type: params.get("type") ?? "", tag: params.get("tag") ?? "" };
  const identity = JSON.stringify([store.loginId, workspace.id, notebook.id, location.key, filter.type, filter.tag]);
  const [metadata, setMetadata] = useState<{ identity: string; first: NotebookIndex }>();
  const report = useCallback((first: NotebookIndex) => setMetadata({ identity, first }), [identity]);
  const current = metadata?.identity === identity ? metadata.first : undefined;
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const t = useT();
  useDocumentTitle(t("index.title"), notebook.name);
  // Shared with the sidebar; disposing the directory must not clear this cache.
  const tree = useSWR(["pages", notebook.id], () => pages.load());
  function choose(next: IndexFilter) {
    const query = new URLSearchParams();
    if (next.type !== "") query.set("type", next.type);
    if (next.tag !== "") query.set("tag", next.tag);
    void setParams(query, { replace: true, preventScrollReset: true });
  }
  return (
    <section className="space-y-4">
      <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold outline-none">
        {t("index.title")}
      </h1>
      <IndexFilters filter={filter} catalogue={current?.types ?? []} choose={choose} />
      {current !== undefined && !current.types_complete && (
        <p className="text-muted-foreground">{t("index.partialTypes")}</p>
      )}
      {current !== undefined && current.unfilterable_type_pages > 0 && (
        <p className="text-muted-foreground">
          {t("index.longTypes", { count: current.unfilterable_type_pages.toLocaleString(store.preferences.locale) })}
        </p>
      )}
      {tree.error !== undefined && <NotLoaded error={tree.error} retry={() => void tree.mutate()} />}
      <IndexEntry
        key={identity}
        notebook={notebook}
        workspaceId={workspace.id}
        locationKey={location.key}
        filter={filter}
        pages={pages}
        heading={heading}
        report={report}
        reopen={location.pathname + location.search}
      />
    </section>
  );
});

type EntryProps = {
  notebook: Notebook;
  workspaceId: string;
  locationKey: string;
  filter: IndexFilter;
  pages: PageTreeStore;
  heading: RefObject<HTMLHeadingElement | null>;
  report: (first: NotebookIndex) => void;
  reopen: string;
};

/** A fresh local cache per opening/filter, including Back/Forward; layout retirement also covers ignored aborts. */
function IndexEntry(props: EntryProps) {
  const control = useRef<Owner>({ alive: true, restarting: false, controllers: new Set() });
  const [config] = useState(() => ({ provider: () => new Map() }));
  useLayoutEffect(() => {
    const owner = control.current;
    owner.alive = true;
    if (owner.restarting) {
      owner.restarting = false;
      owner.restart?.();
    }
    return () => {
      owner.alive = false;
      owner.restarting = true;
      for (const controller of owner.controllers) controller.abort();
      owner.controllers.clear();
      owner.unload?.({ revalidate: false });
    };
  }, []);
  return (
    <SWRConfig value={config}>
      <IndexResults {...props} control={control} />
    </SWRConfig>
  );
}

const IndexResults = observer(function IndexResults({
  notebook,
  workspaceId,
  locationKey,
  filter,
  pages,
  heading,
  report,
  reopen,
  control,
}: EntryProps & { control: RefObject<Owner> }) {
  const store = useStore();
  const { slug } = useWorkspace();
  const t = useT();
  const { unload } = useSWRConfig();
  const current = () => control.current.alive && store.auth.state.loginId === store.loginId;
  const getKey = (page: number, previous: NotebookIndex | null): PageKey | null => {
    if (page > 0 && (previous === null || previous.next_cursor === null)) return null;
    return [
      "notebook-index",
      store.loginId,
      workspaceId,
      notebook.id,
      locationKey,
      filter.type,
      filter.tag,
      previous?.next_cursor ?? null,
    ];
  };
  const { data, error, isValidating, mutate, setSize } = useSWRInfinite<NotebookIndex, unknown, typeof getKey>(
    getKey,
    async (key: PageKey) => {
      const controller = new AbortController();
      const active = () => current() && !controller.signal.aborted;
      control.current.controllers.add(controller);
      try {
        if (!active()) throw new DOMException("Directory request ended", "AbortError");
        const result = await pages.index(filter, key[7] ?? undefined, controller.signal);
        if (!active()) throw new DOMException("Directory request ended", "AbortError");
        return result;
      } catch (cause) {
        if (!active()) throw new DOMException("Directory request ended", "AbortError");
        throw cause;
      } finally {
        control.current.controllers.delete(controller);
      }
    },
    {
      ...unchanging,
      revalidateFirstPage: false,
      onErrorRetry: (cause, key, config, revalidate, options) => {
        if (!current() || (cause instanceof DOMException && cause.name === "AbortError")) return;
        // The normal retry policy, including ordinary server_busy; only a retired generation is suppressed.
        onErrorRetry?.(
          cause,
          key,
          config,
          (next) => {
            if (current()) void revalidate(next);
          },
          options
        );
      },
    }
  );
  useLayoutEffect(() => {
    control.current.unload = unload;
    control.current.restart = () => {
      void mutate().catch(() => undefined);
    };
  }, [control, unload, mutate]);
  const first = data?.[0];
  useEffect(() => {
    if (first !== undefined && control.current.alive && store.auth.state.loginId === store.loginId) report(first);
  }, [first, report, control, store]);
  // A later page with another mode must not mix into the retained result. Reopen is explicit.
  const crossed = data?.findIndex((page) => page.mode !== first?.mode) ?? -1;
  const read = crossed < 0 ? data : data?.slice(0, crossed);
  const mismatch = first !== undefined && (first.mode !== notebook.mode || crossed >= 0);
  const items = read?.flatMap((page) => page.data) ?? [];
  const cursor = read?.at(-1)?.next_cursor;
  const links = useRef(new Map<string, HTMLAnchorElement>());
  // setSize answers with the pages it has, before the next is read: More waits for the render that shows it.
  const waiting = useRef<{ before: number; settle: (pages: NotebookIndex[] | undefined) => void }>(undefined);
  useEffect(() => {
    const wait = waiting.current;
    if (wait === undefined) return;
    const arrived = data !== undefined && data.length > wait.before;
    if (arrived || (error !== undefined && !isValidating)) {
      waiting.current = undefined;
      wait.settle(arrived ? data : undefined);
    }
  }, [data, error, isValidating]);
  useEffect(
    () => () => {
      waiting.current?.settle(undefined);
      waiting.current = undefined;
    },
    []
  );
  const more = useLoadMore(
    async () => {
      if (!current() || mismatch || cursor == null) return { added: [], whole: false, last: undefined };
      const before = data?.length ?? 0;
      const shown = new Promise<NotebookIndex[] | undefined>((settle) => {
        waiting.current = { before, settle };
      });
      await setSize(before + 1);
      const next = await shown;
      const appended = current() && next !== undefined && next.length > before && next.at(-1)?.mode === first?.mode;
      const last = appended ? next.at(-1) : undefined;
      return {
        added: last?.data.map((item) => item.page_id) ?? [],
        whole: last?.next_cursor === null,
        last: last?.data.at(-1)?.page_id,
      };
    },
    (id) => links.current.get(id),
    heading,
    JSON.stringify([locationKey, filter.type, filter.tag])
  );
  const area = first?.source_area;
  const root = area == null ? undefined : nodeTarget(pages, area.root_id);
  const home = `/${slug}/notebooks/${notebook.id}`;
  return (
    <>
      <output aria-label={t("index.results")} className="sr-only">
        {isValidating
          ? t("index.loading")
          : first === undefined
            ? ""
            : t("index.loaded", { count: items.length.toLocaleString(store.preferences.locale) })}
      </output>
      {mismatch && (
        <Alert>
          {t("index.modeChanged")}{" "}
          <Link to={reopen} className="underline">
            {t("index.reopen")}
          </Link>
        </Alert>
      )}
      {area != null && (
        <p>
          <Link to={`${home}/health#undigested_sources`} className="underline underline-offset-4">
            {t("index.sources", {
              path: root?.path ?? t("index.sourceArea"),
              count: area.sources.toLocaleString(store.preferences.locale),
            })}
          </Link>
        </p>
      )}
      {first === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : items.length === 0 ? (
        <p className="text-muted-foreground">{t("index.empty")}</p>
      ) : (
        <IndexRows
          items={items}
          pages={pages}
          home={home}
          linkRef={(id, node) => {
            if (node === null) links.current.delete(id);
            else links.current.set(id, node);
          }}
        />
      )}
      {first !== undefined && error !== undefined && <Alert>{errorText(error, t)}</Alert>}
      {more.failure !== undefined && <Alert>{errorText(more.failure, t)}</Alert>}
      {cursor != null && !mismatch && <LoadMoreButton more={more} label={t("index.more")} />}
    </>
  );
});
