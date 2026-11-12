import { observer } from "mobx-react-lite";
import { useMemo, useRef, type RefObject } from "react";
import { Link } from "react-router";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";

import { NotLoaded } from "../../app/not-loaded";
import { Button } from "../../components/ui/button";
import { formatDateTime } from "../../i18n/format";
import { useT, type PlainKey } from "../../i18n/i18n";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type {
  Upgrade,
  UpgradeCounts as Counts,
  UpgradeGates as Gates,
  UpgradeService,
} from "../../services/upgrade.service";
import type { LintPage, LintRule, LintRuleSummary } from "../../services/wiki.service";
import { useStore, useUpgrades } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { nodeTarget } from "../node-target";
import { openOf } from "../open-node";
import { useWorkspace } from "../workspace/workspace-layout";
import { healthCancelled, healthRequest, healthRetry, type HealthScope } from "./health-request";
import { HealthRule, healthRuleOrder, type HealthCard } from "./health-rule";

/**
 * CheckControl is what one entry to the upgrade's page holds of its checks
 * (M11/P2 design 4.3): whether the entry is still on, the generation of
 * the check it shows, and the requests of that generation, which a new
 * check, or leaving the page, aborts.
 */
export type CheckControl = { alive: boolean; generation: number; controllers: Set<AbortController> };

/** CheckEntry is one entry to the upgrade's page: its nonce, its control, the location it was entered at. */
export type CheckEntry = { nonce: number; control: RefObject<CheckControl>; locationKey: string };

/**
 * TreeRead is what the upgrade's page read of the notebook's tree, which
 * its sections' nodes take their paths from (M11/P3 design 4.3, as the
 * health page's): why its read failed, if it did; whether it is read; how
 * to read it again.
 */
export type TreeRead = { error: unknown; reading: boolean; retry: () => void };

/** What a check's cards, or a report's, hold past their first pages, and whether the upgrade moved on meanwhile. */
type CheckView = { cards: Partial<Record<LintRule, HealthCard>>; moved: boolean };
const noView: CheckView = { cards: {}, moved: false };
const unchanging = { revalidateIfStale: false, revalidateOnFocus: false, revalidateOnReconnect: false };
// The upgrade's page keeps its tree as the left column's: an item's node not in it is not read again here.
const noPaths = () => undefined;

/**
 * itemsReader reads a rule's items of the upgrade's check for a section of
 * the upgrade's page, after a cursor or from the first page (M11/P3 design
 * 4.3): refused as forbidden while the section is current, the account is
 * no longer the notebook's admin, and the workspace's notebooks are read
 * again, in the public provider, before the refusal is thrown.
 */
function itemsReader(
  upgrades: Pick<UpgradeService, "checkItems">,
  notebookId: string,
  current: () => boolean,
  rereadNotebooks: () => void
): (rule: LintRule, cursor: string | undefined, signal: AbortSignal) => Promise<LintPage> {
  return async (rule, cursor, signal) => {
    try {
      return await upgrades.checkItems(notebookId, rule, cursor, signal);
    } catch (refusal) {
      if (refusal instanceof ApiError && refusal.code === "forbidden" && current()) rereadNotebooks();
      throw refusal;
    }
  };
}

/** TreeFailure says, above a section's nodes, that the tree they take their paths from failed to read: try again. */
function TreeFailure({ tree }: { tree: TreeRead }) {
  const t = useT();
  return tree.error === undefined || tree.reading ? null : (
    <fieldset aria-label={t("health.paths")}>
      <NotLoaded error={tree.error} retry={tree.retry} />
    </fieldset>
  );
}

/**
 * UpgradeCheckSection checks the upgrade, upgrading, against an LLM Wiki's
 * contract (M11/P2 design 4.3, decision 259): read only once Check is
 * pressed, a generation of this entry at a time (SWR's key, in this
 * entry's cache), shown from what the read answered, so that the section
 * gone and back, as a submission of one's own takes it away while it is
 * checked (decision 264), shows the same generation without reading it
 * again. Each gate, passed or not, with its nodes; the counts; the rules
 * with issues as the health page's cards, their More reading the check's
 * items, the others in a disclosure. The upgrade no longer under way (its
 * check or a card's More refused for its state) says so and reads the
 * upgrade again; the check or a card's More refused as forbidden, the
 * account is no longer the notebook's admin: the workspace's notebooks are
 * read again. The tree failed to read, it says so above the nodes, which
 * are left without their paths.
 */
