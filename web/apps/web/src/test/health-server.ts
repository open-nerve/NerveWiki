import type { LintPage } from "@nervewiki/api-client";

import { json, notebookJSON, type Answer } from "./fakes";
import { lintRoutes, type LintFacts } from "./lint-server";
import { assetNode, guide, install, linux, notes, pageNode, pageServer } from "./page-server";

export const healthHome = `/lab/notebooks/${notebookJSON.id}`;
export const healthPath = `${healthHome}/health`;
const healthComputed = "2026-10-31T09:10:00Z";
export const healthRaw = pageNode(61, "raw");
export const healthFolder = pageNode(62, "Folder", healthRaw);
export const healthAsset = assetNode(63, "evidence.pdf", healthFolder);

export function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}

function wire(text: string): string {
  return btoa(text).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

/** Independent literal wire facts: no UI sorting, projection or production cursor encoder. */
export function wantedFacts(count = 65, sharedTarget?: string): NonNullable<LintFacts["rules"]["wanted"]> {
  return Array.from({ length: count }, (_, index) => {
    const target = sharedTarget ?? `Missing ${String(index + 1).padStart(3, "0")}`;
    const hash = (index + 1).toString(16).padStart(64, "0");
    return {
      rule: "wanted",
      item_key: `w:${hash}`,
      target,
      target_cut: sharedTarget !== undefined,
      pages: 1,
      cursor: wire(
        JSON.stringify({
          v: 1,
          p: {
            v: 1,
            r: "wanted",
            n: notebookJSON.id,
            s: "I",
            c: 1,
            k: wire(String.fromCharCode(...new TextEncoder().encode(target))),
            h: hash,
          },
        })
      ),
    };
  });
}

export function healthServer(answers: Record<string, Answer> = {}) {
  const wikiPages = Array.from({ length: 6 }, (_, index) => pageNode(100 + index, `Wiki ${index + 1}`));
  const value: LintFacts = {
    mode: "llm_wiki",
    computed_at: healthComputed,
    stale_pages: 0,
    rules: {
      wanted: wantedFacts(),
      missing_summary: wikiPages.map((node) => ({
        rule: "missing_summary",
        item_key: `p:${node.id}`,
        page_id: node.id,
        cursor: wire(
          JSON.stringify({ v: 1, p: { v: 1, r: "missing_summary", n: notebookJSON.id, s: "I", i: node.id } })
        ),
      })),
    },
  };
  const facts = new Map([[notebookJSON.id, value]]);
  const routes = lintRoutes(facts);
  const summary = routes["GET /api/v0/notebooks/*/lint"];
  const page = routes["GET /api/v0/notebooks/*/lint/*"];
  if (summary === undefined || page === undefined) throw new Error("lint routes are missing");
  const responses: { summary: Answer; page: Answer } = { summary, page };
  const summaries: Request[] = [];
  const pages: Request[] = [];
  const server = pageServer({
    role: "reader",
    mode: "llm_wiki",
    sourceRootId: healthRaw.id,
    nodes: [guide, install, linux, notes, healthRaw, healthFolder, healthAsset, ...wikiPages],
    answers: {
      ...routes,
      "GET /api/v0/notebooks/*/lint": (request) => {
        summaries.push(request);
        return responses.summary(request);
      },
      "GET /api/v0/notebooks/*/lint/*": (request) => {
        pages.push(request);
        return responses.page(request);
      },
      ...answers,
    },
  });
  function pageOf(data: LintPage["data"], next: string | null = null, mode: LintPage["mode"] = value.mode): Response {
    return json({ mode, computed_at: value.computed_at, stale_pages: value.stale_pages, data, next_cursor: next });
  }
  return { server, value, facts, responses, summaries, pages, pageOf, summary, page };
}
