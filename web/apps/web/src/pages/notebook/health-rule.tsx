import { observer } from "mobx-react-lite";
import { useEffect, useLayoutEffect, useRef } from "react";
import useSWR, { unstable_serialize } from "swr";

import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { useT, type PlainKey } from "../../i18n/i18n";
import { ApiError } from "../../services/api";
import type { LintItem, LintRule, LintRuleSummary } from "../../services/wiki.service";
import { useStore, useWiki } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { LoadMoreButton } from "../load-more";
import { HealthItem } from "./health-item";
import { useHealthLoadMore } from "./health-load-more";
import { healthCancelled, healthRequest, healthRetry, type HealthScope } from "./health-request";

export type HealthCard = {
  items: readonly LintItem[];
  cursor: string | null;
  error?: unknown;
  told: string;
  again: boolean;
};

export const healthRuleOrder: readonly LintRule[] = [
  "wanted",
  "orphans",
  "ambiguous",
  "invalid_frontmatter",
  "missing_type",
  "missing_summary",
  "undigested_sources",
  "invalid_sources",
];

const labels: Record<LintRule, { title: PlainKey; description: PlainKey }> = {
  wanted: { title: "health.wanted", description: "health.wantedDescription" },
  orphans: { title: "health.orphans", description: "health.orphansDescription" },
  ambiguous: { title: "health.ambiguous", description: "health.ambiguousDescription" },
  invalid_frontmatter: { title: "health.invalidFrontmatter", description: "health.invalidFrontmatterDescription" },
  missing_type: { title: "health.missingType", description: "health.missingTypeDescription" },
  missing_summary: { title: "health.missingSummary", description: "health.missingSummaryDescription" },
  undigested_sources: { title: "health.undigestedSources", description: "health.undigestedSourcesDescription" },
  invalid_sources: { title: "health.invalidSources", description: "health.invalidSourcesDescription" },
};

/** One applicable check, including the checks with no findings in the shared disclosure. */
export const HealthRule = observer(function HealthRule({
  summary,
  pages,
  readingTree,
  scope,
  card,
  mode,
  disabled,
  change,
  mismatch,
  paths,
}: {
  summary: LintRuleSummary;
  pages: PageTreeStore;
  readingTree: boolean;
  scope: HealthScope;
  card: HealthCard;
  mode: "standard" | "llm_wiki";
  disabled: boolean;
  change: (update: (was: HealthCard) => HealthCard) => Promise<HealthCard | undefined>;
  mismatch: () => void;
  paths: (items: readonly LintItem[]) => void;
}) {
  const t = useT();
  const { locale } = useStore().preferences;
  const wiki = useWiki();
  const heading = useRef<HTMLHeadingElement>(null);
  const elements = useRef(new Map<string, HTMLLIElement>());
  const identity = unstable_serialize([...scope.key, summary.rule]);
  const operation = useRef<{ identity: string; id: number; cursor: string; inFlight: boolean } | undefined>(undefined);
  const sequence = useRef(0);
  const enabled = useRef(!disabled);
  useLayoutEffect(() => {
    enabled.current = !disabled;
  }, [disabled]);
  if (operation.current?.identity !== identity) operation.current = undefined;
  const active = () => scope.current() && enabled.current && operation.current?.identity === identity;
  const request = useSWR(
    [...scope.key, "rule", summary.rule],
    async () => {
      const op = operation.current;
      if (op === undefined) throw new DOMException("No health page requested", "AbortError");
      op.inFlight = true;
      try {
        const page = await healthRequest(
          scope,
          (signal) => wiki.items(scope.notebookId, summary.rule, op.cursor, signal),
          () => enabled.current && operation.current === op
        );
        return { page, id: op.id, cursor: op.cursor };
      } finally {
        op.inFlight = false;
      }
    },
    {
      revalidateOnMount: false,
      revalidateIfStale: false,
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
      isPaused: () => !active(),
      onErrorRetry: (error, key, config, revalidate, options) => {
        const failed = operation.current;
        healthRetry(() => active() && operation.current === failed)(error, key, config, revalidate, options);
      },
      onError: (error) => {
        if (!active() || healthCancelled(error)) return;
        if (error instanceof ApiError && error.code === "notebook.not_llm_wiki") mismatch();
        void change((was) => ({ ...was, error }));
      },
      onSuccess: ({ page, id, cursor }) => {
        if (!active() || operation.current?.id !== id) return;
        if (page.mode !== mode) {
          mismatch();
          return;
        }
        void change((was) => {
          if (was.cursor !== cursor) return was;
          const seen = new Set(was.items.map((item) => item.item_key));
          const added = page.data.filter((item) => {
            if (seen.has(item.item_key)) return false;
            seen.add(item.item_key);
            return true;
          });
          const told = t("health.added", { count: new Intl.NumberFormat(locale).format(added.length) });
          return {
            items: [...was.items, ...added],
            cursor: page.next_cursor,
            error: undefined,
            told,
            again: was.told === told && !was.again,
          };
        });
      },
    }
  );
  useEffect(() => {
    paths(card.items);
  }, [paths, card.items, readingTree, pages.nodes]);
  const more = useHealthLoadMore(
    async () => {
      if (card.cursor === null || operation.current?.inFlight || disabled || !scope.current()) {
        return { added: [], whole: false, last: undefined };
      }
      const before = new Set(card.items.map((item) => item.item_key));
      const op = { identity, id: ++sequence.current, cursor: card.cursor, inFlight: false };
      operation.current = op;
      const result = await request.mutate();
      if (
        !scope.current() ||
        !enabled.current ||
        operation.current !== op ||
        result?.id !== op.id ||
        result.page.mode !== mode
      ) {
        throw new DOMException("Health page not accepted", "AbortError");
      }
      const added = result.page.data.filter((item) => !before.has(item.item_key)).map((item) => item.item_key);
      return { added, whole: result.page.next_cursor === null, last: added.at(-1) ?? card.items.at(-1)?.item_key };
    },
    () => scope.current() && enabled.current,
    (id) => elements.current.get(id),
    heading,
    identity,
    disabled,
    request.isValidating
  );
  const text = labels[summary.rule];
  const title = t(text.title);
  const count = new Intl.NumberFormat(locale).format(summary.count) + (summary.count_capped ? "+" : "");
  return (
    <section
      id={summary.rule}
      aria-labelledby={`${summary.rule}-heading`}
      className="min-w-0 space-y-2 rounded-md border p-4"
    >
      <h2 ref={heading} id={`${summary.rule}-heading`} tabIndex={-1} className="font-semibold outline-none">
        {title}
      </h2>
      <p className="text-sm text-muted-foreground">{t(text.description)}</p>
      <p className="text-sm">
        {summary.count === 1 && !summary.count_capped ? t("health.oneIssue") : t("health.count", { count })}
      </p>
      {card.items.length > 0 && (
        <ul aria-label={title} className="divide-y">
          {card.items.map((item) => (
            <HealthItem
              key={item.item_key}
              item={item}
              pages={pages}
              readingTree={readingTree}
              itemRef={(element) => {
                if (element === null) elements.current.delete(item.item_key);
                else elements.current.set(item.item_key, element);
              }}
            />
          ))}
        </ul>
      )}
      <output className="sr-only">{card.told + (card.again ? String.fromCharCode(0xa0) : "")}</output>
      {card.error !== undefined && <Alert>{errorText(card.error, t)}</Alert>}
      {card.cursor !== null && (
        <fieldset disabled={disabled}>
          <LoadMoreButton more={more} label={t(card.error === undefined ? "health.more" : "status.retry")} />
        </fieldset>
      )}
    </section>
  );
});