export const UpgradeCheckSection = observer(function UpgradeCheckSection({
  notebook,
  upgrade,
  entry,
  generation,
  recheck,
  rereadUpgrade,
  rereadNotebooks,
  pages,
  tree,
}: {
  notebook: Notebook;
  upgrade: Upgrade;
  entry: CheckEntry;
  /** The generation of the check to show: none before the first. */
  generation: number;
  /** Checks again: the next generation. */
  recheck: () => void;
  rereadUpgrade: () => void;
  /** Reads the workspace's notebooks again, in the public provider. */
  rereadNotebooks: () => void;
  pages: PageTreeStore;
  tree: TreeRead;
}) {
  const store = useStore();
  const upgrades = useUpgrades();
  const workspace = useWorkspace();
  const t = useT();
  const { locale } = store.preferences;
  const { cache, mutate } = useSWRConfig();
  const scope = useMemo<HealthScope>(
    () => ({
      key: [
        "upgrade-check",
        store.loginId,
        workspace.id,
        notebook.id,
        upgrade.id,
        entry.locationKey,
        entry.nonce,
        generation,
      ],
      controllers: {
        add: (controller) => entry.control.current.controllers.add(controller),
        delete: (controller) => entry.control.current.controllers.delete(controller),
      },
      current: () =>
        entry.control.current.alive &&
        entry.control.current.generation === generation &&
        store.auth.state.loginId === store.loginId,
    }),
    [store, workspace.id, notebook.id, upgrade.id, entry, generation]
  );
  const read = generation > 0;
  const { data, error } = useSWR(
    read ? scope.key : null,
    () => healthRequest(scope, (signal) => upgrades.check(notebook.id, signal)),
    {
      ...unchanging,
      // A generation read already, or under way, is shown from this entry's cache as the section comes back.
      revalidateOnMount: cache.get(unstable_serialize(scope.key)) === undefined,
      onErrorRetry: healthRetry(scope.current),
      onError: (refusal: unknown) => {
        if (!scope.current() || !(refusal instanceof ApiError)) return;
        if (refusal.code === "forbidden") rereadNotebooks();
        else if (refusal.code === "wiki.upgrade_state") rereadUpgrade();
      },
    }
  );
  const viewKey = [...scope.key, "view"];
  const { data: view = noView } = useSWR<CheckView>(viewKey, null, { fallbackData: noView });
  function changeView(update: (was: CheckView) => CheckView): Promise<CheckView | undefined> {
    if (!scope.current()) return Promise.resolve(undefined);
    return mutate<CheckView>(viewKey, (was) => (scope.current() ? update(was ?? noView) : was), {
      revalidate: false,
    });
  }
  const failed = error === undefined || healthCancelled(error) ? undefined : error;
  const moved = view.moved || (failed instanceof ApiError && failed.code === "wiki.upgrade_state");
  const checking = read && data === undefined && error === undefined;
  const number = new Intl.NumberFormat(locale);
  let said = checking ? t("upgrade.checking") : "";
  if (data !== undefined) {
    const notPassed = gateKeys.filter((key) => !data.gates[key].passed).length;
    said = [
      data.passed
        ? t("upgrade.checkPassed")
        : notPassed === 1
          ? t("upgrade.checkFailedOne")
          : t("upgrade.checkFailed", { count: number.format(notPassed) }),
      t("upgrade.checkedAt", { time: formatDateTime(data.computed_at, locale) }),
    ].join(" ");
  }
  const readItems = itemsReader(upgrades, notebook.id, scope.current, rereadNotebooks);
  const rules = healthRuleOrder.flatMap((rule) => data?.rules.find((found) => found.rule === rule) ?? []);
  const found = rules.filter((rule) => rule.count > 0);
  const clear = rules.filter((rule) => rule.count === 0);
  function card(rule: LintRuleSummary) {
    const first: HealthCard = { items: rule.items, cursor: rule.next_cursor, told: "", again: false };
    return (
      <HealthRule
        key={`${rule.rule}:${generation}`}
        summary={rule}
        card={view.cards[rule.rule] ?? first}
        scope={scope}
        mode="llm_wiki"
        pages={pages}
        readingTree={tree.reading}
        treeFailed={tree.error !== undefined}
        disabled={moved}
        paths={noPaths}
        mismatch={() => {
          void changeView((was) => ({ ...was, moved: true }));
          rereadUpgrade();
        }}
        change={async (update) => {
          const changed = await changeView((was) => ({
            ...was,
            cards: { ...was.cards, [rule.rule]: update(was.cards[rule.rule] ?? first) },
          }));
          return changed?.cards[rule.rule];
        }}
        read={readItems}
        moved="wiki.upgrade_state"
        level={3}
      />
    );
  }
  return (
    <section aria-labelledby="upgrade-check-heading" className="min-w-0 space-y-4">
      <h2 id="upgrade-check-heading" className="text-lg font-semibold">
        {t("upgrade.check")}
      </h2>
      <p>{t("upgrade.checkIntro")}</p>
      <Button variant="outline" aria-busy={checking || undefined} onClick={recheck}>
        {t(read ? "upgrade.checkAgain" : "upgrade.checkRead")}
      </Button>
      <output aria-label={t("upgrade.checkResult")} className="block text-sm">
        {said}
      </output>
      {moved && <p className="text-sm">{t("upgrade.checkStateChanged")}</p>}
      {failed !== undefined && !moved && <NotLoaded error={failed} retry={recheck} />}
      {data !== undefined && (
        <>
          <TreeFailure tree={tree} />
          <UpgradeGates gates={data.gates} stalePages={data.stale_pages} pages={pages} tree={tree} />
          <UpgradeCounts counts={data.counts} />
          {found.map(card)}
          {clear.length > 0 && (
            <details className="space-y-3">
              <summary className="cursor-pointer font-medium">{t("health.clear", { count: clear.length })}</summary>
              {clear.map(card)}
            </details>
          )}
        </>
      )}
    </section>
  );
});

