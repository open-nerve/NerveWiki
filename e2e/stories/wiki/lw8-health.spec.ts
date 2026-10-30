/* oxlint-disable no-await-in-loop -- Each read is a separate snapshot; continuation requests consume the preceding real cursor. */
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import path from "node:path";

import type { ApiClient, Notebook, Page } from "@nervewiki/api-client";

import { uploadAsset, utf8 } from "../../fixtures/assets";
import { bearer, createToken, emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { errorOf, mcpClient, postMcp, resultOf, type McpClient } from "../../fixtures/mcp";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, listNodes, readContent, renameNode, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// LW8 is a client story: the running binary parses and saves every page below. The expected facts come from this
// small graph and its input bytes, never another lint response or a production rule/normalization helper.
const rules = [
  "wanted",
  "orphans",
  "ambiguous",
  "invalid_frontmatter",
  "missing_type",
  "missing_summary",
  "undigested_sources",
  "invalid_sources",
] as const;
type Rule = (typeof rules)[number];
type Mode = "standard" | "llm_wiki";
type Entry = "REST" | "notebook" | "account";
type Fields = Record<string, unknown> & { rule: Rule; item_key: string };
type Item = Fields & { cursor: string };
interface ExpectedItem {
  fields: Fields;
  path?: string;
  position: Record<string, unknown>;
}
interface Envelope {
  mode: Mode;
  computed_at: string;
  stale_pages: number;
}
interface Summary extends Envelope {
  rules: { rule: Rule; count: number; count_capped: boolean; items: Item[]; next_cursor: string | null }[];
}
interface ItemPage extends Envelope {
  data: Item[];
  next_cursor: string | null;
}
type Oracle = Record<Rule, ExpectedItem[]>;
interface Graph {
  notebook: Notebook;
  expected: Oracle;
  overview: Page;
  content: string;
}

const toolLists = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, "../../../api/mcp/tools.json"), "utf8")
) as Record<"notebook" | "account", { name: string }[]>;
const missingTargets = [
  "missing a",
  "missing b",
  "missing c",
  "missing d",
  ...["left", "right"].map((s) => `${"x".repeat(255)}/${s}`),
];
const valid = "---\ntype: note\nsummary: A complete note\n---\n";

function sha(value: string | Buffer): string {
  return createHash("sha256").update(value).digest("hex");
}

function emptyOracle(): Oracle {
  return {
    wanted: [],
    orphans: [],
    ambiguous: [],
    invalid_frontmatter: [],
    missing_type: [],
    missing_summary: [],
    undigested_sources: [],
    invalid_sources: [],
  };
}

function wanted(targets: string[]): ExpectedItem[] {
  // These fixture targets are already canonical ASCII, so the wire display's 256-byte cut is also a string slice.
  return targets
    .map((target) => ({
      fields: {
        rule: "wanted" as const,
        item_key: `w:${sha(target)}`,
        target: target.slice(0, 256),
        target_cut: target.length > 256,
        pages: 1,
      },
      position: { c: 1, k: Buffer.from(target.slice(0, 256)).toString("base64url"), h: sha(target) },
    }))
    .toSorted(
      (a, b) =>
        Buffer.compare(Buffer.from(String(a.fields.target)), Buffer.from(String(b.fields.target))) ||
        String(a.position.h).localeCompare(String(b.position.h))
    );
}

function pageItem(rule: Rule, page: Page, nodePath: string): ExpectedItem {
  return {
    fields: { rule, item_key: `p:${page.id}`, page_id: page.id },
    path: `${nodePath}.md`,
    position: { i: page.id },
  };
}

function sourceItem(
  node: { id: string; created_at: string },
  kind: "page" | "asset",
  bytes: number,
  nodePath: string
): ExpectedItem {
  return {
    fields: {
      rule: "undigested_sources",
      item_key: `n:${node.id}`,
      node_id: node.id,
      kind,
      bytes,
      created_at: node.created_at,
    },
    path: kind === "page" ? `${nodePath}.md` : nodePath,
    position: { i: node.id, t: node.created_at },
  };
}

