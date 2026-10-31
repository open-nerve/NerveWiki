import type { LintItem, LintPage, LintRuleSummary, LintSummary } from "@nervewiki/api-client";

import { type Answer, json, problem } from "./fakes";

type Rule = LintItem["rule"];

/** Already computed facts, in each rule's order; the fake does no rule evaluation. */
export interface LintFacts extends Pick<LintSummary, "mode" | "computed_at" | "stale_pages"> {
  rules: { [R in Rule]?: Extract<LintItem, { rule: R }>[] };
  busy?: boolean;
}

const rules: Rule[] = [
  "wanted",
  "orphans",
  "ambiguous",
  "invalid_frontmatter",
  "missing_type",
  "missing_summary",
  "undigested_sources",
  "invalid_sources",
];
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const hash = /^[0-9a-f]{64}$/;

function isRule(value: unknown): value is Rule {
  return typeof value === "string" && rules.some((rule) => rule === value);
}

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function base64(raw: string): string {
  return btoa(raw).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

function unbase64(raw: string): string | undefined {
  if (!/^[A-Za-z0-9_-]+$/.test(raw)) return undefined;
  try {
    const text = atob(raw.replaceAll("-", "+").replaceAll("_", "/"));
    return base64(text) === raw ? text : undefined;
  } catch {
    return undefined;
  }
}

/** UTC RFC3339Nano, including its sub-millisecond ordering. */
function timeKey(raw: string): string | undefined {
  const match = /^(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.(\d{1,9}))?Z$/.exec(raw);
  if (!match?.[1] || match[2]?.endsWith("0")) return undefined;
  const at = new Date(`${match[1]}Z`);
  if (!Number.isFinite(at.getTime()) || at.toISOString().slice(0, 19) !== match[1]) return undefined;
  return match[1] + (match[2] ?? "").padEnd(9, "0");
}

interface Cursor {
  rule: Rule;
  notebook: string;
  scope: string;
  begin: boolean;
  pages: bigint;
  prefix: string;
  hash: string;
  id: string;
  at: string;
}

/** Decode the wire shape only; binding belongs before notebook lookup, for known rules only. */
function cursorOf(raw: string): Cursor | undefined {
  if (raw.length > 2048) return undefined;
  const text = unbase64(raw);
  if (text === undefined) return undefined;
  try {
    const outer: unknown = JSON.parse(text);
    if (!record(outer) || outer.v !== 1 || !record(outer.p)) return undefined;
    const p = outer.p;
    if (p.v !== 1 || !isRule(p.r) || typeof p.n !== "string" || !uuid.test(p.n) || typeof p.s !== "string") {
      return undefined;
    }
    const scope = p.s.split(":");
    if (
      p.s !== "I" &&
      !(
        scope.length === 3 &&
        scope[0] === "E" &&
        (scope[1] === "standard" || scope[1] === "llm_wiki") &&
        (uuid.test(scope[2] ?? "") || (scope[1] === "standard" && scope[2] === ""))
      )
    )
      return undefined;
    if (
      (p.c !== undefined && (typeof p.c !== "number" || !Number.isInteger(p.c) || p.c < 0)) ||
      ["k", "h", "i", "t"].some((key) => p[key] !== undefined && typeof p[key] !== "string") ||
      (p.b !== undefined && typeof p.b !== "boolean")
    )
      return undefined;
    // Keep the integer spelling: JSON.parse alone rounds valid int64 counts.
    const countText = text.match(/,"c":([1-9]\d*)[,}]/)?.[1];
    const pages = countText === undefined ? 0n : BigInt(countText);
    if (pages > 9223372036854775807n || (p.c !== undefined && p.c !== 0 && pages === 0n)) return undefined;
    const fields: [string, unknown][] = [
      ["v", p.v],
      ["r", p.r],
      ["n", p.n],
      ["s", p.s],
    ];
    for (const key of ["c", "k", "h", "i", "t", "b"]) {
      if (p[key]) fields.push([key, p[key]]);
    }
    const spelling = `{"v":1,"p":{${fields
      .map(([key, value]) => `${JSON.stringify(key)}:${key === "c" ? countText : JSON.stringify(value)}`)
      .join(",")}}}`;
    if (text !== spelling) return undefined;
    const k = typeof p.k === "string" ? p.k : "";
    const h = typeof p.h === "string" ? p.h : "";
    const i = typeof p.i === "string" ? p.i : "";
    const t = typeof p.t === "string" ? p.t : "";
    const prefix = unbase64(k) ?? "";
    if (p.b === true) {
      if (pages !== 0n || k !== "" || h !== "" || i !== "" || t !== "") return undefined;
    } else if (p.r === "wanted") {
      if (pages <= 0n || prefix.length < 1 || prefix.length > 256 || !hash.test(h) || i !== "" || t !== "") {
        return undefined;
      }
    } else {
      if (pages !== 0n || k !== "" || !uuid.test(i)) return undefined;
      if (p.r === "ambiguous" || p.r === "invalid_sources") {
        if (!hash.test(h) || t !== "") return undefined;
      } else if (p.r === "undigested_sources") {
        if (h !== "" || timeKey(t) === undefined) return undefined;
      } else if (h !== "" || t !== "") return undefined;
    }
    return { rule: p.r, notebook: p.n, scope: p.s, begin: p.b === true, pages, prefix, hash: h, id: i, at: t };
  } catch {
    return undefined;
  }
}

