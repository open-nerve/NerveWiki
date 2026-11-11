import type { LintItem, LintPage, LintRule, LintRuleSummary } from "@nervewiki/api-client";

import { titleKey } from "../lib/title-key";
import type { Notebook } from "../services/notebook.service";
import type { TransferJob } from "../services/transfer.service";
import type { Upgrade, UpgradeCheck, UpgradeReport } from "../services/upgrade.service";
import { under } from "../stores/transfer.store";
import type { eventServer } from "./event-server";
import { json, notebookJSON, problem, workspaceJSON, type Answer } from "./fakes";
import { jobChangesetId, jobJSON, jobsServer, reportOf, underWay, writing } from "./jobs-server";
import { after, cursorOf, isRule } from "./lint-server";
import { ada, bob, pageNode } from "./page-server";

const plans = `/api/v0/notebooks/${notebookJSON.id}`;

/** The pages an upgrade's start writes when Plans lacks them, at its root: raw, then AGENTS (M11/P1 design 3.14). */
export const seededRaw = pageNode(90, "raw");
export const seededAgents = pageNode(91, "AGENTS");

/**
 * upgradeJSON is Plans' upgrade as its admins read it (M11/P1 design 4.4):
 * upgrading, started by Ada on 2026-10-05, its source root the raw it wrote
 * with AGENTS, of a baseline of 4 nodes, 3 pages, none wanted; more says
 * otherwise.
 */
export function upgradeJSON(more: Partial<Upgrade> = {}): Upgrade {
  return {
    id: "0199a2b4-0000-7000-8000-0000000a0001",
    notebook_id: notebookJSON.id,
    state: "upgrading",
    started_by: ada,
    started_at: "2026-10-05T08:00:00Z",
    source_root_id: seededRaw.id,
    seeded_page_ids: [seededRaw.id, seededAgents.id],
    baseline: { nodes: 4, pages: 3, wanted: 0 },
    submitted_by: null,
    submitted_at: null,
    rollback_job_id: null,
    ending: null,
    rollback_complete: null,
    rollback_left: null,
    report: null,
    ...more,
  };
}

/** The rules of an LLM Wiki's health, in their order (M10/P3). */
const lintRules: readonly LintRule[] = [
  "wanted",
  "orphans",
  "ambiguous",
  "invalid_frontmatter",
  "missing_type",
  "missing_summary",
  "undigested_sources",
  "invalid_sources",
];

