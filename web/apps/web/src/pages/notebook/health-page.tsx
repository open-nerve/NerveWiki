import { observer } from "mobx-react-lite";
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type RefObject } from "react";
import { useLocation } from "react-router";
import useSWR, { SWRConfig, useSWRConfig, type Unloader } from "swr";

import { useDocumentTitle } from "../../app/document-title";
import { NotLoaded } from "../../app/not-loaded";
import { Button } from "../../components/ui/button";
import { formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { LintItem, LintRule, LintSummary } from "../../services/wiki.service";
import { usePageTree, useStore, useWiki } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { nodeTarget } from "../node-target";
import { watchReader } from "../page/readers-input";
import { useWorkspace } from "../workspace/workspace-layout";
import { healthNodeId } from "./health-item";
import { healthCancelled, healthRequest, healthRetry, type HealthScope } from "./health-request";
import { HealthRule, healthRuleOrder, type HealthCard } from "./health-rule";
import { useNotebook } from "./notebook-layout";

type HealthView = {
  summary?: LintSummary;
  cards: Partial<Record<LintRule, HealthCard>>;
  checking: boolean;
  error?: unknown;
  mismatch: boolean;
  accepted: number;
  announcements: number;
};
type EntryControl = {
  alive: boolean;
  generation: number;
  restarting: boolean;
  controllers: Set<AbortController>;
  unload?: Unloader;
  restart?: () => void;
};
type Entry = { nonce: number; control: RefObject<EntryControl> };
type TreeRead = { error: unknown; reading: boolean; retry: () => Promise<unknown> };
let nextEntry = 0;
const unchanging = { revalidateIfStale: false, revalidateOnFocus: false, revalidateOnReconnect: false };

/** The tree stays in the public provider shared with the notebook's left column. */
export const HealthPage = observer(function HealthPage() {
  const notebook = useNotebook();
  const workspace = useWorkspace();
  const pages = usePageTree(notebook);
  const location = useLocation();
  const tree = useSWR(["pages", notebook.id], () => pages.load());
  return (
    <HealthEntry
      key={location.key}
      notebook={notebook}
      workspaceId={workspace.id}
      locationKey={location.key}
      hash={location.hash}
      pages={pages}
      tree={{ error: tree.error, reading: tree.isValidating, retry: tree.mutate }}
    />
  );
});

/** Its layout cleanup precedes SWRConfig's cleanup, while the scoped unloader is still registered. */
function HealthEntry(props: {
  notebook: Notebook;
  workspaceId: string;
  locationKey: string;
  hash: string;
  pages: PageTreeStore;
  tree: TreeRead;
}) {
  const [nonce] = useState(() => ++nextEntry);
  const control = useRef<EntryControl>({ alive: true, generation: 0, restarting: false, controllers: new Set() });
  const entry = useMemo(() => ({ nonce, control }), [nonce]);
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
      <HealthResults {...props} entry={entry} />
    </SWRConfig>
  );
}