function compare(a: string | bigint, b: string | bigint): number {
  return a === b ? 0 : a < b ? -1 : 1;
}

function after(item: LintItem, position: Cursor): boolean {
  if (position.begin) return true;
  const next = cursorOf(item.cursor);
  if (!next || next.begin) throw new Error("Lint fixture needs an actual item cursor");
  if (next.rule === "wanted") {
    return (
      (compare(position.pages, next.pages) ||
        compare(next.prefix, position.prefix) ||
        compare(next.hash, position.hash)) > 0
    );
  }
  if (next.rule === "undigested_sources") {
    return (compare(timeKey(next.at) ?? "", timeKey(position.at) ?? "") || compare(next.id, position.id)) > 0;
  }
  return (compare(next.id, position.id) || compare(next.hash, position.hash)) > 0;
}

function ruleSummary(rule: Rule, facts: LintFacts): LintRuleSummary {
  const all = facts.rules[rule] ?? [];
  const items = all.slice(0, 5);
  // The mapped facts type binds each item branch to this rule.
  return {
    rule,
    count: Math.min(all.length, 10000),
    count_capped: all.length >= 10000,
    items,
    next_cursor: all.length > items.length ? (items.at(-1)?.cursor ?? null) : null,
  } as LintRuleSummary;
}

function busy(): Response {
  return problem(503, "server_busy", {}, { "Retry-After": "1" });
}

/** Compose with byRoute. Absent notebooks are invisible; callers may replace facts between requests. */
export function lintRoutes(notebooks: Map<string, LintFacts>): Record<string, Answer> {
  return {
    "GET /api/v0/notebooks/*/lint": (request) => {
      const notebook = new URL(request.url).pathname.split("/")[4]?.toLowerCase() ?? "";
      if (!uuid.test(notebook)) return problem(400, "bad_request");
      const facts = notebooks.get(notebook);
      if (!facts) return problem(404, "notebook.not_found");
      if (facts.busy) return busy();
      const summary: LintSummary = {
        mode: facts.mode,
        computed_at: facts.computed_at,
        stale_pages: facts.stale_pages,
        rules: rules.slice(0, facts.mode === "standard" ? 4 : 8).map((rule) => ruleSummary(rule, facts)),
      };
      return json(summary);
    },
    "GET /api/v0/notebooks/*/lint/*": (request) => {
      const url = new URL(request.url);
      const notebook = url.pathname.split("/")[4]?.toLowerCase() ?? "";
      const rule = decodeURIComponent(url.pathname.split("/")[6] ?? "");
      const limitText = url.searchParams.get("limit");
      if (!uuid.test(notebook) || (limitText !== null && !/^[+-]?\d+$/.test(limitText))) {
        return problem(400, "bad_request");
      }
      const limit = limitText === null ? 50n : BigInt(limitText);
      if (limit < -9223372036854775808n || limit > 9223372036854775807n) return problem(400, "bad_request");
      const raw = url.searchParams.get("cursor");
      const position = raw === null ? undefined : cursorOf(raw);
      if (raw !== null && !position) return problem(400, "bad_request");
      if (
        position &&
        isRule(rule) &&
        (position.notebook !== notebook || position.rule !== rule || position.scope !== "I")
      ) {
        return problem(400, "bad_request");
      }
      const facts = notebooks.get(notebook);
      if (!facts) return problem(404, "notebook.not_found");
      if (!isRule(rule)) return problem(404, "wiki.lint_rule_not_found");
      if (facts.mode === "standard" && rules.indexOf(rule) >= 4) return problem(409, "notebook.not_llm_wiki");
      if (limit < 1n || limit > 100n) {
        return problem(422, "validation_failed", {
          errors: [{ field: "limit", code: "out_of_range", message: "must be between 1 and 100" }],
        });
      }
      if (facts.busy) return busy();
      const all = (facts.rules[rule] ?? []).filter((item) => !position || after(item, position));
      const data = all.slice(0, Number(limit));
      const page: LintPage = {
        mode: facts.mode,
        computed_at: facts.computed_at,
        stale_pages: facts.stale_pages,
        data,
        next_cursor: all.length > data.length ? (data.at(-1)?.cursor ?? null) : null,
      };
      return json(page);
    },
  };
}