function timeKey(value: unknown): string {
  const match = /^(.*T\d{2}:\d{2}:\d{2})(?:\.(\d+))?Z$/.exec(String(value));
  if (!match) throw new Error(`not a UTC creation time: ${String(value)}`);
  return `${match[1]}.${(match[2] ?? "").padEnd(9, "0")}`;
}

async function seed(api: ApiClient, credential: string, slug: string, mode: Mode): Promise<Graph> {
  const notebook = await createNotebook(api, credential, slug, `Health ${mode}`, "none", {
    mode,
    templateLocale: "en",
  });
  const make = (title: string, content = valid, parent: string | null = null) =>
    createPage(api, credential, notebook.id, title, parent, content);
  const a = await make("A");
  const b = await make("B");
  await make("Twin", valid, a.id);
  await make("Twin", valid, b.id);
  const broken = await make("Broken", "---\ntype: [\n---\nAn invalid YAML document.\n");
  const missingType = await make("MissingType", "---\nsummary: Has no type\n---\n[[Overview]]\n");
  const missingSummary = await make("MissingSummary", "---\ntype: note\n---\n[[Overview]]\n");
  const lonely = await make("Lonely");
  const expected = emptyOracle();
  let sourceLinks = "";
  if (mode === "llm_wiki") {
    const raw = notebook.source_root_id;
    if (raw === null) throw new Error("LLM creation omitted its source root");
    await make("Empty", "", raw);
    await make("Consumed", "A directly consumed source.\n", raw);
    const pendingBody = "Pending 原始证据\n[[raw/Pending/child.txt]]\n";
    const pending = await make("Pending", pendingBody, raw);
    await uploadAsset(
      api,
      credential,
      notebook.id,
      { name: "child.txt", bytes: utf8("represented by its body parent") },
      pending.id
    );
    const propertyBody = '---\n"": "[[raw/PropertyOnly/floating.txt]]"\n---\nOnly a property mentions this child.\n';
    const propertyOnly = await make("PropertyOnly", propertyBody, raw);
    const floatingBytes = utf8("not a body reference — 浮动\n");
    const floating = await uploadAsset(
      api,
      credential,
      notebook.id,
      { name: "floating.txt", bytes: floatingBytes },
      propertyOnly.id
    );
    const looseBytes = utf8("an independent source\n");
    const loose = await uploadAsset(api, credential, notebook.id, { name: "loose.txt", bytes: looseBytes }, raw);
    const renamed = await renameNode(api, credential, loose.id, "renamed.txt");
    expect(renamed.response.status, JSON.stringify(renamed.error)).toBe(200);
    expect(renamed.data?.created_at, "renaming preserves the original creation time").toBe(loose.created_at);
    expect(renamed.data?.updated_at).not.toBe(loose.created_at);
    await uploadAsset(
      api,
      credential,
      notebook.id,
      { name: "direct.txt", bytes: utf8("a directly consumed attachment") },
      raw
    );
    expected.undigested_sources = [
      sourceItem(pending, "page", utf8(pendingBody).length, "raw/Pending"),
      sourceItem(propertyOnly, "page", utf8(propertyBody).length, "raw/PropertyOnly"),
      sourceItem(floating, "asset", floatingBytes.length, "raw/PropertyOnly/floating.txt"),
      sourceItem(loose, "asset", looseBytes.length, "raw/renamed.txt"),
    ].toSorted(
      (x, y) =>
        timeKey(x.fields.created_at).localeCompare(timeKey(y.fields.created_at)) ||
        String(x.fields.node_id).localeCompare(String(y.fields.node_id))
    );
    sourceLinks = "\n[[raw/Consumed]] [[raw/direct.txt]]\n";
  }
  const content =
    "---\ntype: note\nsummary: The index\nsources: 42\n" +
    (mode === "llm_wiki" ? '"": "[[raw/PropertyOnly]]"\n' : "") +
    "---\n" +
    [...missingTargets, "Twin", "A/Twin", "B/Twin", "A", "B", "Broken", "MissingType", "MissingSummary"]
      .map((target) => `[[${target}]]`)
      .join(" ") +
    sourceLinks;
  const overview = await make("Overview", content);
  expected.wanted = wanted(missingTargets);
  expected.orphans = [pageItem("orphans", lonely, "Lonely")];
  expected.ambiguous = [
    {
      fields: {
        rule: "ambiguous",
        item_key: `a:${overview.id}:${sha("Twin")}`,
        page_id: overview.id,
        target: "Twin",
        target_cut: false,
      },
      path: "Overview.md",
      position: { i: overview.id, h: sha("Twin") },
    },
  ];
  expected.invalid_frontmatter = [pageItem("invalid_frontmatter", broken, "Broken")];
  if (mode === "llm_wiki") {
    expected.missing_type = [pageItem("missing_type", missingType, "MissingType")];
    expected.missing_summary = [pageItem("missing_summary", missingSummary, "MissingSummary")];
    // Stable JSON 42 has exactly two UTF-8 bytes. J + uint64BE(2) + 42 identifies the whole non-link slot.
    const j42 = sha(Buffer.from([0x4a, 0, 0, 0, 0, 0, 0, 0, 2, 0x34, 0x32]));
    expected.invalid_sources = [
      {
        fields: {
          rule: "invalid_sources",
          item_key: `s:${overview.id}:${j42}`,
          page_id: overview.id,
          target: "42",
          target_cut: false,
          not_link: true,
        },
        path: "Overview.md",
        position: { i: overview.id, h: j42 },
      },
    ];
  }
  return { notebook, expected, overview, content };
}