const HealthResults = observer(function HealthResults({
  notebook,
  workspaceId,
  locationKey,
  hash,
  pages,
  tree,
  entry,
}: {
  notebook: Notebook;
  workspaceId: string;
  locationKey: string;
  hash: string;
  pages: PageTreeStore;
  tree: TreeRead;
  entry: Entry;
}) {
  const store = useStore();
  const wiki = useWiki();
  const t = useT();
  const { locale } = store.preferences;
  const title = t("health.title");
  useDocumentTitle(title, notebook.name);
  const [generation, setGeneration] = useState(0);
  const { mutate, unload } = useSWRConfig();
  const scope = useMemo<HealthScope>(
    () => ({
      key: ["lint", store.loginId, workspaceId, notebook.id, locationKey, entry.nonce, generation],
      notebookId: notebook.id,
      controllers: {
        add: (controller) => entry.control.current.controllers.add(controller),
        delete: (controller) => entry.control.current.controllers.delete(controller),
      },
      current: () =>
        entry.control.current.alive &&
        entry.control.current.generation === generation &&
        store.auth.state.loginId === store.loginId,
    }),
    [store, workspaceId, notebook.id, locationKey, entry, generation]
  );
  const initial = useMemo<HealthView>(
    () => ({ cards: {}, checking: true, mismatch: false, accepted: -1, announcements: 0 }),
    []
  );
  const viewKey = [...scope.key, "view"];
  const { data: displayed } = useSWR<HealthView>(viewKey, null, { fallbackData: initial });
  const view = displayed ?? initial;
  function change(update: (was: HealthView) => HealthView): Promise<HealthView | undefined> {
    if (!scope.current()) return Promise.resolve(undefined);
    return mutate<HealthView>(viewKey, (was) => (scope.current() ? update(was ?? initial) : was), {
      revalidate: false,
    });
  }
  const { mutate: readSummary } = useSWR(
    scope.key,
    () => healthRequest(scope, (signal) => wiki.summary(notebook.id, signal)),
    {
      ...unchanging,
      onErrorRetry: healthRetry(scope.current),
      onError: (error) => {
        if (!healthCancelled(error)) void change((was) => ({ ...was, checking: false, error }));
      },
      onSuccess: (data) => {
        void change((was) => {
          if (data.mode !== notebook.mode && was.summary !== undefined)
            return { ...was, checking: false, error: undefined, mismatch: true };
          const cards: HealthView["cards"] = {};
          for (const rule of data.rules)
            cards[rule.rule] = { items: rule.items, cursor: rule.next_cursor, told: "", again: false };
          return {
            summary: data,
            cards,
            checking: false,
            mismatch: data.mode !== notebook.mode,
            accepted: generation,
            announcements: was.announcements + 1,
          };
        });
      },
    }
  );
  useLayoutEffect(() => {
    entry.control.current.unload = unload;
    entry.control.current.restart = () => {
      void readSummary().catch(() => undefined);
    };
  }, [entry, unload, readSummary]);

  function recheck() {
    const owner = entry.control.current;
    if (!owner.alive) return;
    const next = ++owner.generation;
    for (const controller of owner.controllers) controller.abort();
    owner.controllers.clear();
    // Only this entry's visible data is carried forward. Requests always get fresh generation keys.
    const nextKey = ["lint", store.loginId, workspaceId, notebook.id, locationKey, entry.nonce, next, "view"];
    void mutate<HealthView>(nextKey, { ...view, checking: true, error: undefined }, { revalidate: false });
    setGeneration(next);
  }
  const attempted = useRef(false);
  function paths(items: readonly LintItem[]) {
    if (!scope.current() || attempted.current || pages.nodes === undefined || tree.reading || tree.error !== undefined)
      return;
    if (
      items.some((item) => {
        const id = healthNodeId(item);
        return id !== undefined && nodeTarget(pages, id) === undefined;
      })
    ) {
      attempted.current = true;
      void tree.retry().catch(() => undefined);
    }
  }
  const details = useRef<HTMLDetailsElement>(null);
  const reader = useRef<ReturnType<typeof watchReader> | undefined>(undefined);
  const anchored = useRef(false);
  useLayoutEffect(() => {
    reader.current = watchReader();
    return () => reader.current?.end();
  }, []);
  useEffect(() => {
    if (anchored.current || view.summary === undefined || hash !== "#undigested_sources" || !scope.current()) return;
    anchored.current = true;
    if (details.current !== null && details.current.querySelector("#undigested_sources-heading") !== null)
      details.current.open = true;
    if (!reader.current?.acted()) document.getElementById("undigested_sources-heading")?.focus({ preventScroll: true });
  }, [view.summary, hash, scope]);

  const data = view.summary;
  const mismatch = view.mismatch || (data !== undefined && data.mode !== notebook.mode);
  const applicable = healthRuleOrder
    .slice(0, data?.mode === "standard" ? 4 : 8)
    .flatMap((rule) => data?.rules.find((found) => found.rule === rule) ?? []);
  const found = applicable.filter((rule) => rule.count > 0);
  const clear = applicable.filter((rule) => rule.count === 0);
  function card(rule: (typeof applicable)[number]) {
    const state = view.cards[rule.rule];
    if (state === undefined) return null;
    return (
      <HealthRule
        key={`${rule.rule}:${view.accepted}`}
        summary={rule}
        card={state}
        scope={scope}
        mode={notebook.mode}
        pages={pages}
        readingTree={pages.nodes === undefined || tree.reading}
        disabled={view.checking || mismatch}
        paths={paths}
        mismatch={() => {
          void change((was) => ({ ...was, mismatch: true }));
        }}
        change={async (update) => {
          const result = await change((was) => {
            const current = was.cards[rule.rule];
            return current === undefined ? was : { ...was, cards: { ...was.cards, [rule.rule]: update(current) } };
          });
          return result?.cards[rule.rule];
        }}
      />
    );
  }
  return (
    <section className="min-w-0 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold">{title}</h1>
        <Button variant="outline" aria-busy={view.checking || undefined} onClick={recheck}>
          {t("health.recheck")}
        </Button>
      </div>
      <output aria-label={t("health.statusLabel")} className="sr-only">
        {data === undefined
          ? ""
          : t("health.completed", { count: applicable.length }) +
            (view.announcements % 2 === 0 ? String.fromCharCode(0xa0) : "")}
      </output>
      {data === undefined ? (
        <NotLoaded error={view.error} retry={recheck} />
      ) : (
        <>
          <p className="text-sm text-muted-foreground">
            {t("health.computedAt")} <time dateTime={data.computed_at}>{formatDateTime(data.computed_at, locale)}</time>
          </p>
          {view.checking && <p className="text-sm">{t("health.checking")}</p>}
          {view.error !== undefined && <NotLoaded error={view.error} retry={recheck} />}
          {mismatch && <p className="text-sm">{t("health.modeChanged")}</p>}
          {data.stale_pages > 0 && (
            <p className="text-sm">
              {t("health.stale", { count: new Intl.NumberFormat(locale).format(data.stale_pages) })}
            </p>
          )}
          {tree.error !== undefined && (
            <fieldset aria-label={t("health.paths")}>
              <NotLoaded error={tree.error} retry={() => void tree.retry().catch(() => undefined)} />
            </fieldset>
          )}
          {found.map(card)}
          {clear.length > 0 && (
            <details ref={details} className="space-y-3">
              <summary className="cursor-pointer font-medium">{t("health.clear", { count: clear.length })}</summary>
              {clear.map(card)}
            </details>
          )}
          {found.some((rule) => rule.next_cursor !== null) && (
            <p className="text-sm text-muted-foreground">{t("health.paginationNote")}</p>
          )}
        </>
      )}
    </section>
  );
});