/**
 * UpgradeReportSection is the report the upgrade's submission kept, while
 * it is reviewing (M11/P2 design 4.3, decision 259): read only, its gates
 * and counts as a check's, its rules as the check's cards (M11/P3 design
 * 4.3, decision 287), whose More reads the check's items from their first
 * page, the report keeping none of their cursors: a rule whose first items
 * are all of them has none. Its cards are of this entry to the page, in
 * its cache, and of the submission shown: left and come back to, or
 * another submission's report shown, they start again from the report.
 * The upgrade no longer in review (a card's More refused for its state)
 * says so and reads the upgrade again; a More refused as forbidden reads
 * the workspace's notebooks again. None yet: a submission is being
 * checked, or the last one did not finish.
 */
export const UpgradeReportSection = observer(function UpgradeReportSection({
  notebook,
  upgrade,
  entry,
  rereadUpgrade,
  rereadNotebooks,
  pages,
  tree,
}: {
  notebook: Notebook;
  upgrade: Upgrade;
  entry: CheckEntry;
  rereadUpgrade: () => void;
  /** Reads the workspace's notebooks again, in the public provider. */
  rereadNotebooks: () => void;
  pages: PageTreeStore;
  tree: TreeRead;
}) {
  const store = useStore();
  const upgrades = useUpgrades();
  const workspace = useWorkspace();
  const t = useT();
  const { locale } = store.preferences;
  const { mutate } = useSWRConfig();
  const report = upgrade.report;
  // The submission an upgrade answers is known by when it was submitted: each, a takeover too, has its own time.
  const submitted = upgrade.submitted_at;
  // The submission shown, as each render shows it: a card's request for another one is not current.
  const shown = useRef(submitted);
  shown.current = submitted;
  const scope = useMemo<HealthScope>(
    () => ({
      key: ["upgrade-report", store.loginId, workspace.id, notebook.id, upgrade.id, submitted],
      controllers: {
        add: (controller) => entry.control.current.controllers.add(controller),
        delete: (controller) => entry.control.current.controllers.delete(controller),
      },
      current: () =>
        entry.control.current.alive && store.auth.state.loginId === store.loginId && shown.current === submitted,
    }),
    [store, workspace.id, notebook.id, upgrade.id, submitted, entry]
  );
  const viewKey = [...scope.key, "view"];
  const { data: view = noView } = useSWR<CheckView>(viewKey, null, { fallbackData: noView });
  function changeView(update: (was: CheckView) => CheckView): Promise<CheckView | undefined> {
    if (!scope.current()) return Promise.resolve(undefined);
    return mutate<CheckView>(viewKey, (was) => (scope.current() ? update(was ?? noView) : was), {
      revalidate: false,
    });
  }
  const readItems = itemsReader(upgrades, notebook.id, scope.current, rereadNotebooks);
  const rules = healthRuleOrder.flatMap((rule) => report?.rules.find((found) => found.rule === rule) ?? []);
  const found = rules.filter((rule) => rule.count > 0);
  const clear = rules.filter((rule) => rule.count === 0);
  function card(rule: (typeof rules)[number]) {
    const first: HealthCard = {
      items: rule.items,
      cursor: rule.count > rule.items.length || rule.count_capped ? undefined : null,
      told: "",
      again: false,
    };
    return (
      <HealthRule
        key={`${rule.rule}:${submitted}`}
        summary={rule}
        card={view.cards[rule.rule] ?? first}
        scope={scope}
        mode="llm_wiki"
        pages={pages}
        readingTree={tree.reading}
        treeFailed={tree.error !== undefined}
        disabled={view.moved}
        paths={noPaths}
        mismatch={() => {
          void changeView((was) => ({ ...was, moved: true }));
          rereadUpgrade();
        }}
        change={async (update) => {
          const changed = await changeView((was) => ({
            ...was,
            cards: { ...was.cards, [rule.rule]: update(was.cards[rule.rule] ?? first) },
          }));
          return changed?.cards[rule.rule];
        }}
        read={readItems}
        moved="wiki.upgrade_state"
        level={3}
      />
    );
  }
  return (
    <section aria-labelledby="upgrade-report-heading" className="min-w-0 space-y-4">
      <h2 id="upgrade-report-heading" className="text-lg font-semibold">
        {t("upgrade.report")}
      </h2>
      {report === null ? (
        <p>{t("upgrade.reportPending")}</p>
      ) : (
        <>
          <p>{t("upgrade.reportPassed", { time: formatDateTime(report.computed_at, locale) })}</p>
          {view.moved && <p className="text-sm">{t("upgrade.checkStateChanged")}</p>}
          <TreeFailure tree={tree} />
          <UpgradeGates gates={report.gates} stalePages={report.stale_pages} pages={pages} tree={tree} />
          <UpgradeCounts counts={report.counts} />
          {found.map(card)}
          {clear.length > 0 && (
            <details className="space-y-3">
              <summary className="cursor-pointer font-medium">{t("health.clear", { count: clear.length })}</summary>
              {clear.map(card)}
            </details>
          )}
        </>
      )}
    </section>
  );
});