function checkEnvelope(answer: Envelope, mode: Mode, before: number, after: number): void {
  expect(answer.mode).toBe(mode);
  expect(answer.stale_pages).toBe(0);
  expect(answer.computed_at).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/);
  expect(Date.parse(answer.computed_at)).toBeGreaterThanOrEqual(before);
  expect(Date.parse(answer.computed_at)).toBeLessThanOrEqual(after);
}

function checkItems(actual: Item[], expected: ExpectedItem[], notebook: string, entry: Entry): void {
  expect(actual).toHaveLength(expected.length);
  for (const [index, want] of expected.entries()) {
    const { cursor, ...fields } = actual[index]!;
    expect(fields).toEqual({
      ...want.fields,
      ...(entry !== "REST" && want.path !== undefined ? { path: want.path } : {}),
    });
    expect(cursor).toMatch(/^[A-Za-z0-9_-]+$/);
    expect(Buffer.byteLength(cursor)).toBeLessThanOrEqual(2048);
    expect(JSON.parse(Buffer.from(cursor, "base64url").toString("utf8"))).toEqual({
      v: 1,
      p: { v: 1, r: want.fields.rule, n: notebook, s: "I", ...want.position },
    });
  }
}

function checkSummary(answer: Summary, graph: Pick<Graph, "notebook" | "expected">, entry: Entry): void {
  const active = graph.notebook.mode === "standard" ? rules.slice(0, 4) : rules;
  expect(Object.keys(answer).toSorted()).toEqual(["computed_at", "mode", "rules", "stale_pages"]);
  expect(answer.rules.map((r) => r.rule)).toEqual(active);
  for (const row of answer.rules) {
    const all = graph.expected[row.rule];
    expect(Object.keys(row).toSorted()).toEqual(["count", "count_capped", "items", "next_cursor", "rule"]);
    expect(row.count).toBe(all.length);
    expect(row.count_capped).toBe(false);
    checkItems(row.items, all.slice(0, 5), graph.notebook.id, entry);
    expect(row.next_cursor).toBe(all.length > 5 ? row.items.at(-1)!.cursor : null);
  }
}

