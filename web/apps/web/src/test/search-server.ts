import type { Notebook } from "../services/notebook.service";
import type { SearchCount, SearchHit, TextRange, TreeNode } from "../services/page.service";
import { json, notebookJSON, problem, type Answer } from "./fakes";

// The search of the page server (page-server.ts; M8/P5), as the server
// answers it, its semantics made simple: Plans' nodes and those of the
// other notebooks the account sees (others, listed with Plans), each page's
// content what contents has, by default its title on a line. A term is
// found as a substring in any case, a word run of one or two letters or
// digits at its ends as a whole word (a CJK character bounds it); a term
// whose one keyed run is a single CJK character, and, in a query of no key
// of the body, one of single CJK characters alone, in names alone. The
// title tier lists the nodes whose name holds every term; the body tier
// the pages whose name does not, holding the titles-only ones, whose name
// or content holds each other term, walking searchWalk candidates a page
// at most (no bound by default): a page may hold fewer than limit, none
// even, with a cursor below the last walked. Each tier newest first, by
// updated_at then id. The first page's count: exact without a next page;
// else none while none passed, about the hits there are, 1,000 at least
// past it, unless searchCount says otherwise; building is searchBuilding.
// The codes in the server's order (page/adapter/http/search.go,
// page/app/search.go): the binding's 400 bad_request, a notebook_id that
// is no UUID or a limit no integer; a cursor it cannot read 400
// bad_request; q 422 (missing or blank required, NUL invalid_format, past
// 256 bytes trimmed, 8 terms or 64 ASCII letters and digits in a row
// too_long, no letter, digit or CJK character not_allowed); a workspace
// other than lab 404 workspace.not_found; a notebook_id of none it sees
// 404 notebook.not_found; a limit outside 1–50 422 out_of_range. While
// searchDown is set, a search cannot be read; searchWaits holds its reads.

/** What the search's reads wait for while a test holds them: a first page's, a next page's (after a cursor). */
export type SearchWaits = { first?: Promise<void>; next?: Promise<void> };

/** What of the page server the search reads. */
type SearchState = {
  sent: string[];
  nodes: TreeNode[];
  contents: Map<string, { content: string; revision: number }>;
  /** The notebooks other than Plans the account sees, and their nodes. */
  others: Notebook[];
  otherNodes: TreeNode[];
  searchDown: boolean;
  searchWaits: SearchWaits;
  /** The body tier's candidates a page walks at most. */
  searchWalk: number;
  /** The first page's count, when not as the fake counts it. */
  searchCount: SearchCount | null | undefined;
  searchBuilding: boolean;
};

/** An attachment's type and size, as the server answers them. */
type AssetOf = (node: TreeNode) => { mime: string; byte_size: number };

/** A tier's place: its tier (0 the names', 1 the contents'), then a hit's time and id, below which a page goes on. */
type Place = { tier: 0 | 1; at: string; id: string };

const uuidPattern = /^[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}$/i;
const cjk = String.raw`\p{sc=Han}\p{sc=Hiragana}\p{sc=Katakana}\p{sc=Hangul}`;
/** A character of a word run: a letter, a digit or a mark, but a CJK character. */
const wordChar = String.raw`(?:(?![${cjk}])[\p{L}\p{N}\p{M}])`;
const cjkChar = new RegExp(`^[${cjk}]$`, "u");
const wordCharOnly = new RegExp(`^${wordChar}$`, "u");

/** A term of a query: its pattern, and whether it is found in names alone. */
type Term = { pattern: RegExp; titlesOnly: boolean; keys: boolean; unigrams: number };

/** classOf is a character's class, as the server's: a CJK character, one of a word run, or another. */
function classOf(char: string): "cjk" | "word" | "other" {
  return cjkChar.test(char) ? "cjk" : wordCharOnly.test(char) ? "word" : "other";
}

/**
 * termOf is the term text: its pattern, with a word boundary at an end that is a word run of one or two
 * characters; undefined for one of a run of more than 64 ASCII letters and digits.
 */
function termOf(text: string): Term | undefined {
  const runs: { kind: "cjk" | "word" | "other"; n: number; ascii: boolean }[] = [];
  for (const char of text) {
    const kind = classOf(char);
    const last = runs.at(-1);
    if (last !== undefined && last.kind === kind && kind !== "other") {
      last.n++;
      last.ascii &&= char.length === 1 && char.charCodeAt(0) < 0x80;
    } else {
      runs.push({ kind, n: 1, ascii: char.length === 1 && char.charCodeAt(0) < 0x80 });
    }
  }
  if (runs.some((run) => run.kind === "word" && run.ascii && run.n > 64)) {
    return undefined;
  }
  const keyed = runs.filter((run) => run.kind !== "other");
  const keys = keyed.some((run) => run.kind === "word" || run.n > 1);
  const unigrams = keyed.filter((run) => run.kind === "cjk" && run.n === 1).length;
  const first = runs[0];
  const last = runs.at(-1);
  const left = first?.kind === "word" && first.n <= 2 ? `(?<!${wordChar})` : "";
  const right = last?.kind === "word" && last.n <= 2 ? `(?!${wordChar})` : "";
  const escaped = text.replaceAll(/[.*+?^${}()|[\]\\/]/g, String.raw`\$&`);
  return {
    pattern: new RegExp(`${left}${escaped}${right}`, "giu"),
    titlesOnly: keyed.length === 1 && !keys && unigrams === 1,
    keys,
    unigrams,
  };
}

