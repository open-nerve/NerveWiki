import { titleKey } from "../lib/title-key";
import type { Notebook } from "../services/notebook.service";
import type { Upgrade } from "../services/upgrade.service";
import { json, notebookJSON, problem, type Answer } from "./fakes";
import { jobsServer, writing } from "./jobs-server";
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
    ...more,
  };
}

/** summaryOf is the upgrade as Plans' readers see it in the notebook (Notebook.upgrade), or none once it ended. */
function summaryOf(upgrade: Upgrade | undefined): Notebook["upgrade"] {
  return upgrade === undefined || upgrade.state === "approved" || upgrade.state === "rolled_back"
    ? null
    : { id: upgrade.id, state: upgrade.state, started_by: upgrade.started_by.user_id, started_at: upgrade.started_at };
}

type UpgradeServerOptions = NonNullable<Parameters<typeof jobsServer>[0]> & {
  /** Plans' upgrade under way, if any. */
  upgrade?: Upgrade;
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
 * Plans as the list holds it carries the upgrade (withUpgrade keeps the
 * two together). It answers no capacity nor time out (server_busy) and
 * no stale index, which a test answers through answers.
 */
export function upgradeServer({ upgrade, answers = {}, ...options }: UpgradeServerOptions = {}) {
  const routes: Record<string, Answer> = {
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
        : json(server.upgrade);
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
  };
  const server = Object.assign(jobsServer({ ...options, answers: { ...routes, ...answers } }), {
    /** Plans' upgrade: under way, or the last one, ended; none before its first. */
    upgrade: undefined as Upgrade | undefined,
    /** withUpgrade gives Plans upgrade, or none, in its read and in the notebook the list holds. */
    withUpgrade(next: Upgrade | undefined): void {
      server.upgrade = next;
      server.notebook = { ...server.notebook, upgrade: summaryOf(next) };
    },
  });
  server.withUpgrade(upgrade);
  return server;
}

/** bobsUpgrade is Plans' upgrade under way started by Bob, another of its admins. */
export function bobsUpgrade(more: Partial<Upgrade> = {}): Upgrade {
  return upgradeJSON({ started_by: bob, ...more });
}
