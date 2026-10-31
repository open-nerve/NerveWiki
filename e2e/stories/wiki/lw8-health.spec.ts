/* oxlint-disable no-await-in-loop -- Each read is a separate snapshot; continuation requests consume the preceding real cursor. */
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import path from "node:path";

import type { ApiClient, Notebook, Page } from "@nervewiki/api-client";
import type { Page as BrowserPage, TestInfo } from "@playwright/test";

import { uploadAsset, utf8 } from "../../fixtures/assets";
import { bearer, createToken, emailFor } from "../../fixtures/auth";
import { answerTo, eventStreamPath } from "../../fixtures/browser";
import { settleEvents } from "../../fixtures/events";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { errorOf, mcpClient, postMcp, resultOf, type McpClient } from "../../fixtures/mcp";
import { addedNotebookMember, removeNotebookMember } from "../../fixtures/notebook-members";
import { downgradeWith, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook, downgradeNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  deleteNode,
  listNodes,
  moveNode,
  readContent,
  renameNode,
  writeContent,
} from "../../fixtures/pages";
import { expect, test, watchOf } from "../../fixtures/test";
import {
  connectedHealth,
  healthCard,
  healthDocumentStayed,
  healthLink,
  healthPagination,
  healthPath,
  healthReads,
  healthShown,
  holdHealthGet,
  streamHellos,
} from "../../fixtures/wiki-health";
import { wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

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

async function healthReader(api: ApiClient, credential: string, slug: string, notebook: string, info: TestInfo) {
  const tokens = await joinOnboarded(api, credential, slug, emailFor(info, "health-reader"), "guest");
  const me = await api.GET("/api/v0/me", { headers: bearer(tokens.access_token) });
  expect(me.response.status).toBe(200);
  if (!me.data) throw new Error("the health reader has no account");
  await addedNotebookMember(api, credential, notebook, me.data.id, "reader");
  return { tokens, id: me.data.id };
}

async function healthAnswer(page: BrowserPage, notebook: Notebook, action: () => Promise<unknown>, checks = 8) {
  const answer = answerTo(page, "GET", `/api/v0/notebooks/${notebook.id}/lint`);
  await action();
  const response = await answer;
  expect(response.status()).toBe(200);
  await healthShown(page, checks);
  return (await response.json()) as Summary;
}

const modeNotice = "The notebook mode has changed. Recheck its health.";

test.describe("LW8 (page)", () => {
  test.beforeEach(async ({ browser }, info) => {
    await info.attach("browser-runtime", {
      body: JSON.stringify({
        engine: browser.browserType().name(),
        version: browser.version(),
        project: info.project.name,
      }),
      contentType: "application/json",
    });
  });

  for (const mode of ["standard", "llm_wiki"] as const) {
    test(`a reader opens ${mode} from the sidebar: every applicable rule, full paths and literal values fit 375px`, async ({
      api,
      signedInPage,
    }, info) => {
      const team = await newOnboardedTeam(api, info);
      const graph = await seed(api, team.pat, team.workspace.slug, mode);
      const reader = await healthReader(api, team.pat, team.workspace.slug, graph.notebook.id, info);
      const page = await signedInPage(reader.tokens);
      const reads = healthReads(page, graph.notebook.id);
      await page.goto(wikiPagePath(team.workspace.slug, graph.notebook.id, graph.overview.id));
      await page.evaluate(() => Object.assign(window, { lw8Document: true }));
      const active = mode === "standard" ? rules.slice(0, 4) : rules;
      checkSummary(
        await healthAnswer(page, graph.notebook, () => healthLink(page, graph.notebook).click(), active.length),
        graph,
        "REST"
      );
      const main = page.getByRole("main");
      expect(await main.locator("section[id]").evaluateAll((cards) => cards.map((card) => card.id))).toEqual(active);
      for (const rule of active) {
        const card = healthCard(page, rule);
        await expect(card).toBeVisible();
        await expect(card.getByRole("listitem")).toHaveCount(Math.min(5, graph.expected[rule].length));
      }
      await expect(healthCard(page, "orphans").getByRole("link", { name: "Lonely.md", exact: true })).toBeVisible();
      const wantedCard = healthCard(page, "wanted");
      await wantedCard.getByRole("button", { name: "More", exact: true }).click();
      await expect(wantedCard.getByRole("listitem")).toHaveCount(6);
      await expect(wantedCard.getByText("Target shortened.", { exact: true })).toHaveCount(2);
      await expect(wantedCard.getByText(`${"x".repeat(255)}/`, { exact: true })).toHaveCount(2);
      if (mode === "llm_wiki") {
        await expect(
          healthCard(page, "invalid_sources").getByText("This value is not a single link.", { exact: true })
        ).toBeVisible();
        await expect(
          healthCard(page, "undigested_sources").getByRole("link", {
            name: "raw/PropertyOnly/floating.txt",
            exact: true,
          })
        ).toBeVisible();
      }
      await page.setViewportSize({ width: 375, height: 800 });
      const width = await page.evaluate(() => ({
        viewport: document.documentElement.clientWidth,
        content: document.documentElement.scrollWidth,
      }));
      expect(width).toEqual({ viewport: 375, content: 375 });
      const box = await wantedCard.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.x + box!.width).toBeLessThanOrEqual(375);
      expect([reads.summary(), reads.rule("wanted")]).toEqual([1, 1]);
      await healthDocumentStayed(page);
    });
  }

  test("a clean notebook keeps all zero checks collapsed, and the undigested anchor opens and focuses its zero check", async ({
    api,
    signedInPage,
  }, info) => {
    const team = await newOnboardedTeam(api, info);
    const notebook = await createNotebook(api, team.pat, team.workspace.slug, "Clean health", "none", {
      mode: "llm_wiki",
    });
    const page = await signedInPage(team.tokens);
    await page.goto(notebookPath(team.workspace.slug, notebook.id));
    await healthAnswer(page, notebook, () => healthLink(page, notebook).click());
    const details = page.getByRole("main").locator("details");
    await expect(details.locator("summary")).toHaveText("No issues (8 checks)");
    await expect(details).not.toHaveAttribute("open");
    await expect(healthCard(page, "undigested_sources")).toBeHidden();
    await details.locator("summary").click();
    expect(await details.locator("section[id]").evaluateAll((cards) => cards.map((card) => card.id))).toEqual(rules);
    await expect(details.getByText("0 issues", { exact: true })).toHaveCount(8);
    await expect(page.getByRole("button", { name: "More", exact: true })).toHaveCount(0);
    await page.goto(`${healthPath(team.workspace.slug, notebook.id)}#undigested_sources`);
    await healthShown(page);
    await expect(page.locator("#undigested_sources-heading")).toBeFocused();
    await expect(details).toHaveAttribute("open", "");
  });

  test("focus: 61 wanted groups give five previews, fifty non-final additions, six final additions, and a fresh preview after recheck", async ({
    api,
    signedInPage,
  }, info) => {
    const team = await newOnboardedTeam(api, info);
    const { notebook, overview, targets } = await healthPagination(api, team.pat, team.workspace.slug);
    const page = await signedInPage(team.tokens);
    const reads = healthReads(page, notebook.id);
    await page.goto(wikiPagePath(team.workspace.slug, notebook.id, overview.id));
    const summary = await healthAnswer(page, notebook, () => healthLink(page, notebook).click());
    const wantedSummary = summary.rules.find((rule) => rule.rule === "wanted")!;
    expect(wantedSummary.count).toBe(61);
    expect(wantedSummary.items.map((item) => item.target)).toEqual(targets.slice(0, 5));
    const card = healthCard(page, "wanted");
    const items = card.getByRole("listitem");
    await expect(items).toHaveCount(5);
    const more = card.getByRole("button", { name: "More", exact: true });
    const first = answerTo(page, "GET", `/api/v0/notebooks/${notebook.id}/lint/wanted`);
    await more.click();
    const response = await first;
    expect(response.status()).toBe(200);
    const query = new URL(response.url()).searchParams;
    expect(query.get("limit")).toBeNull();
    expect(query.get("cursor")).toBe(wantedSummary.next_cursor);
    const next = (await response.json()) as ItemPage;
    expect(next.data.map((item) => item.target)).toEqual(targets.slice(5, 55));
    expect(new Set(next.data.map((item) => item.item_key)).size).toBe(50);
    expect(next.next_cursor).toBe(next.data.at(-1)!.cursor);
    await expect(items).toHaveCount(55);
    await expect(more).toBeVisible();
    await expect(items.nth(5)).toBeFocused();
    await expect(items.nth(5)).toContainText("m005");
    const last = answerTo(page, "GET", `/api/v0/notebooks/${notebook.id}/lint/wanted`);
    await more.click();
    const final = (await (await last).json()) as ItemPage;
    expect(final.data.map((item) => item.target)).toEqual(targets.slice(55));
    expect(final.next_cursor).toBeNull();
    await expect(items).toHaveCount(61);
    await expect(more).toHaveCount(0);
    await expect(items.nth(55)).toBeFocused();
    await createPage(api, team.pat, notebook.id, "m000", null, valid);
    expect([reads.summary(), reads.rule("wanted")]).toEqual([1, 2]);
    const changed = await healthAnswer(page, notebook, () =>
      page.getByRole("button", { name: "Recheck", exact: true }).click()
    );
    expect(changed.rules.find((rule) => rule.rule === "wanted")?.count).toBe(60);
    await expect(items).toHaveCount(5);
    await expect(items.first()).toContainText("m001");
    expect([reads.summary(), reads.rule("wanted")]).toEqual([2, 2]);
  });

  test("focus: non-final More leaves the reader's moved focus alone", async ({ api, signedInPage }, info) => {
    const team = await newOnboardedTeam(api, info);
    const { notebook, overview } = await healthPagination(api, team.pat, team.workspace.slug);
    const page = await signedInPage(team.tokens);
    const reads = healthReads(page, notebook.id);
    await page.goto(wikiPagePath(team.workspace.slug, notebook.id, overview.id));
    await healthAnswer(page, notebook, () => healthLink(page, notebook).click());
    const held = await holdHealthGet(page, `/api/v0/notebooks/${notebook.id}/lint/wanted`);
    const card = healthCard(page, "wanted");
    try {
      await card.getByRole("button", { name: "More", exact: true }).press("Enter");
      await held.entered;
      await page.keyboard.press("Tab");
      const movedTo = healthCard(page, "orphans").getByRole("link", { name: "Overview.md", exact: true });
      await expect(movedTo).toBeFocused();
      held.release();
      await expect(card.getByRole("listitem")).toHaveCount(55);
      await expect(card.getByRole("button", { name: "More", exact: true })).toBeVisible();
      await expect(movedTo).toBeFocused();
      expect([reads.summary(), reads.rule("wanted")]).toEqual([1, 1]);
    } finally {
      await held.finish();
    }
  });

  test("real page clicks, Back, Forward and same-address sidebar reentry each recalculate health without reloading", async ({
    api,
    signedInPage,
  }, info) => {
    const team = await newOnboardedTeam(api, info);
    const graph = await seed(api, team.pat, team.workspace.slug, "llm_wiki");
    const page = await signedInPage(team.tokens);
    const reads = healthReads(page, graph.notebook.id);
    const initial = wikiPagePath(team.workspace.slug, graph.notebook.id, graph.overview.id);
    await page.goto(initial);
    await page.evaluate(() => Object.assign(window, { lw8Document: true }));
    await healthAnswer(page, graph.notebook, () => healthLink(page, graph.notebook).click());
    await healthCard(page, "orphans").getByRole("link", { name: "Lonely.md", exact: true }).click();
    await expect(page.getByRole("heading", { level: 1, name: "Lonely", exact: true })).toBeVisible();
    expect(reads.summary()).toBe(1);
    await createPage(api, team.pat, graph.notebook.id, "missing a", null, valid);
    const back = await healthAnswer(page, graph.notebook, () => page.goBack());
    expect(back.rules.find((rule) => rule.rule === "wanted")?.count).toBe(5);
    expect(reads.summary()).toBe(2);
    await page.goForward();
    await expect(page.getByRole("heading", { level: 1, name: "Lonely", exact: true })).toBeVisible();
    expect(reads.summary()).toBe(2);
    await healthAnswer(page, graph.notebook, () => page.goBack());
    expect(reads.summary()).toBe(3);
    await page.goBack();
    await expect(page).toHaveURL(initial);
    expect(reads.summary()).toBe(3);
    await healthAnswer(page, graph.notebook, () => page.goForward());
    expect(reads.summary()).toBe(4);
    await healthAnswer(page, graph.notebook, () => healthLink(page, graph.notebook).click());
    expect(reads.summary()).toBe(5);
    await healthDocumentStayed(page);
  });

  test("leaving an in-flight summary and returning starts a fresh request and does not leave Loading cached", async ({
    api,
    signedInPage,
  }, info) => {
    const team = await newOnboardedTeam(api, info);
    const { notebook, overview } = await healthPagination(api, team.pat, team.workspace.slug);
    const page = await signedInPage(team.tokens);
    const reads = healthReads(page, notebook.id);
    await page.goto(wikiPagePath(team.workspace.slug, notebook.id, overview.id));
    await page.evaluate(() => Object.assign(window, { lw8Document: true }));
    const held = await holdHealthGet(page, `/api/v0/notebooks/${notebook.id}/lint`);
    try {
      await healthLink(page, notebook).click();
      const abandoned = await held.entered;
      await page.goBack();
      await expect(page.getByRole("heading", { level: 1, name: "Overview", exact: true })).toBeVisible();
      await healthAnswer(page, notebook, () => page.goForward());
      expect(reads.summary()).toBe(2);
      await expect(healthCard(page, "wanted").getByRole("listitem")).toHaveCount(5);
      await expect.poll(() => abandoned.failure()).not.toBeNull();
      await healthDocumentStayed(page);
    } finally {
      await held.finish();
    }
  });

  test("focus: a snapshot attachment follows its current parent and opens the real attachment row", async ({
    api,
    signedInPage,
  }, info) => {
    const team = await newOnboardedTeam(api, info);
    const { notebook, overview } = await healthPagination(api, team.pat, team.workspace.slug);
    const old = await createPage(api, team.pat, notebook.id, "Old", notebook.source_root_id);
    const next = await createPage(api, team.pat, notebook.id, "NewParent", notebook.source_root_id);
    const asset = await uploadAsset(
      api,
      team.pat,
      notebook.id,
      { name: "evidence.txt", bytes: utf8("Move this real source.\n") },
      old.id
    );
    const page = await signedInPage(team.tokens);
    const reads = healthReads(page, notebook.id);
    await connectedHealth(page, team.workspace.slug, notebook, overview.id);
    const card = healthCard(page, "undigested_sources");
    await expect(card.getByRole("link", { name: "raw/Old/evidence.txt", exact: true })).toBeVisible();
    const computed = await page.getByRole("main").locator("time").first().getAttribute("datetime");
    await moveNode(api, team.pat, asset.id, { parent_id: next.id });
    const target = card.getByRole("link", { name: "raw/NewParent/evidence.txt", exact: true });
    await expect(target).toBeVisible();
    await expect(card.getByText("1 issue", { exact: true })).toBeVisible();
    expect(await page.getByRole("main").locator("time").first().getAttribute("datetime")).toBe(computed);
    expect(reads.total()).toBe(1);
    await target.click();
    await expect(page).toHaveURL(wikiPagePath(team.workspace.slug, notebook.id, next.id));
    await expect(page.getByRole("heading", { level: 1, name: "NewParent", exact: true })).toBeVisible();
    await expect(
      page.getByRole("region", { name: "Attachments", exact: true }).locator(`[data-attachment="${asset.id}"]`)
    ).toBeFocused();
    await healthDocumentStayed(page);
  });

  test("a disappeared snapshot node keeps its issue and count after the current tree has been read", async ({
    api,
    signedInPage,
  }, info) => {
    const team = await newOnboardedTeam(api, info);
    const { notebook, overview } = await healthPagination(api, team.pat, team.workspace.slug);
    const asset = await uploadAsset(
      api,
      team.pat,
      notebook.id,
      { name: "gone.txt", bytes: utf8("This source will disappear.\n") },
      notebook.source_root_id!
    );
    const page = await signedInPage(team.tokens);
    const reads = healthReads(page, notebook.id);
    const connection = await connectedHealth(page, team.workspace.slug, notebook, overview.id);
    const card = healthCard(page, "undigested_sources");
    await expect(card.getByRole("link", { name: "raw/gone.txt", exact: true })).toBeVisible();
    const before = connection.nodes();
    expect((await deleteNode(api, team.pat, asset.id)).response.status).toBe(204);
    await expect(card.getByText("This node is no longer available.", { exact: true })).toBeVisible();
    await expect.poll(connection.nodes).toBeGreaterThanOrEqual(before + 2);
    await expect(card.getByRole("listitem")).toHaveCount(1);
    await expect(card.getByText("1 issue", { exact: true })).toBeVisible();
    expect(reads.total()).toBe(1);
    await healthDocumentStayed(page);
  });

  test("mode arrives on the original SSE connection from another context and only manual Recheck recalculates", async ({
    api,
    nervewiki,
    signedInPage,
    anotherPage,
  }, info) => {
    const team = await newOnboardedTeam(api, info);
    const { notebook, overview } = await healthPagination(api, team.pat, team.workspace.slug);
    const reader = await healthReader(api, team.pat, team.workspace.slug, notebook.id, info);
    const marker = await createPage(api, team.pat, notebook.id, "Event marker", null, valid);
    await settleEvents(api, nervewiki.baseURL, team.pat, marker.id);
    const owner = await signedInPage(team.tokens);
    const page = await anotherPage(reader.tokens);
    expect(owner.context()).not.toBe(page.context());
    await owner.goto(notebookPath(team.workspace.slug, notebook.id, "general"));
    const reads = healthReads(page, notebook.id);
    const connection = await connectedHealth(page, team.workspace.slug, notebook, overview.id);
    const before = { notebooks: connection.notebooks(), opened: connection.streams.opened() };
    const answer = answerTo(owner, "POST", `/api/v0/notebooks/${notebook.id}/downgrade`);
    await downgradeWith(owner);
    expect((await answer).status()).toBe(200);
    await expect(page.getByText(modeNotice, { exact: true })).toBeVisible();
    await expect(healthCard(page, "wanted").getByRole("button", { name: "More", exact: true })).toBeDisabled();
    expect(connection.notebooks()).toBeGreaterThan(before.notebooks);
    expect(connection.streams.opened()).toBe(before.opened);
    expect(connection.ended.has(connection.stream)).toBe(false);
    expect((await streamHellos(page)).length).toBe(1);
    expect(reads.total()).toBe(1);
    const changed = await healthAnswer(
      page,
      notebook,
      () => page.getByRole("button", { name: "Recheck", exact: true }).click(),
      4
    );
    expect(changed.rules.map((rule) => rule.rule)).toEqual(rules.slice(0, 4));
    await expect(page.getByText(modeNotice, { exact: true })).toHaveCount(0);
    expect(reads.total()).toBe(2);
    await healthDocumentStayed(page);
    expect(watchOf(page).eventStreamErrors).toEqual([]);
  });

  test("mode lost after a real SSE reset is learned on reconnect hello through notebooks without rerunning lint", async ({
    api,
    nervewiki,
    openEvents,
    signedInPage,
  }, info) => {
    const team = await newOnboardedTeam(api, info);
    const { notebook, overview } = await healthPagination(api, team.pat, team.workspace.slug);
    const reader = await healthReader(api, team.pat, team.workspace.slug, notebook.id, info);
    const scratch = await createNotebook(api, team.pat, team.workspace.slug, "Reset witness", "none");
    const membership = await addedNotebookMember(api, team.pat, scratch.id, reader.id, "reader");
    const marker = await createPage(api, team.pat, notebook.id, "Event marker", null, valid);
    await settleEvents(api, nervewiki.baseURL, team.pat, marker.id);
    const page = await signedInPage(reader.tokens);
    const reads = healthReads(page, notebook.id);
    const connection = await connectedHealth(page, team.workspace.slug, notebook, overview.id);
    const before = { notebooks: connection.notebooks(), opened: connection.streams.opened() };
    const gate = await holdHealthGet(page, eventStreamPath);
    try {
      const reset = await openEvents(reader.tokens.access_token);
      expect((await removeNotebookMember(api, team.pat, membership.id)).response.status).toBe(204);
      await reset.expectReset("access");
      await expect.poll(() => connection.ended.has(connection.stream)).toBe(true);
      const reconnect = await gate.entered;
      expect(reconnect).not.toBe(connection.stream);
      expect(connection.streams.opened()).toBe(before.opened + 1);
      expect((await streamHellos(page)).length).toBe(1);
      const mode = await openEvents(team.pat);
      expect((await downgradeNotebook(api, team.pat, notebook.id)).response.status).toBe(200);
      expect(await mode.expectNext("mode")).toEqual({ workspace_id: team.workspace.id, notebook_id: notebook.id });
      expect(connection.notebooks()).toBe(before.notebooks);
      expect(reads.total()).toBe(1);
      await expect(page.getByText(modeNotice, { exact: true })).toHaveCount(0);
      const connected = answerTo(page, "GET", eventStreamPath);
      const notebooks = answerTo(page, "GET", `/api/v0/workspaces/${team.workspace.slug}/notebooks`);
      gate.release();
      expect((await connected).status()).toBe(200);
      await expect.poll(async () => (await streamHellos(page)).length).toBe(2);
      const listed = await notebooks;
      expect(listed.status()).toBe(200);
      expect((await listed.json()).data.find((book: Notebook) => book.id === notebook.id).mode).toBe("standard");
      await expect(page.getByText(modeNotice, { exact: true })).toBeVisible();
      await expect(healthCard(page, "wanted").getByRole("button", { name: "More", exact: true })).toBeDisabled();
      expect(reads.total()).toBe(1);
      await healthAnswer(page, notebook, () => page.getByRole("button", { name: "Recheck", exact: true }).click(), 4);
      expect(reads.total()).toBe(2);
      await healthDocumentStayed(page);
      expect(watchOf(page).eventStreamErrors).toEqual([]);
    } finally {
      await gate.finish();
    }
  });

  test("a real summary transport failure is shown and one manual Try again gets the real answer", async ({
    api,
    signedInPage,
  }, info) => {
    const team = await newOnboardedTeam(api, info);
    const { notebook, overview } = await healthPagination(api, team.pat, team.workspace.slug);
    const page = await signedInPage(team.tokens);
    const reads = healthReads(page, notebook.id);
    await page.goto(wikiPagePath(team.workspace.slug, notebook.id, overview.id));
    let failedAt = 0;
    await page.route(
      (url) => url.pathname === `/api/v0/notebooks/${notebook.id}/lint`,
      async (route) => {
        failedAt = Date.now();
        await route.abort("failed");
      },
      { times: 1 }
    );
    watchOf(page).expectConsole({ errors: ["Failed to load resource: net::ERR_FAILED"] });
    await healthLink(page, notebook).click();
    const retry = page.getByRole("main").getByRole("button", { name: "Try again", exact: true });
    await expect(retry).toBeVisible();
    await expect(page.getByRole("main").getByRole("alert")).toBeVisible();
    expect(reads.summary()).toBe(1);
    expect(reads.answers).toEqual([]);
    expect(watchOf(page).apiFailures.filter((failure) => failure.endsWith(`/notebooks/${notebook.id}/lint`))).toEqual([
      `net::ERR_FAILED GET /api/v0/notebooks/${notebook.id}/lint`,
    ]);
    const clickedAt = Date.now();
    expect(clickedAt - failedAt, "manual retry precedes the normal first 5s automatic retry").toBeLessThan(5_000);
    const summary = await healthAnswer(page, notebook, () => retry.click());
    expect(summary.rules.find((rule) => rule.rule === "wanted")?.count).toBe(61);
    expect(reads.summary()).toBe(2);
    expect(reads.answers).toEqual([{ path: `/api/v0/notebooks/${notebook.id}/lint`, status: 200 }]);
    await expect(retry).toHaveCount(0);
    await expect(page.getByRole("main").getByRole("alert")).toHaveCount(0);
  });
});