/** queryOf is q's terms, or its 422's field code. */
function queryOf(q: string | null): Term[] | "required" | "invalid_format" | "too_long" | "not_allowed" {
  if (q === null) {
    return "required";
  }
  if (q.includes("\0")) {
    return "invalid_format";
  }
  const trimmed = q.trim();
  if (trimmed === "") {
    return "required";
  }
  if (new TextEncoder().encode(trimmed).length > 256) {
    return "too_long";
  }
  // Terms of one folded form are one: the first.
  const texts = trimmed
    .split(/\s+/u)
    .filter((text, i, all) => all.findIndex((each) => each.toLowerCase() === text.toLowerCase()) === i);
  if (texts.length > 8) {
    return "too_long";
  }
  const terms: Term[] = [];
  for (const text of texts) {
    const term = termOf(text);
    if (term === undefined) {
      return "too_long";
    }
    terms.push(term);
  }
  if (!terms.some((term) => term.keys || term.unigrams > 0)) {
    return "not_allowed";
  }
  if (!terms.some((term) => term.keys)) {
    return terms.map((term) => ({ ...term, titlesOnly: term.unigrams > 0 }));
  }
  return terms;
}

/** rangesOf are where the terms are found in text, sorted, those that overlap made one. */
function rangesOf(text: string, terms: readonly Term[]): TextRange[] {
  const found = terms
    .flatMap((term) =>
      [...text.matchAll(term.pattern)].map((match) => ({ start: match.index, end: match.index + match[0].length }))
    )
    .toSorted((a, b) => a.start - b.start || a.end - b.end);
  const ranges: TextRange[] = [];
  for (const range of found) {
    const last = ranges.at(-1);
    if (last !== undefined && range.start < last.end) {
      last.end = Math.max(last.end, range.end);
    } else {
      ranges.push({ ...range });
    }
  }
  return ranges;
}

/** holds tells whether text holds term. */
function holds(text: string, term: Term): boolean {
  term.pattern.lastIndex = 0;
  return term.pattern.test(text);
}

/** below tells whether node is below place in its tier's order, newest first, then by id. */
function below(node: TreeNode, place: Place | undefined): boolean {
  return place === undefined || node.updated_at < place.at || (node.updated_at === place.at && node.id < place.id);
}

/** newestFirst orders nodes as a tier lists them. */
function newestFirst(a: TreeNode, b: TreeNode): number {
  return a.updated_at === b.updated_at ? (a.id < b.id ? 1 : -1) : a.updated_at < b.updated_at ? 1 : -1;
}

/** cursorOf is the cursor of place; placeOf reads one back, undefined for one it cannot read. */
function cursorOf({ tier, at, id }: Place): string {
  return `${tier.toString()} ${at} ${id}`;
}

function placeOf(cursor: string): Place | undefined {
  const [tier, at, id, ...rest] = cursor.split(" ");
  return (tier === "0" || tier === "1") &&
    at !== undefined &&
    !Number.isNaN(Date.parse(at)) &&
    id !== undefined &&
    rest.length === 0
    ? { tier: tier === "0" ? 0 : 1, at, id }
    : undefined;
}

/** invalid is the 422 of field's code. */
function invalid(field: string, code: string): Response {
  return problem(422, "validation_failed", { errors: [{ field, code, message: code }] });
}