/** The gates in their order (M11 design 4.7). */
const gateKeys = [
  "agents",
  "source_root",
  "reserved_titles",
  "frontmatter",
  "sources",
  "links",
  "preserved",
  "index",
] as const satisfies readonly (keyof Gates)[];

const gateText: Record<keyof Gates, PlainKey> = {
  agents: "upgrade.gate.agents",
  source_root: "upgrade.gate.source_root",
  reserved_titles: "upgrade.gate.reserved_titles",
  frontmatter: "upgrade.gate.frontmatter",
  sources: "upgrade.gate.sources",
  links: "upgrade.gate.links",
  preserved: "upgrade.gate.preserved",
  index: "upgrade.gate.index",
};

/**
 * UpgradeGates is each gate of a check, passed or not: the nodes at the
 * root holding a reserved title, the pages that lead to more pages that do
 * not exist and the nodes not kept, each leading to it while the tree has
 * it; how many pages are worse off, how many nodes and pages are kept; the
 * pages of an old link index.
 */
const UpgradeGates = observer(function UpgradeGates({
  gates,
  stalePages,
  pages,
  tree,
}: {
  gates: Gates;
  stalePages: number;
  pages: PageTreeStore;
  tree: TreeRead;
}) {
  const t = useT();
  const number = new Intl.NumberFormat(useStore().preferences.locale);
  return (
    <ul aria-label={t("upgrade.gates")} className="space-y-2 text-sm">
      {gateKeys.map((key) => {
        const gate = gates[key];
        return (
          <li key={key} className="space-y-1">
            <p className={gate.passed ? undefined : "font-medium text-destructive"}>
              {t(gate.passed ? "upgrade.gatePassed" : "upgrade.gateFailed", { gate: t(gateText[key]) })}
            </p>
            {key === "links" && gates.links.pages_worse > 0 && (
              <p>{t("upgrade.gateLinksWorse", { count: number.format(gates.links.pages_worse) })}</p>
            )}
            {key === "preserved" && (
              <p>
                {t("upgrade.gatePreserved", {
                  nodesKept: number.format(gates.preserved.nodes_kept),
                  nodes: number.format(gates.preserved.nodes),
                  pagesKept: number.format(gates.preserved.pages_kept),
                  pages: number.format(gates.preserved.pages),
                })}
              </p>
            )}
            {key === "index" && !gate.passed && <p>{t("health.stale", { count: number.format(stalePages) })}</p>}
            {"node_ids" in gate && gate.node_ids.length > 0 && (
              <ul className="space-y-1 ps-4">
                {gate.node_ids.map((id) => (
                  <li key={id}>
                    <GateNode id={id} pages={pages} readingTree={tree.reading} treeFailed={tree.error !== undefined} />
                  </li>
                ))}
              </ul>
            )}
          </li>
        );
      })}
    </ul>
  );
});