async function restSummary(
  api: ApiClient,
  credential: string,
  graph: Pick<Graph, "notebook" | "expected">
): Promise<Summary> {
  const before = Date.now();
  const { data, error, response } = await api.GET("/api/v0/notebooks/{notebook_id}/lint", {
    params: { path: { notebook_id: graph.notebook.id } },
    headers: bearer(credential),
  });
  expect(response.status, JSON.stringify(error)).toBe(200);
  if (!data) throw new Error("lint answered without a summary");
  checkEnvelope(data, graph.notebook.mode, before, Date.now());
  checkSummary(data, graph, "REST");
  return data;
}

async function restItems(
  api: ApiClient,
  credential: string,
  notebook: Notebook,
  rule: Rule,
  limit: number,
  cursor?: string
): Promise<ItemPage> {
  const before = Date.now();
  const { data, error, response } = await api.GET("/api/v0/notebooks/{notebook_id}/lint/{rule}", {
    params: { path: { notebook_id: notebook.id, rule }, query: { limit, ...(cursor === undefined ? {} : { cursor }) } },
    headers: bearer(credential),
  });
  expect(response.status, JSON.stringify(error)).toBe(200);
  if (!data) throw new Error("lint answered without items");
  checkEnvelope(data, notebook.mode, before, Date.now());
  return data;
}

async function agentFor(
  baseURL: string,
  api: ApiClient,
  credential: string,
  entry: "notebook" | "account",
  notebook: string
): Promise<McpClient> {
  const token = await createToken(api, credential, { name: `LW8 ${entry}` });
  const agent = await mcpClient(baseURL, token.token, entry === "notebook" ? { notebook } : {});
  const actual = await agent.listTools();
  expect(actual).toHaveLength(entry === "notebook" ? 16 : 18);
  expect(byName(actual)).toEqual(byName(toolLists[entry]));
  return agent;
}

function byName(tools: { name: string }[]) {
  return Object.fromEntries(tools.map((tool) => [tool.name, tool]));
}

function notebookArg(entry: Entry, notebook: string): Record<string, string> {
  return entry === "account" ? { notebook } : {};
}

async function agentSummary(
  agent: McpClient,
  entry: Entry,
  graph: Pick<Graph, "notebook" | "expected">
): Promise<Summary> {
  const before = Date.now();
  const answer = resultOf<Summary>(
    await agent.call("lint", notebookArg(entry, graph.notebook.id)),
    `${entry} lint summary`
  );
  checkEnvelope(answer, graph.notebook.mode, before, Date.now());
  checkSummary(answer, graph, entry);
  return answer;
}

async function agentItems(
  agent: McpClient,
  entry: Entry,
  notebook: Notebook,
  rule: Rule,
  limit: number,
  cursor?: string,
  pending = false
): Promise<ItemPage> {
  const before = Date.now();
  const answer = resultOf<ItemPage>(
    await agent.call(pending ? "pending_sources" : "lint", {
      ...notebookArg(entry, notebook.id),
      ...(pending ? {} : { rule }),
      limit,
      ...(cursor === undefined ? {} : { cursor }),
    }),
    `${entry} ${rule}`
  );
  checkEnvelope(answer, notebook.mode, before, Date.now());
  return answer;
}

function checkPage(answer: ItemPage, expected: ExpectedItem[], notebook: string, entry: Entry, more: boolean): void {
  expect(Object.keys(answer).toSorted()).toEqual(["computed_at", "data", "mode", "next_cursor", "stale_pages"]);
  checkItems(answer.data, expected, notebook, entry);
  expect(answer.next_cursor).toBe(more ? answer.data.at(-1)!.cursor : null);
}