/** checkCursor is the cursor of rule's item at the node id in a check of Plans' upgrade: bound to its source root. */
function checkCursor(rule: LintRule, id: string): string {
  const text = JSON.stringify({
    v: 1,
    p: { v: 1, r: rule, n: notebookJSON.id, s: `E:llm_wiki:${seededRaw.id}`, i: id },
  });
  return btoa(text).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

/** missingTypes are the items of missing_type of the pages ids, in their order, as a check of Plans' upgrade has them. */
export function missingTypes(ids: readonly string[]): Extract<LintItem, { rule: "missing_type" }>[] {
  return ids.map((id) => ({
    rule: "missing_type",
    item_key: `p:${id}`,
    page_id: id,
    cursor: checkCursor("missing_type", id),
  }));
}

/** ruleOf is a check's rule of count issues, its first items shown, more after the last of them unless said. */
export function ruleOf(
  rule: LintRule,
  count: number,
  items: readonly LintItem[] = [],
  more: Partial<LintRuleSummary> = {}
): LintRuleSummary {
  return {
    rule,
    count,
    count_capped: false,
    items,
    next_cursor: count > items.length ? (items.at(-1)?.cursor ?? null) : null,
    ...more,
  } as LintRuleSummary;
}

/**
 * checkJSON is a check of Plans' upgrade as its admins read it (M11/P2
 * design 4.3): every gate passed, of the four nodes and three pages it
 * started with, no rule with an issue; more says otherwise.
 */
export function checkJSON(more: Partial<UpgradeCheck> = {}): UpgradeCheck {
  return {
    computed_at: "2026-10-06T09:30:00Z",
    stale_pages: 0,
    passed: true,
    gates: {
      agents: { passed: true },
      source_root: { passed: true },
      reserved_titles: { passed: true, node_ids: [] },
      frontmatter: { passed: true },
      sources: { passed: true },
      links: { passed: true, pages_worse: 0, node_ids: [] },
      preserved: { passed: true, nodes: 4, nodes_kept: 4, pages: 3, pages_kept: 3, node_ids: [] },
      index: { passed: true },
    },
    rules: lintRules.map((rule) => ruleOf(rule, 0)),
    counts: {
      sources: 0,
      moved_into_sources: 0,
      wiki_pages: 3,
      new_pages: 2,
      changesets: 1,
      wanted_before: { count: 0, count_capped: false },
      wanted_now: { count: 0, count_capped: false },
    },
    ...more,
  };
}

/** reportJSON is the report a submission of Plans' upgrade kept: checkJSON's, its rules' items without cursors. */
export function reportJSON(more: Partial<UpgradeReport> = {}): UpgradeReport {
  const { rules, ...check } = checkJSON();
  return {
    ...check,
    rules: rules.map(({ rule, count, count_capped }) => ({ rule, count, count_capped, items: [] })),
    ...more,
  };
}

/**
 * How a submission's second step ends (M11/P2 design 3.12): its report
 * kept; upgrading again, a gate not passed, an old index, the check's
 * computations busy.
 */
type SubmitOutcome = "stored" | "not_ready" | "stale_index" | "busy";

/** summaryOf is the upgrade as Plans' readers see it in the notebook (Notebook.upgrade), or none once it ended. */
function summaryOf(upgrade: Upgrade | undefined): Notebook["upgrade"] {
  return upgrade === undefined || upgrade.state === "approved" || upgrade.state === "rolled_back"
    ? null
    : { id: upgrade.id, state: upgrade.state, started_by: upgrade.started_by.user_id, started_at: upgrade.started_at };
}

/**
 * rollbackJobJSON is the job n of an upgrade's rollback, Ada's, as the
 * server answers it: named upgrade, with the rollback changeset it writes
 * (jobChangesetId), done, its four nodes put back; more changes what it
 * says.
 */
export function rollbackJobJSON(n: number, more: Partial<TransferJob> = {}): TransferJob {
  return jobJSON(n, {
    kind: "rollback",
    name: "upgrade",
    changeset_id: jobChangesetId(n),
    report: reportOf(null, { put_back: 4 }),
    ...more,
  });
}

/** What a rollback left: none of the three. */
const nothingLeft = { nodes: 0, pages: 0, created: 0 };

type UpgradeServerOptions = NonNullable<Parameters<typeof jobsServer>[0]> & {
  /** Plans' upgrade under way, if any. */
  upgrade?: Upgrade;
  /** The event stream (GET events), whose last connection a submission's two steps each tell (mode). */
  events?: ReturnType<typeof eventServer>;
};

/**
 * upgradeServer is jobsServer with Plans' upgrade (M11/P1 design 4.4), as
 * the server answers its admins, in the contract's order of codes:
 *
 * - its read (GET upgrade): Plans gone 404 notebook.not_found; not its
 *   admin 403 forbidden; none under way 404 wiki.upgrade_not_found; else
 *   the upgrade, logged as "GET upgrade";
 * - its start (POST upgrade): Plans gone 404 notebook.not_found; not its
 *   admin 403 forbidden; an LLM Wiki 409 notebook.already_llm_wiki; an
 *   upgrade under way 409 notebook.upgrading; a job writing the tree under
 *   way 409 transfer.busy; a page someone holds 409
 *   wiki.upgrade_pages_locked naming each; an attachment at the root
 *   titled raw or AGENTS 409 wiki.upgrade_title_taken; else it writes the
 *   raw and the AGENTS Plans lacks at its root, its source root the raw
 *   at the root, its baseline what was there, and answers the upgrade,
 *   started by Ada. Logged as "START <locale>".
 *
 * - its abandonment (POST upgrade/abandon): Plans gone 404
 *   notebook.not_found; not its admin 403 forbidden; none under way 409
 *   wiki.upgrade_state; its rollback's job under way 409 transfer.busy;
 *   the queue full (queueFull) 503 server_busy; else its rollback's job,
 *   Ada's, queued, first among the jobs, and the upgrade rolling back with
 *   it, abandoned unless it was rejected. Logged as "ABANDON". Rolling
 *   back with its job stopped, that is its continuation;
 * - its rollback's end (POST upgrade/end-rollback): 404 and 403 as
 *   above; not rolling back 409 wiki.upgrade_state; its job under way 409
 *   transfer.busy; else the upgrade ends, rolled back, as the rollback
 *   left it (left). Logged as "END ROLLBACK".
 *
 * Its check (M11/P2 design 4.3), as the server answers it, in the
 * contract's order of codes:
 *
 * - the check (GET upgrade/check): Plans gone 404 notebook.not_found; not
 *   its admin 403 forbidden; no upgrade upgrading or reviewing 409
 *   wiki.upgrade_state; the computations busy (checkBusy) 503
 *   server_busy; else the check (check). Logged as "CHECK";
 * - a rule's items (GET upgrade/check/{rule}): a limit not an integer or
 *   a cursor that does not decode 400 bad_request; then 404 and 403 as
 *   the check's; a rule unknown 404 wiki.lint_rule_not_found; no upgrade
 *   upgrading or reviewing 409 wiki.upgrade_state; a cursor bound to
 *   another notebook, rule or scope than the explicit LLM Wiki's of the
 *   upgrade's source root (a lint's) 400 bad_request; a limit past 1-100
 *   422 validation_failed; busy 503; else the rule's items (checkItems)
 *   after the cursor. Logged as "CHECK <rule> <cursor>";
 * - its submission (POST upgrade/submit), in the server's two steps:
 *   404 and 403 as above; not upgrading 409 wiki.upgrade_state; else the
 *   upgrade is reviewing, submitted by Ada, without a report, and the
 *   mode event is sent; once the submission's check is released
 *   (submission.checked), an upgrade no longer that submission's (taken
 *   over, abandoned) is 409 wiki.upgrade_state, nothing written; else, as
 *   submission.outcome says, its report kept (submission.report), or
 *   upgrading again, the submission's columns cleared, 409
 *   wiki.upgrade_not_ready naming the gates, 409 wiki.upgrade_stale_index,
 *   503 server_busy; the mode event is sent again once submission.mode is
 *   released, and the answer once submission.answered is. Logged as
 *   "SUBMIT".
 *
 * A rolling back upgrade read says what its rollback left (left) once its
 * job is neither queued nor running, as the server counts it then; none
 * while it is. Plans as the list holds it carries the upgrade (withUpgrade
 * keeps the two together). It answers no capacity nor time out
 * (server_busy) on a start and no stale index, which a test answers
 * through answers. The test moves the rollback's job as the server's work
 * would (rollbackJob), and ends the upgrade as a rollback that put
 * everything back does (rolledBack).
 */
export function upgradeServer({ upgrade, events, answers = {}, ...options }: UpgradeServerOptions = {}) {
  const routes: Record<string, Answer> = {
    ...(events === undefined ? {} : { "GET /api/v0/events": events.answer }),
    [`GET ${plans}/upgrade`]: () => {
      server.asked.push("GET upgrade");
      if (server.notebookGone) {
        return problem(404, "notebook.not_found");
      }
      if (server.notebook.role !== "admin") {
        return problem(403, "forbidden");
      }
      return server.upgrade === undefined || summaryOf(server.upgrade) === null
        ? problem(404, "wiki.upgrade_not_found")
        : json(read(server.upgrade));
    },
    [`POST ${plans}/upgrade`]: async (request) => {
      const { template_locale: locale } = (await request.clone().json()) as { template_locale?: string };
      server.asked.push(`START ${locale ?? "none"}`);
      if (server.notebookGone) {
        return problem(404, "notebook.not_found");
      }
      if (server.notebook.role !== "admin") {
        return problem(403, "forbidden");
      }
      if (server.notebook.mode === "llm_wiki") {
        return problem(409, "notebook.already_llm_wiki");
      }
      if (summaryOf(server.upgrade) !== null) {
        return problem(409, "notebook.upgrading");
      }
      if (server.busy || writing(server.jobs)) {
        return problem(409, "transfer.busy");
      }
      const locks = server.nodes.flatMap((node) => {
        const holder = server.lockOf(node.id).holder;
        return holder === null ? [] : [{ page_id: node.id, ...holder }];
      });
      if (locks.length > 0) {
        return problem(409, "wiki.upgrade_pages_locked", { locks: locks.slice(0, 100) });
      }
      const atRoot = (key: string) =>
        server.nodes.find((node) => node.parent_id === null && titleKey(node.name) === titleKey(key));
      if ([atRoot("raw"), atRoot("AGENTS")].some((node) => node?.kind === "asset")) {
        return problem(409, "wiki.upgrade_title_taken");
      }
      const baseline = server.nodes;
      const seeded = [atRoot("raw") === undefined ? seededRaw : undefined, atRoot("AGENTS") ? undefined : seededAgents];
      const written = seeded.filter((node) => node !== undefined);
      server.nodes = [...server.nodes, ...written];
      const started = upgradeJSON({
        source_root_id: atRoot("raw")?.id ?? seededRaw.id,
        seeded_page_ids: written.map((node) => node.id),
        baseline: {
          nodes: baseline.length,
          pages: baseline.filter((node) => node.kind === "page").length,
          wanted: 0,
        },
      });
      server.withUpgrade(started);
      return json(started);
    },
    [`POST ${plans}/upgrade/abandon`]: () => {
      server.asked.push("ABANDON");
      const refused = refusedChange();
      if (refused !== undefined) {
        return refused;
      }
      const held = server.upgrade;
      if (held === undefined || summaryOf(held) === null) {
        return problem(409, "wiki.upgrade_state");
      }
      if (rollingNow()) {
        return problem(409, "transfer.busy");
      }
      if (server.queueFull) {
        return problem(503, "server_busy", {}, { "Retry-After": "2" });
      }
      const job = underWay(rollbackJobJSON(++rollbacks, { created_at: "2026-10-06T08:00:00Z" }), 0, 0);
      server.jobs = [job, ...server.jobs];
      const rolling: Upgrade = {
        ...held,
        state: "rolling_back",
        ending: held.ending ?? "abandoned",
        rollback_job_id: job.id,
      };
      server.withUpgrade(rolling);
      return json(read(rolling));
    },
    [`GET ${plans}/upgrade/check`]: () => {
      server.asked.push("CHECK");
      const refused = refusedChange() ?? (checked() ? undefined : problem(409, "wiki.upgrade_state"));
      if (refused !== undefined) {
        return refused;
      }
      return server.checkBusy ? problem(503, "server_busy", {}, { "Retry-After": "1" }) : json(server.check);
    },
    [`GET ${plans}/upgrade/check/*`]: (request) => {
      const url = new URL(request.url);
      const rule = decodeURIComponent(url.pathname.split("/").at(-1) ?? "");
      const raw = url.searchParams.get("cursor");
      const limitText = url.searchParams.get("limit");
      server.asked.push(`CHECK ${rule} ${raw ?? "none"}`);
      const position = raw === null ? undefined : cursorOf(raw);
      if ((limitText !== null && !/^[+-]?\d+$/.test(limitText)) || (raw !== null && position === undefined)) {
        return problem(400, "bad_request");
      }
      const refused = refusedChange();
      if (refused !== undefined) {
        return refused;
      }
      if (!isRule(rule)) {
        return problem(404, "wiki.lint_rule_not_found");
      }
      if (!checked()) {
        return problem(409, "wiki.upgrade_state");
      }
      const scope = `E:llm_wiki:${server.upgrade?.source_root_id ?? ""}`;
      if (
        position !== undefined &&
        (position.notebook !== notebookJSON.id || position.rule !== rule || position.scope !== scope)
      ) {
        return problem(400, "bad_request");
      }
      const limit = limitText === null ? 50 : Number(limitText);
      if (limit < 1 || limit > 100) {
        return problem(422, "validation_failed", {
          errors: [{ field: "limit", code: "out_of_range", message: "must be between 1 and 100" }],
        });
      }
      if (server.checkBusy) {
        return problem(503, "server_busy", {}, { "Retry-After": "1" });
      }
      const all = (server.checkItems[rule] ?? []).filter((item) => position === undefined || after(item, position));
      const data = all.slice(0, limit);
      const page: LintPage = {
        mode: "llm_wiki",
        computed_at: server.check.computed_at,
        stale_pages: server.check.stale_pages,
        data,
        next_cursor: all.length > data.length ? (data.at(-1)?.cursor ?? null) : null,
      };
      return json(page);
    },
    [`POST ${plans}/upgrade/submit`]: async () => {
      server.asked.push("SUBMIT");
      const refused =
        refusedChange() ?? (server.upgrade?.state === "upgrading" ? undefined : problem(409, "wiki.upgrade_state"));
      if (refused !== undefined || server.upgrade === undefined) {
        return refused ?? problem(409, "wiki.upgrade_state");
      }
      const submitted: Upgrade = {
        ...server.upgrade,
        state: "reviewing",
        submitted_by: ada,
        submitted_at: "2026-10-06T09:40:00Z",
        report: null,
      };
      server.withUpgrade(submitted);
      sendMode();
      const { outcome, report, checked, mode, answered } = server.submission;
      await checked;
      if (server.upgrade !== submitted) {
        return problem(409, "wiki.upgrade_state");
      }
      let answer: Response;
      if (outcome === "stored") {
        const kept: Upgrade = { ...submitted, report };
        server.withUpgrade(kept);
        answer = json(kept);
      } else {
        server.withUpgrade({ ...submitted, state: "upgrading", submitted_by: null, submitted_at: null });
        answer =
          outcome === "not_ready"
            ? problem(409, "wiki.upgrade_not_ready", {
                detail: "The upgrade does not pass its checks yet: frontmatter.",
              })
            : outcome === "stale_index"
              ? problem(409, "wiki.upgrade_stale_index")
              : problem(503, "server_busy", {}, { "Retry-After": "1" });
      }
      void mode.then(sendMode);
      await answered;
      return answer;
    },
    [`POST ${plans}/upgrade/end-rollback`]: () => {
      server.asked.push("END ROLLBACK");
      const refused = refusedChange();
      if (refused !== undefined) {
        return refused;
      }
      const held = server.upgrade;
      if (held?.state !== "rolling_back") {
        return problem(409, "wiki.upgrade_state");
      }
      if (rollingNow()) {
        return problem(409, "transfer.busy");
      }
      const { left } = server;
      const ended: Upgrade = {
        ...held,
        state: "rolled_back",
        rollback_complete: left.nodes + left.pages + left.created === 0,
        rollback_left: left,
      };
      server.withUpgrade(ended);
      return json(ended);
    },
  };
  let rollbacks = 80;
  // Plans gone, or the account not its admin.
  const refusedChange = () =>
    server.notebookGone
      ? problem(404, "notebook.not_found")
      : server.notebook.role === "admin"
        ? undefined
        : problem(403, "forbidden");
  // The mode event a submission's step sends its notebook's subscribers: the last stream's, while connected.
  const sendMode = () => {
    const stream = events?.streams.at(-1);
    if (stream !== undefined && !stream.ended) {
      stream.send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });
    }
  };
  // An upgrade a check reads: upgrading or reviewing.
  const checked = () => server.upgrade?.state === "upgrading" || server.upgrade?.state === "reviewing";
  // The upgrade's rollback's job, if it has one under way.
  const rollingNow = () => server.jobs.some((job) => job.id === server.upgrade?.rollback_job_id && under(job));
  // The upgrade as a read answers it: rolling back, what its rollback left once its job is not under way.
  const read = (held: Upgrade): Upgrade =>
    held.state === "rolling_back" ? { ...held, rollback_left: rollingNow() ? null : server.left } : held;
  const server = Object.assign(jobsServer({ ...options, answers: { ...routes, ...answers } }), {
    /** Plans' upgrade: under way, or the last one, ended; none before its first. */
    upgrade: undefined as Upgrade | undefined,
    /** What the upgrade's rollback left once its job stopped, as the server counts it then. */
    left: nothingLeft as NonNullable<Upgrade["rollback_left"]>,
    /** What the upgrade's check answers. */
    check: checkJSON(),
    /** Each rule's items of the check, all of them, in their order: its pages read past a cursor. */
    checkItems: {} as Partial<Record<LintRule, LintItem[]>>,
    /** Whether the check's computations are busy: 503 server_busy. */
    checkBusy: false,
    /** How a submission's second step ends, and what it waits for: nothing unless a test holds it. */
    submission: {
      outcome: "stored" as SubmitOutcome,
      report: reportJSON(),
      checked: Promise.resolve(),
      mode: Promise.resolve(),
      answered: Promise.resolve(),
    },
    /** withUpgrade gives Plans upgrade, or none, in its read and in the notebook the list holds. */
    withUpgrade(next: Upgrade | undefined): void {
      server.upgrade = next;
      server.notebook = { ...server.notebook, upgrade: summaryOf(next) };
    },
    /** rollbackJob changes the upgrade's rollback's job as the server's work would. */
    rollbackJob(change: (job: TransferJob) => TransferJob): void {
      server.jobs = server.jobs.map((job) => (job.id === server.upgrade?.rollback_job_id ? change(job) : job));
    },
    /** rolledBack has the rollback's job done, everything put back, and the upgrade ended with it, rolled back. */
    rolledBack(): void {
      server.rollbackJob((job) => rollbackJobJSON(0, { id: job.id, changeset_id: job.changeset_id }));
      const held = server.upgrade;
      if (held !== undefined) {
        server.withUpgrade({ ...held, state: "rolled_back", rollback_complete: true, rollback_left: nothingLeft });
      }
    },
  });
  server.withUpgrade(upgrade);
  return server;
}

/** bobsUpgrade is Plans' upgrade under way started by Bob, another of its admins. */
export function bobsUpgrade(more: Partial<Upgrade> = {}): Upgrade {
  return upgradeJSON({ started_by: bob, ...more });
}
