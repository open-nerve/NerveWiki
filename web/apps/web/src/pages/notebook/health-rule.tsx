import { observer } from "mobx-react-lite";

import { useT, type PlainKey } from "../../i18n/i18n";
import type { LintRule, LintRuleSummary } from "../../services/wiki.service";
import { useStore } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { HealthItem } from "./health-item";

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
}: {
  summary: LintRuleSummary;
  pages: PageTreeStore;
  readingTree: boolean;
}) {
  const t = useT();
  const { locale } = useStore().preferences;
  const text = labels[summary.rule];
  const title = t(text.title);
  const count = new Intl.NumberFormat(locale).format(summary.count) + (summary.count_capped ? "+" : "");
  return (
    <section
      id={summary.rule}
      aria-labelledby={`${summary.rule}-heading`}
      className="min-w-0 space-y-2 rounded-md border p-4"
    >
      <h2 id={`${summary.rule}-heading`} tabIndex={-1} className="font-semibold outline-none">
        {title}
      </h2>
      <p className="text-sm text-muted-foreground">{t(text.description)}</p>
      <p className="text-sm">
        {summary.count === 1 && !summary.count_capped ? t("health.oneIssue") : t("health.count", { count })}
      </p>
      {summary.items.length > 0 && (
        <ul aria-label={title} className="divide-y">
          {summary.items.map((item) => (
            <HealthItem key={item.item_key} item={item} pages={pages} readingTree={readingTree} />
          ))}
        </ul>
      )}
    </section>
  );
});
