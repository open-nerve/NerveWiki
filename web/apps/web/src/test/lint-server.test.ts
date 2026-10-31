import type { ApiClient, LintItem, LintPage, LintSummary, LintWantedItem, paths } from "@nervewiki/api-client";
import { describe, expect, test } from "vitest";

import { ApiError, unwrap } from "../services/api";
import { byRoute, fakeApi } from "./fakes";
import { type LintFacts, lintRoutes } from "./lint-server";

const summaryPath = "/api/v0/notebooks/{notebook_id}/lint" satisfies keyof paths;
const itemsPath = "/api/v0/notebooks/{notebook_id}/lint/{rule}" satisfies keyof paths;
const llm = "0199a2b4-0000-7000-8000-000000000001";
const standard = "0199a2b4-0000-7000-8000-000000000002";
const hidden = "0199a2b4-0000-7000-8000-000000000003";
const zero = "00000000-0000-0000-0000-000000000000";
const one = "00000000-0000-0000-0000-000000000001";
const two = "00000000-0000-0000-0000-000000000002";
const at = "2026-10-30T01:02:03Z";
const hashA = "a".repeat(64);
const hashB = "b".repeat(64);
const display = "界".repeat(85) + "x";
const displayKey = wire(String.fromCharCode(...new TextEncoder().encode(display)));
const order = [
  "wanted",
  "orphans",
  "ambiguous",
  "invalid_frontmatter",
  "missing_type",
  "missing_summary",
  "undigested_sources",
  "invalid_sources",
] as const;