for (const locale of ["zh-CN", "en"] as const) {
  test(`LW8: a new ${locale} LLM notebook has eight clean rules`, async ({ api }, testInfo) => {
    const team = await newTeam(api, testInfo);
    const notebook = await createNotebook(api, team.pat, team.workspace.slug, `Clean ${locale}`, "none", {
      mode: "llm_wiki",
      templateLocale: locale,
    });
    const nodes = await listNodes(api, team.pat, notebook.id);
    expect(nodes.map((n) => n.name).toSorted()).toEqual(["AGENTS", "raw"]);
    expect(nodes.find((n) => n.name === "raw")?.id).toBe(notebook.source_root_id);
    const agents = nodes.find((n) => n.name === "AGENTS")!;
    const content = await readContent(api, team.pat, agents.id);
    expect(content.content).toContain(locale === "zh-CN" ? "# 这本 LLM Wiki 的约定" : "# How this LLM Wiki is kept");
    await restSummary(api, team.pat, { notebook, expected: emptyOracle() });
  });
}

for (const entry of ["REST", "notebook", "account"] as const) {
  test(`LW8: ${entry} reads four standard and eight LLM rules from saved facts`, async ({
    api,
    nervewiki,
  }, testInfo) => {
    const team = await newTeam(api, testInfo);
    for (const mode of ["standard", "llm_wiki"] as const) {
      const graph = await seed(api, team.pat, team.workspace.slug, mode);
      const agent =
        entry === "REST" ? undefined : await agentFor(nervewiki.baseURL, api, team.pat, entry, graph.notebook.id);
      if (agent) await agentSummary(agent, entry, graph);
      else await restSummary(api, team.pat, graph);
      for (const rule of mode === "standard" ? rules.slice(0, 4) : rules) {
        const answer = agent
          ? await agentItems(agent, entry, graph.notebook, rule, 100)
          : await restItems(api, team.pat, graph.notebook, rule, 100);
        checkPage(answer, graph.expected[rule], graph.notebook.id, entry, false);
      }
      if (agent) {
        const unknown = errorOf(
          await agent.call("lint", { ...notebookArg(entry, graph.notebook.id), rule: "unknown" }),
          "unknown rule"
        );
        expect(unknown.code).toBe("wiki.lint_rule_not_found");
        if (mode === "standard") {
          expect(
            errorOf(await agent.call("pending_sources", notebookArg(entry, graph.notebook.id)), "standard pending").code
          ).toBe("notebook.not_llm_wiki");
        } else {
          const pending = await agentItems(agent, entry, graph.notebook, "undigested_sources", 100, undefined, true);
          checkPage(pending, graph.expected.undigested_sources, graph.notebook.id, entry, false);
        }
      }
    }
  });
}

test("LW8: a genuine middle page crosses REST, notebook MCP, account MCP, and REST without losing identity", async ({
  api,
  nervewiki,
}, testInfo) => {
  const team = await newTeam(api, testInfo);
  const graph = await seed(api, team.pat, team.workspace.slug, "llm_wiki");
  // Each MCP entry receives a separately issued PAT; neither shares a client session with the other entry.
  const book = await agentFor(nervewiki.baseURL, api, team.pat, "notebook", graph.notebook.id);
  const account = await agentFor(nervewiki.baseURL, api, team.pat, "account", graph.notebook.id);
  const summary = await restSummary(api, team.pat, graph);
  const previewNext = summary.rules[0]!.next_cursor;
  expect(previewNext).not.toBeNull();
  const tail = await restItems(api, team.pat, graph.notebook, "wanted", 1, previewNext!);
  checkPage(tail, graph.expected.wanted.slice(5), graph.notebook.id, "REST", false);
  let cursor: string | undefined;
  const keys: string[] = [];
  const entries = ["REST", "notebook", "account", "REST", "notebook", "account"] as const;
  for (const [index, entry] of entries.entries()) {
    const answer =
      entry === "REST"
        ? await restItems(api, team.pat, graph.notebook, "wanted", 1, cursor)
        : await agentItems(entry === "notebook" ? book : account, entry, graph.notebook, "wanted", 1, cursor);
    checkPage(answer, graph.expected.wanted.slice(index, index + 1), graph.notebook.id, entry, index < 5);
    keys.push(answer.data[0]!.item_key);
    cursor = answer.next_cursor ?? undefined;
  }
  expect(new Set(keys).size).toBe(6);
  const equalDisplay = graph.expected.wanted.filter((item) => item.fields.target_cut);
  expect(equalDisplay).toHaveLength(2);
  expect(equalDisplay[0]!.fields.target).toBe(equalDisplay[1]!.fields.target);
  expect(equalDisplay[0]!.fields.item_key).not.toBe(equalDisplay[1]!.fields.item_key);
  await writeContent(api, team.pat, graph.overview.id, {
    base_revision: graph.overview.revision,
    content: `${graph.content}\n[[after read]]\n`,
  });
  const changed = { ...graph, expected: { ...graph.expected, wanted: wanted([...missingTargets, "after read"]) } };
  await restSummary(api, team.pat, changed);
  await agentSummary(book, "notebook", changed);
  await agentSummary(account, "account", changed);
});