/** The answer to the search of server's workspace, lab's. */
export function searchRoutes(server: SearchState, assetOf: AssetOf): Record<string, Answer> {
  return {
    "GET /api/v0/workspaces/*/search": async (request) => {
      const url = new URL(request.url);
      const query = url.searchParams;
      const q = query.get("q");
      const notebookId = query.get("notebook_id");
      const cursor = query.get("cursor");
      const limitText = query.get("limit");
      const notebooks = [notebookJSON, ...server.others];
      const named = notebooks.find((each) => each.id === notebookId?.toLowerCase());
      server.sent.push(
        `GET search ${q ?? ""}${named === undefined ? "" : ` in ${named.name}`}${cursor === null ? "" : " more"}`
      );
      await (cursor === null ? server.searchWaits.first : server.searchWaits.next);
      if (server.searchDown) {
        return problem(500, "internal");
      }
      if (
        (notebookId !== null && !uuidPattern.test(notebookId)) ||
        (limitText !== null && !/^-?\d+$/.test(limitText))
      ) {
        return problem(400, "bad_request");
      }
      const from = cursor === null ? undefined : placeOf(cursor);
      if (cursor !== null && from === undefined) {
        return problem(400, "bad_request");
      }
      const terms = queryOf(q);
      if (typeof terms === "string") {
        return invalid("q", terms);
      }
      if (url.pathname.split("/")[4] !== "lab") {
        return problem(404, "workspace.not_found");
      }
      if (notebookId !== null && named === undefined) {
        return problem(404, "notebook.not_found");
      }
      const limit = limitText === null ? 20 : Number(limitText);
      if (limit < 1 || limit > 50) {
        return invalid("limit", "out_of_range");
      }
      const scope = new Set(named === undefined ? notebooks.map((each) => each.id) : [named.id]);
      const all = [...server.nodes, ...server.otherNodes].filter((node) => scope.has(node.notebook_id));
      const contentOf = (node: TreeNode) => server.contents.get(node.id)?.content ?? `${node.name}\n`;
      const titled = (node: TreeNode) => terms.every((term) => holds(node.name, term));
      const candidate = (node: TreeNode) =>
        node.kind === "page" && !titled(node) && terms.every((term) => !term.titlesOnly || holds(node.name, term));
      const passes = (node: TreeNode) =>
        terms.every((term) => term.titlesOnly || holds(node.name, term) || holds(contentOf(node), term));
      const bodied = terms.some((term) => !term.titlesOnly);
      const rows: { node: TreeNode; tier: 0 | 1 }[] = [];
      if (from === undefined || from.tier === 0) {
        const titles = all.filter((node) => titled(node) && below(node, from)).toSorted(newestFirst);
        rows.push(...titles.slice(0, limit + 1).map((node) => ({ node, tier: 0 as const })));
      }
      let walked: TreeNode | undefined;
      let unwalked = false;
      if (rows.length <= limit && bodied) {
        const pool = all
          .filter((node) => candidate(node) && below(node, from?.tier === 1 ? from : undefined))
          .toSorted(newestFirst);
        for (const node of pool.slice(0, server.searchWalk)) {
          walked = node;
          if (passes(node)) {
            rows.push({ node, tier: 1 });
            if (rows.length > limit) {
              break;
            }
          }
        }
        unwalked = pool.length > server.searchWalk;
      }
      let next: Place | undefined;
      const last = rows[limit - 1];
      if (rows.length > limit && last !== undefined) {
        next = { tier: last.tier, at: last.node.updated_at, id: last.node.id };
        rows.splice(limit);
      } else if (unwalked && walked !== undefined) {
        next = { tier: 1, at: walked.updated_at, id: walked.id };
      }
      let count: SearchCount | null = null;
      if (server.searchCount !== undefined) {
        count = server.searchCount;
      } else if (next === undefined) {
        count = { value: rows.length, kind: "exact" };
      } else if (rows.length > 0) {
        const total = all.filter((node) => titled(node) || (bodied && candidate(node) && passes(node))).length;
        count = total > 1000 ? { value: 1000, kind: "at_least" } : { value: total, kind: "about" };
      }
      return json({
        data: rows.map(({ node }) =>
          hitOf(node, [...server.nodes, ...server.otherNodes], contentOf(node), terms, assetOf)
        ),
        next_cursor: next === undefined ? null : cursorOf(next),
        count: from === undefined ? count : null,
        building: server.searchBuilding,
      });
    },
  };
}

/** hitOf is the hit of node, of content, among nodes, as the server answers it: its excerpt around its first hit. */
function hitOf(
  node: TreeNode,
  nodes: readonly TreeNode[],
  content: string,
  terms: readonly Term[],
  assetOf: AssetOf
): SearchHit {
  const ancestors: { id: string; name: string }[] = [];
  for (let at = node.parent_id; at !== null;) {
    const parent = nodes.find((each) => each.id === at);
    if (parent === undefined) {
      break;
    }
    ancestors.unshift({ id: parent.id, name: parent.name });
    at = parent.parent_id;
  }
  const first = node.kind === "page" ? rangesOf(content, terms)[0] : undefined;
  let excerpt: SearchHit["excerpt"] = null;
  if (first !== undefined) {
    const start = Math.max(first.start - 40, 0);
    const text = content.slice(start, start + 162);
    excerpt = { text, hits: rangesOf(text, terms), cut_start: start > 0, cut_end: start + 162 < content.length };
  }
  const asset = node.kind === "asset" ? assetOf(node) : undefined;
  return {
    node: { id: node.id, notebook_id: node.notebook_id, kind: node.kind, name: node.name },
    ancestors,
    title_hits: rangesOf(node.name, terms),
    excerpt,
    changed_at: node.updated_at,
    attachment: asset === undefined ? null : { mime: asset.mime, byte_size: asset.byte_size },
  };
}