// These are hand-written wire fixtures, independent of the fake's private decoder.
function wire(text: string): string {
  return btoa(text).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

function cursor(rule: LintItem["rule"], position: Record<string, unknown>, notebook = llm, scope = "I"): string {
  return wire(JSON.stringify({ v: 1, p: { v: 1, r: rule, n: notebook, s: scope, ...position } }));
}

function wanted(h: string): LintWantedItem {
  return {
    rule: "wanted",
    item_key: `w:${h}`,
    cursor: cursor("wanted", { c: 2, k: displayKey, h }),
    target: display,
    target_cut: true,
    pages: 2,
  };
}

function orphan(id: string, notebook = llm): Extract<LintItem, { rule: "orphans" }> {
  return { rule: "orphans", item_key: `p:${id}`, cursor: cursor("orphans", { i: id }, notebook), page_id: id };
}

function fixture() {
  const facts: LintFacts = {
    mode: "llm_wiki",
    computed_at: at,
    stale_pages: 7,
    rules: {
      wanted: [wanted(hashA), wanted(hashB)],
      orphans: [orphan(zero), orphan(one)],
      ambiguous: [
        {
          rule: "ambiguous",
          item_key: `a:${one}:${hashA}`,
          cursor: cursor("ambiguous", { h: hashA, i: one }),
          page_id: one,
          target: "Shared",
          target_cut: false,
        },
      ],
      invalid_frontmatter: [
        {
          rule: "invalid_frontmatter",
          item_key: `p:${one}`,
          cursor: cursor("invalid_frontmatter", { i: one }),
          page_id: one,
        },
      ],
      missing_type: [
        { rule: "missing_type", item_key: `p:${one}`, cursor: cursor("missing_type", { i: one }), page_id: one },
      ],
      missing_summary: [
        { rule: "missing_summary", item_key: `p:${two}`, cursor: cursor("missing_summary", { i: two }), page_id: two },
      ],
      undigested_sources: [
        {
          rule: "undigested_sources",
          item_key: `n:${zero}`,
          cursor: cursor("undigested_sources", { i: zero, t: at }),
          node_id: zero,
          kind: "asset",
          bytes: 0,
          created_at: at,
        },
        {
          rule: "undigested_sources",
          item_key: `n:${one}`,
          cursor: cursor("undigested_sources", { i: one, t: "2026-10-30T01:02:03.000000001Z" }),
          node_id: one,
          kind: "page",
          bytes: 17,
          created_at: "2026-10-30T01:02:03.000000001Z",
        },
      ],
      invalid_sources: [
        {
          rule: "invalid_sources",
          item_key: `s:${one}:${hashB}`,
          cursor: cursor("invalid_sources", { h: hashB, i: one }),
          page_id: one,
          target: "",
          target_cut: false,
          not_link: false,
        },
      ],
    },
  };
  const notebooks = new Map<string, LintFacts>([
    [llm, facts],
    [standard, { mode: "standard", computed_at: at, stale_pages: 0, rules: { orphans: [orphan(zero, standard)] } }],
  ]);
  return { facts, notebooks, client: fakeApi(byRoute(lintRoutes(notebooks))) };
}

function summary(client: ApiClient, notebook = llm) {
  return client.GET(summaryPath, { params: { path: { notebook_id: notebook } } });
}

function items(client: ApiClient, rule: string, query: { limit?: number; cursor?: string } = {}, notebook = llm) {
  return client.GET(itemsPath, { params: { path: { notebook_id: notebook, rule }, query } });
}

test("both generated GETs preserve both modes, every nonempty branch and exact item values", async () => {
  const { client, facts, notebooks } = fixture();
  const expectedRules = structuredClone(facts.rules);
  const before = structuredClone([...notebooks]);
  const result: LintSummary = unwrap(await summary(client));
  expect(result).toEqual({
    mode: "llm_wiki",
    computed_at: at,
    stale_pages: 7,
    rules: order.map((rule) => ({
      rule,
      count: expectedRules[rule]?.length,
      count_capped: false,
      items: expectedRules[rule],
      next_cursor: null,
    })),
  });
  expect(result.rules.map((rule) => rule.rule)).toEqual([
    "wanted",
    "orphans",
    "ambiguous",
    "invalid_frontmatter",
    "missing_type",
    "missing_summary",
    "undigested_sources",
    "invalid_sources",
  ]);
  await Promise.all(
    order.map(async (rule) => {
      const page: LintPage = unwrap(await items(client, rule));
      expect(page, rule).toEqual({
        mode: "llm_wiki",
        computed_at: at,
        stale_pages: 7,
        data: expectedRules[rule],
        next_cursor: null,
      });
      expect(page.data.length, rule).toBeGreaterThan(0);
    })
  );
  expect(result.rules.find((rule) => rule.rule === "invalid_sources")?.items[0]).toMatchObject({
    not_link: false,
    target: "",
    target_cut: false,
  });
  expect(result.rules.find((rule) => rule.rule === "undigested_sources")?.items[0]).toMatchObject({
    node_id: "00000000-0000-0000-0000-000000000000",
    kind: "asset",
    bytes: 0,
    created_at: "2026-10-30T01:02:03Z",
  });
  const ordinary = unwrap(await summary(client, standard));
  expect(ordinary).toEqual({
    mode: "standard",
    computed_at: at,
    stale_pages: 0,
    rules: [
      { rule: "wanted", count: 0, count_capped: false, items: [], next_cursor: null },
      { rule: "orphans", count: 1, count_capped: false, items: [orphan(zero, standard)], next_cursor: null },
      { rule: "ambiguous", count: 0, count_capped: false, items: [], next_cursor: null },
      { rule: "invalid_frontmatter", count: 0, count_capped: false, items: [], next_cursor: null },
    ],
  });
  expect(unwrap(await items(client, "orphans", {}, standard))).toEqual({
    mode: "standard",
    computed_at: at,
    stale_pages: 0,
    data: [orphan(zero, standard)],
    next_cursor: null,
  });
  expect(unwrap(await items(client, "wanted", {}, standard))).toEqual({
    mode: "standard",
    computed_at: at,
    stale_pages: 0,
    data: [],
    next_cursor: null,
  });
  expect([...notebooks]).toEqual(before);
});

test("same displayed targets retain distinct complete keys and per-item continuation", async () => {
  const { client } = fixture();
  const first = unwrap(await items(client, "wanted", { limit: 1 }));
  expect(new TextEncoder().encode(display)).toHaveLength(256);
  expect(first).toEqual({
    mode: "llm_wiki",
    computed_at: at,
    stale_pages: 7,
    data: [wanted(hashA)],
    next_cursor: cursor("wanted", { c: 2, k: displayKey, h: hashA }),
  });
  const last = unwrap(await items(client, "wanted", { limit: 1, cursor: first.next_cursor ?? "missing" }));
  expect(last).toEqual({ mode: "llm_wiki", computed_at: at, stale_pages: 7, data: [wanted(hashB)], next_cursor: null });
  expect([first.data[0]?.item_key, last.data[0]?.item_key]).toEqual([`w:${hashA}`, `w:${hashB}`]);
});

test("summary preview, true Begin, middle pages and the last page advance past the actual item", async () => {
  const { client, facts } = fixture();
  const ids = [zero, one, two, ...[3, 4, 5, 6].map((n) => `00000000-0000-0000-0000-${n.toString().padStart(12, "0")}`)];
  facts.rules.orphans = ids.map((id) => orphan(id));
  const preview = unwrap(await summary(client)).rules.find((rule) => rule.rule === "orphans");
  expect(preview).toEqual({
    rule: "orphans",
    count: 7,
    count_capped: false,
    items: ids.slice(0, 5).map((id) => orphan(id)),
    next_cursor: cursor("orphans", { i: ids[4] }),
  });
  const fromPreview = unwrap(await items(client, "orphans", { limit: 1, cursor: preview?.next_cursor ?? "missing" }));
  expect(fromPreview.data.map((item) => item.item_key)).toEqual([`p:${ids[5]}`]);
  expect(fromPreview.next_cursor).toBe(cursor("orphans", { i: ids[5] }));
  const last = unwrap(await items(client, "orphans", { limit: 1, cursor: fromPreview.next_cursor ?? "missing" }));
  expect(last.data.map((item) => item.item_key)).toEqual([`p:${ids[6]}`]);
  expect(last.next_cursor).toBeNull();
  const start = unwrap(await items(client, "orphans", { limit: 1, cursor: cursor("orphans", { b: true }) }));
  expect(start.data).toEqual([orphan(zero)]);
  expect(start.next_cursor).toBe(cursor("orphans", { i: zero }));
  const middle = unwrap(await items(client, "orphans", { limit: 2, cursor: start.next_cursor ?? "missing" }));
  expect(middle.data).toEqual([orphan(one), orphan(two)]);
  expect(middle.next_cursor).toBe(cursor("orphans", { i: two }));
  facts.rules.orphans = facts.rules.orphans.filter((item) => item.page_id !== two);
  const moved = unwrap(await items(client, "orphans", { limit: 1, cursor: middle.next_cursor ?? "missing" }));
  expect(moved.data.map((item) => item.item_key)).toEqual([`p:${ids[3]}`]);
});

test("supplied cursor positions keep wanted byte order and source nanosecond order", async () => {
  const { client, facts } = fixture();
  // A 256-byte prefix may stop within a UTF-8 character. Its final 0xcf sorts
  // before 0xd0, though the corresponding base64url suffixes zw and 0A do not.
  const a = {
    ...wanted(hashA),
    target: "a".repeat(255),
    cursor: cursor("wanted", { c: 2, k: wire("a".repeat(255) + "\xcf"), h: hashA }),
  };
  const b = {
    ...wanted(hashB),
    target: "a".repeat(255),
    cursor: cursor("wanted", { c: 2, k: wire("a".repeat(255) + "\xd0"), h: hashB }),
  };
  facts.rules.wanted = [a, b];
  expect(unwrap(await items(client, "wanted", { cursor: a.cursor })).data.map((item) => item.item_key)).toEqual([
    `w:${hashB}`,
  ]);
  const sources = unwrap(await items(client, "undigested_sources", { limit: 1 }));
  expect(sources.data).toEqual([
    {
      rule: "undigested_sources",
      item_key: `n:${zero}`,
      cursor: cursor("undigested_sources", { i: zero, t: at }),
      node_id: zero,
      kind: "asset",
      bytes: 0,
      created_at: at,
    },
  ]);
  expect(unwrap(await items(client, "undigested_sources", { cursor: sources.next_cursor ?? "missing" })).data).toEqual([
    {
      rule: "undigested_sources",
      item_key: `n:${one}`,
      cursor: cursor("undigested_sources", { i: one, t: "2026-10-30T01:02:03.000000001Z" }),
      node_id: one,
      kind: "page",
      bytes: 17,
      created_at: "2026-10-30T01:02:03.000000001Z",
    },
  ]);
  // An int64 position need not be a safe JavaScript integer to have a valid shape.
  const far = wire(
    `{"v":1,"p":{"v":1,"r":"wanted","n":"${llm}","s":"I","c":9223372036854775807,"k":"YQ","h":"${hashA}"}}`
  );
  expect(unwrap(await items(client, "wanted", { cursor: far })).data).toEqual([a, b]);
});

test("count caps at 10000 exactly and later items remain pageable with default and maximum limits", async () => {
  const { client, facts } = fixture();
  const rows = Array.from({ length: 10001 }, (_, n) =>
    orphan(`00000000-0000-0000-0000-${n.toString().padStart(12, "0")}`)
  );
  async function countAt(length: number, count: number, capped: boolean) {
    facts.rules.orphans = rows.slice(0, length);
    const rule = unwrap(await summary(client)).rules.find((each) => each.rule === "orphans");
    expect(rule).toMatchObject({ count, count_capped: capped });
    expect(rule?.items).toHaveLength(5);
  }
  // Each response must finish before replacing the mutable fact snapshot.
  await countAt(9999, 9999, false);
  await countAt(10000, 10000, true);
  await countAt(10001, 10000, true);
  const page = unwrap(await items(client, "orphans"));
  expect(page.data).toEqual(rows.slice(0, 50));
  expect(page.next_cursor).toBe(rows[49]?.cursor);
  const maximum = unwrap(await items(client, "orphans", { limit: 100 }));
  expect(maximum.data).toEqual(rows.slice(0, 100));
  expect(maximum.next_cursor).toBe(rows[99]?.cursor);
  const beyond = unwrap(await items(client, "orphans", { cursor: rows[9999]?.cursor }));
  expect(beyond.data.map((item) => item.item_key)).toEqual(["p:00000000-0000-0000-0000-000000010000"]);
  expect(beyond.next_cursor).toBeNull();
});

describe("REST error order", () => {
  test("known cursor binding precedes notebook visibility for notebook, rule and scope", async () => {
    const { client } = fixture();
    await Promise.all(
      [
        cursor("orphans", { b: true }, llm),
        cursor("wanted", { b: true }, hidden),
        cursor("orphans", { b: true }, hidden, "E:standard:"),
        cursor("orphans", { b: true }, hidden, `E:llm_wiki:${zero}`),
      ].map(async (foreign) => {
        const answer = await items(client, "orphans", { limit: 0, cursor: foreign }, hidden);
        expect(answer.response.status).toBe(400);
        expect(answer.error?.code).toBe("bad_request");
      })
    );
    const boundHidden = await items(
      client,
      "orphans",
      { limit: 0, cursor: cursor("orphans", { b: true }, hidden) },
      hidden
    );
    expect(boundHidden.response.status).toBe(404);
    expect(boundHidden.error?.code).toBe("notebook.not_found");
  });

  test("unknown rule binds no cursor and is disclosed only after notebook access", async () => {
    const { client } = fixture();
    const foreign = cursor("orphans", { b: true }, standard, `E:llm_wiki:${zero}`);
    await Promise.all(
      [
        [hidden, "notebook.not_found"],
        [llm, "wiki.lint_rule_not_found"],
      ].map(async ([notebook, code]) => {
        const answer = await items(client, "not-a-rule", { limit: 0, cursor: foreign }, notebook);
        expect(answer.response.status).toBe(404);
        expect(answer.error?.code).toBe(code);
      })
    );
    const malformed = await items(client, "not-a-rule", { limit: 0, cursor: "bad" }, hidden);
    expect(malformed.response.status).toBe(400);
    expect(malformed.error?.code).toBe("bad_request");
  });

  test("cursor, notebook, rule, mode, limit and busy are repaired in their actual order", async () => {
    const { client, notebooks } = fixture();
    const book = notebooks.get(standard);
    expect(book).toBeDefined();
    if (!book) throw new Error("missing standard fixture");
    book.busy = true;
    const cases = [
      [hidden, "not-a-rule", { cursor: "bad", limit: 0 }, 400, "bad_request"],
      [hidden, "not-a-rule", { limit: 0 }, 404, "notebook.not_found"],
      [standard, "not-a-rule", { limit: 0 }, 404, "wiki.lint_rule_not_found"],
      [standard, "missing_type", { limit: 0 }, 409, "notebook.not_llm_wiki"],
      [standard, "orphans", { limit: 0 }, 422, "validation_failed"],
      [standard, "orphans", { limit: 1 }, 503, "server_busy"],
    ] as const;
    await Promise.all(
      cases.map(async ([notebook, rule, query, status, code]) => {
        const answer = await items(client, rule, query, notebook);
        expect(answer.response.status, code).toBe(status);
        expect(answer.error?.code, code).toBe(code);
        if (status === 422)
          expect(answer.error?.errors).toEqual([
            { field: "limit", code: "out_of_range", message: "must be between 1 and 100" },
          ]);
      })
    );
    book.busy = false;
    expect(unwrap(await items(client, "orphans", { limit: 1 }, standard)).data).toEqual([orphan(zero, standard)]);
  });

  test("binder failures are 400 and both GETs expose busy Retry-After to the real API client", async () => {
    const { client, facts } = fixture();
    for (const answer of [
      await summary(client, "invalid-id"),
      await items(client, "not-a-rule", { limit: 1.5 }, hidden),
    ]) {
      expect(answer.response.status).toBe(400);
      expect(answer.error?.code).toBe("bad_request");
    }
    expect((await summary(client, hidden)).error?.code).toBe("notebook.not_found");
    expect((await items(client, "orphans", { limit: 101 })).error?.code).toBe("validation_failed");
    facts.busy = true;
    for (const answer of [await summary(client), await items(client, "orphans")]) {
      expect(answer.response.status).toBe(503);
      expect(answer.error?.code).toBe("server_busy");
      expect(answer.response.headers.get("Retry-After")).toBe("1");
      expect(() => unwrap<LintSummary | LintPage>(answer)).toThrow(ApiError);
      try {
        unwrap<LintSummary | LintPage>(answer);
      } catch (error) {
        expect(error).toMatchObject({ status: 503, retryAfter: 1, code: "server_busy" });
      }
    }
  });
});

test("strict cursor shape rejects alternate envelopes and mixed Begin or rule fields before access", async () => {
  const { client } = fixture();
  const valid = cursor("orphans", { i: zero });
  const payload = `{"v":1,"r":"orphans","n":"${llm}","s":"I","i":"${zero}"}`;
  const invalid = [
    "",
    "bad",
    valid + "=",
    valid + "\n",
    "a".repeat(2049),
    wire(`{"v":2,"p":${payload}}`),
    wire(`{"v":1,"p":${payload.replace('"v":1', '"v":2')}}`),
    wire(`{"v":1,"p":${payload.replace('"orphans"', '"unknown"')}}`),
    wire(`{"v":1,"p":null}`),
    wire(`{"v":1,"p":${payload},"extra":1}`),
    wire(`{"v":1,"v":1,"p":${payload}}`),
    wire(`{ "v":1,"p":${payload}}`),
    cursor("orphans", { i: zero, b: true }),
    cursor("orphans", { i: zero, h: hashA }),
    cursor("orphans", { i: zero, b: false }),
    cursor("orphans", { i: "00000000-0000-0000-0000-0000000000AB" }),
    cursor("wanted", { c: 1, k: "YR", h: hashA }),
    cursor("wanted", { c: 0, k: "YQ", h: hashA }),
    cursor("wanted", { c: 1, k: "YQ", h: hashA.toUpperCase() }),
    cursor("invalid_sources", { i: one }),
    cursor("undigested_sources", { i: one, t: "2026-10-30T09:02:03+08:00" }),
    cursor("undigested_sources", { i: one, t: "2026-10-30T01:02:03.000Z" }),
    cursor("orphans", { i: one }, llm, "E:llm_wiki:"),
  ];
  await Promise.all(
    invalid.map(async (token) => {
      const answer = await items(client, "not-a-rule", { cursor: token, limit: 0 }, hidden);
      expect(answer.response.status, token).toBe(400);
      expect(answer.error?.code, token).toBe("bad_request");
    })
  );
  expect(unwrap(await items(client, "orphans", { cursor: valid })).data).toEqual([orphan(one)]);
});

test("fresh responses isolate caller edits and implicit cursors survive a stored mode change", async () => {
  const { client, facts } = fixture();
  const first = unwrap(await summary(client));
  first.rules[0]?.items.splice(0);
  expect(unwrap(await items(client, "wanted")).data).toEqual([wanted(hashA), wanted(hashB)]);
  facts.mode = "standard";
  facts.computed_at = "2026-10-30T01:02:04Z";
  const next = unwrap(await items(client, "orphans", { cursor: cursor("orphans", { i: zero }) }));
  expect(next).toEqual({
    mode: "standard",
    computed_at: "2026-10-30T01:02:04Z",
    stale_pages: 7,
    data: [orphan(one)],
    next_cursor: null,
  });
  expect(unwrap(await summary(client)).rules.map((rule) => rule.rule)).toEqual([
    "wanted",
    "orphans",
    "ambiguous",
    "invalid_frontmatter",
  ]);
});
