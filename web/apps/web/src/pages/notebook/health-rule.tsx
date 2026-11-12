import { observer } from "mobx-react-lite";
import { useEffect, useLayoutEffect, useRef } from "react";
import useSWR, { unstable_serialize } from "swr";

import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { useT, type PlainKey, type Translate } from "../../i18n/i18n";
import type { Locale } from "../../i18n/locale";
import { ApiError } from "../../services/api";
import type { UpgradeReportItem } from "../../services/upgrade.service";
import type { LintItem, LintPage, LintRule, LintRuleSummary } from "../../services/wiki.service";
import { useStore } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { LoadMoreButton } from "../load-more";
import { HealthItem } from "./health-item";
import { useHealthLoadMore } from "./health-load-more";
import { healthCancelled, healthRequest, healthRetry, type HealthScope } from "./health-request";

/** An item a rule's card lists: lint's, or a stored report's, which has no cursor (M11/P3 design 4.3). */
export type HealthRuleItem = LintItem | UpgradeReportItem;

export type HealthCard = {
  items: readonly HealthRuleItem[];
  /** The next page's cursor; undefined: the next read starts from the first page; null: there is no more. */
  cursor: string | null | undefined;
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

/** issueCount is what a rule's check found: one issue, or how many, with a + once capped. */
function issueCount(
  t: Translate,
  locale: Locale,
  { count, count_capped: capped }: Pick<LintRuleSummary, "count" | "count_capped">
): string {
  return count === 1 && !capped
    ? t("health.oneIssue")
    : t("health.count", { count: new Intl.NumberFormat(locale).format(count) + (capped ? "+" : "") });
}

/**
 * One applicable check, including the checks with no findings in the shared
 * disclosure: the health page's, an upgrade's check's (M11/P2 design 4.3),
 * or a rule of the report an upgrade's submission kept (M11/P3 design 4.3),
 * whose caller says how a page of its items is read, which refusal says
 * the scope it was read in is gone, its heading's level, and whether the
 * tree its items' paths come from failed to read. A card whose cursor is
 * undefined reads its next page from the first: a stored report's first
 * items have none; those read again are not listed twice.
 */
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
  read,
  moved,
  level = 2,
  treeFailed,
}: {
  summary: Pick<LintRuleSummary, "rule" | "count" | "count_capped">;
  pages: PageTreeStore;
  readingTree: boolean;
  scope: HealthScope;
  card: HealthCard;
  mode: "standard" | "llm_wiki";
  disabled: boolean;
  change: (update: (was: HealthCard) => HealthCard) => Promise<HealthCard | undefined>;
  mismatch: () => void;
  paths: (items: readonly HealthRuleItem[]) => void;
  /** Reads a page of the rule's items after cursor; from the first, without one. */
  read: (rule: LintRule, cursor: string | undefined, signal: AbortSignal) => Promise<LintPage>;
  /** The code that says the scope the summary was read in is gone. */
  moved: string;
  /** The card's heading's level: 2 on a page of its own, 3 under a section's. */
  level?: 2 | 3;
  /** Whether the tree failed to read: an item's path is then left empty, not said to be unavailable. */
  treeFailed?: boolean;
}) {
  const t = useT();
  const { locale } = useStore().preferences;
  const heading = useRef<HTMLHeadingElement>(null);
  const elements = useRef(new Map<string, HTMLLIElement>());
  const identity = unstable_serialize([...scope.key, summary.rule]);
  const operation = useRef<{ identity: string; id: number; cursor: string | undefined; inFlight: boolean } | undefined>(
    undefined
  );
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
          (signal) => read(summary.rule, op.cursor, signal),
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
        if (error instanceof ApiError && error.code === moved) mismatch();
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
  const Heading = level === 3 ? "h3" : "h2";
  return (
    <section
      id={summary.rule}
      aria-labelledby={`${summary.rule}-heading`}
      className="min-w-0 space-y-2 rounded-md border p-4"
    >
      <Heading ref={heading} id={`${summary.rule}-heading`} tabIndex={-1} className="font-semibold outline-none">
        {title}
      </Heading>
      <p className="text-sm text-muted-foreground">
        {t(summary.rule === "orphans" && mode === "llm_wiki" ? "health.orphansDescriptionWiki" : text.description)}
      </p>
      <p className="text-sm">{issueCount(t, locale, summary)}</p>
      {card.items.length > 0 && (
        <ul aria-label={title} className="divide-y">
          {card.items.map((item) => (
            <HealthItem
              key={item.item_key}
              item={item}
              pages={pages}
              readingTree={readingTree}
              treeFailed={treeFailed}
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