/**
 * GateNode leads to a gate's node by its path while the tree has it; else
 * it is loading, or no longer there; left empty once the tree failed to
 * read, which does not tell (M11/P3 design 4.3).
 */
const GateNode = observer(function GateNode({
  id,
  pages,
  readingTree,
  treeFailed,
}: {
  id: string;
  pages: PageTreeStore;
  readingTree: boolean;
  treeFailed: boolean;
}) {
  const t = useT();
  const { slug } = useWorkspace();
  const target = nodeTarget(pages, id);
  if (target === undefined) {
    if (!readingTree && treeFailed) return null;
    return <span className="text-muted-foreground">{t(readingTree ? "status.loading" : "upgrade.gateNodeGone")}</span>;
  }
  const open = openOf(
    { id, kind: target.node.kind, parent: target.node.parent_id },
    `/${slug}/notebooks/${pages.notebookId}`
  );
  return (
    <Link to={open.to} state={open.state} className="underline underline-offset-4">
      {target.path}
    </Link>
  );
});

/** UpgradeCounts is what a check counted; a count of wanted pages capped ends with +, as the health page's. */
const UpgradeCounts = observer(function UpgradeCounts({ counts }: { counts: Counts }) {
  const t = useT();
  const number = new Intl.NumberFormat(useStore().preferences.locale);
  const wanted = ({ count, count_capped: capped }: Counts["wanted_now"]) => number.format(count) + (capped ? "+" : "");
  const rows = [
    ["upgrade.counts.sources", number.format(counts.sources)],
    ["upgrade.counts.movedIntoSources", number.format(counts.moved_into_sources)],
    ["upgrade.counts.wikiPages", number.format(counts.wiki_pages)],
    ["upgrade.counts.newPages", number.format(counts.new_pages)],
    ["upgrade.counts.changesets", number.format(counts.changesets)],
    ["upgrade.counts.wantedBefore", wanted(counts.wanted_before)],
    ["upgrade.counts.wantedNow", wanted(counts.wanted_now)],
  ] as const;
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
      {rows.map(([label, value]) => (
        <div key={label} className="contents">
          <dt className="text-muted-foreground">{t(label)}</dt>
          <dd>{value}</dd>
        </div>
      ))}
    </dl>
  );
});