test("LW8: readers can read health while a private notebook is hidden before the notebook MCP Gate", async ({
  api,
  nervewiki,
}, testInfo) => {
  const team = await newTeam(api, testInfo);
  const graph = await seed(api, team.pat, team.workspace.slug, "llm_wiki");
  const hidden = await createNotebook(api, team.pat, team.workspace.slug, "Private", "none", { mode: "llm_wiki" });
  const reader = await joinAs(api, team.pat, team.workspace.slug, emailFor(testInfo, "reader"), "guest");
  const me = await api.GET("/api/v0/me", { headers: bearer(reader) });
  if (!me.data) throw new Error("reader has no account metadata");
  await addedNotebookMember(api, team.pat, graph.notebook.id, me.data.id, "reader");
  await restSummary(api, reader, graph);
  const notebookPAT = await createToken(api, reader, { name: "LW8 reader notebook" });
  const accountPAT = await createToken(api, reader, { name: "LW8 reader account" });
  expect(notebookPAT.id).not.toBe(accountPAT.id);
  const book = await mcpClient(nervewiki.baseURL, notebookPAT.token, { notebook: graph.notebook.id });
  const account = await mcpClient(nervewiki.baseURL, accountPAT.token);
  await agentSummary(book, "notebook", graph);
  await agentSummary(account, "account", graph);
  const denied = await api.GET("/api/v0/notebooks/{notebook_id}/lint", {
    params: { path: { notebook_id: hidden.id } },
    headers: bearer(reader),
  });
  expect(denied.response.status).toBe(404);
  expect(denied.error?.code).toBe("notebook.not_found");
  const gate = await postMcp(
    nervewiki.baseURL,
    notebookPAT.token,
    {
      jsonrpc: "2.0",
      id: 1,
      method: "tools/call",
      params: { name: "lint", arguments: { rule: "orphans", cursor: "!" } },
    },
    hidden.id
  );
  expect(gate.status).toBe(404);
  expect((await gate.json()).code).toBe("notebook.not_found");
  // Account entry has no notebook Gate: the ordinary tool error is the cursor's 400 bad_request, before visibility.
  const bad = errorOf(
    await account.call("lint", { notebook: hidden.id, rule: "orphans", cursor: "!" }),
    "account bad cursor before hidden notebook"
  );
  expect(bad.code).toBe("bad_request");
  expect(bad.fields).toEqual([
    { field: "cursor", code: "invalid_format", message: "is not a cursor this list can read" },
  ]);
  const hiddenTool = errorOf(
    await account.call("lint", { notebook: hidden.id, rule: "orphans" }),
    "account hidden notebook after fixing cursor"
  );
  expect(hiddenTool.code).toBe("notebook.not_found");
});
