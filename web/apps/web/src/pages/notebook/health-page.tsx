import { observer } from "mobx-react-lite";
import { useEffect, useRef } from "react";
import { useLocation } from "react-router";
import useSWR from "swr";

import { useDocumentTitle } from "../../app/document-title";
import { NotLoaded } from "../../app/not-loaded";
import { Button } from "../../components/ui/button";
import { formatDateTime } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { usePageTree, useStore, useWiki } from "../../stores/context";
import { nodeTarget } from "../node-target";
import { useWorkspace } from "../workspace/workspace-layout";
import { healthNodeId } from "./health-item";
import { HealthRule, healthRuleOrder } from "./health-rule";
import { useNotebook } from "./notebook-layout";

// Health is computed only when opened or requested, independently of tree and notebook refreshes.
const unchanging = { revalidateIfStale: false, revalidateOnFocus: false, revalidateOnReconnect: false };

/** The notebook's health summary, available to every reader (M10/P3/S2-1). */
export const HealthPage = observer(function HealthPage() {
  const notebook = useNotebook();
  const workspace = useWorkspace();
  const store = useStore();
  const wiki = useWiki();
  const pages = usePageTree(notebook);
  const { locale } = store.preferences;
  const { key: entry } = useLocation();
  const t = useT();
  const title = t("health.title");
  useDocumentTitle(title, notebook.name);
  const { data, error, isValidating, mutate } = useSWR(
    ["lint", store.loginId, workspace.id, notebook.id, entry],
    () => wiki.summary(notebook.id),
    unchanging
  );
  const tree = useSWR(["pages", notebook.id], () => pages.load());
  const attempted = useRef(false);
  const missing =
    data?.rules.some((rule) =>
      rule.items.some((item) => {
        const id = healthNodeId(item);
        return id !== undefined && nodeTarget(pages, id) === undefined;
      })
    ) ?? false;
  const { mutate: readTree, error: treeError, isValidating: readingTree } = tree;
  const treeLoaded = pages.nodes !== undefined;
  // All findings in this opening share one attempt, even when several rules name the missing nodes.
  useEffect(() => {
    if (treeLoaded && missing && !attempted.current && !readingTree && treeError === undefined) {
      attempted.current = true;
      void readTree().catch(() => undefined);
    }
  }, [treeLoaded, missing, readingTree, treeError, readTree]);
  const applicable = healthRuleOrder
    .slice(0, data?.mode === "standard" ? 4 : 8)
    .flatMap((rule) => data?.rules.find((summary) => summary.rule === rule) ?? []);
  const found = applicable.filter((rule) => rule.count > 0);
  const clear = applicable.filter((rule) => rule.count === 0);
  const loadingPaths = !treeLoaded || readingTree;
  return (
    <section className="min-w-0 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold">{title}</h1>
        <Button
          variant="outline"
          aria-busy={isValidating || undefined}
          aria-disabled={isValidating || undefined}
          onClick={() => {
            if (!isValidating) void mutate().catch(() => undefined);
          }}
        >
          {t("health.recheck")}
        </Button>
      </div>
      <output aria-label={t("health.statusLabel")} className="sr-only">
        {data === undefined ? "" : t("health.completed", { count: applicable.length })}
      </output>
      {data === undefined ? (
        <NotLoaded error={error} retry={() => void mutate().catch(() => undefined)} />
      ) : (
        <>
          <p className="text-sm text-muted-foreground">
            {t("health.computedAt")} <time dateTime={data.computed_at}>{formatDateTime(data.computed_at, locale)}</time>
          </p>
          {data.stale_pages > 0 && (
            <p className="text-sm">
              {t("health.stale", { count: new Intl.NumberFormat(locale).format(data.stale_pages) })}
            </p>
          )}
          {treeError !== undefined && (
            <fieldset aria-label={t("health.paths")}>
              <NotLoaded error={treeError} retry={() => void readTree().catch(() => undefined)} />
            </fieldset>
          )}
          {found.map((summary) => (
            <HealthRule key={summary.rule} summary={summary} pages={pages} readingTree={loadingPaths} />
          ))}
          {clear.length > 0 && (
            <details className="space-y-3">
              <summary className="cursor-pointer font-medium">{t("health.clear", { count: clear.length })}</summary>
              {clear.map((summary) => (
                <HealthRule key={summary.rule} summary={summary} pages={pages} readingTree={loadingPaths} />
              ))}
            </details>
          )}
        </>
      )}
    </section>
  );
});
