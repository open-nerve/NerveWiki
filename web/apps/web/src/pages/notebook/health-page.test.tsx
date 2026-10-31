import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { json, notebookJSON, type Answer } from "../../test/fakes";
import { lintRoutes, type LintFacts } from "../../test/lint-server";
import { assetNode, guide, install, linux, notes, pageNode, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

const home = `/lab/notebooks/${notebookJSON.id}`;
const healthPath = `${home}/health`;
const summaryRoute = "GET /api/v0/notebooks/*/lint";
const nodesRoute = `GET /api/v0/notebooks/${notebookJSON.id}/nodes`;
const raw = pageNode(51, "raw");
const folder = pageNode(52, "Folder", raw);
const source = pageNode(53, "Source", folder);
const asset = assetNode(54, "source.pdf", folder);
const nodes = [guide, install, linux, notes, raw, folder, source, asset];
const computed = "2026-10-31T09:10:00Z";
const created = "2026-10-30T08:01:00Z";
const hashA = "a".repeat(64);
const hashB = "b".repeat(64);
const target = "界".repeat(85) + "x";

// Literal protocol fixtures, independent of the fake's decoder and the UI's projections.
function wire(text: string): string {
  return btoa(text).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

function cursor(ruleName: keyof LintFacts["rules"], position: Record<string, unknown>): string {
  return wire(JSON.stringify({ v: 1, p: { v: 1, r: ruleName, n: notebookJSON.id, s: "I", ...position } }));
}

function facts(mode: LintFacts["mode"] = "llm_wiki"): LintFacts {
  return {
    mode,
    computed_at: computed,
    stale_pages: 7,
    rules: {
      wanted: [hashA, hashB].map((hash, index) => ({
        rule: "wanted",
        item_key: `w:${hash}`,
        cursor: cursor("wanted", {
          c: 3 - index,
          k: wire(String.fromCharCode(...new TextEncoder().encode(target))),
          h: hash,
        }),
        target,
        target_cut: true,
        pages: 3 - index,
      })),
      orphans: [
        { rule: "orphans", item_key: `p:${linux.id}`, cursor: cursor("orphans", { i: linux.id }), page_id: linux.id },
      ],
      ambiguous: [
        {
          rule: "ambiguous",
          item_key: `a:${linux.id}:${hashA}`,
          cursor: cursor("ambiguous", { h: hashA, i: linux.id }),
          page_id: linux.id,
          target: "Shared",
          target_cut: false,
        },
      ],
      invalid_frontmatter: [
        {
          rule: "invalid_frontmatter",
          item_key: `p:${notes.id}`,
          cursor: cursor("invalid_frontmatter", { i: notes.id }),
          page_id: notes.id,
        },
      ],
      missing_type: [
        {
          rule: "missing_type",
          item_key: `p:${linux.id}`,
          cursor: cursor("missing_type", { i: linux.id }),
          page_id: linux.id,
        },
      ],
      missing_summary: [
        {
          rule: "missing_summary",
          item_key: `p:${notes.id}`,
          cursor: cursor("missing_summary", { i: notes.id }),
          page_id: notes.id,
        },
      ],
      undigested_sources: [
        {
          rule: "undigested_sources",
          item_key: `n:${source.id}`,
          cursor: cursor("undigested_sources", { i: source.id, t: created }),
          node_id: source.id,
          kind: "page",
          bytes: 17,
          created_at: created,
        },
        {
          rule: "undigested_sources",
          item_key: `n:${asset.id}`,
          cursor: cursor("undigested_sources", { i: asset.id, t: created }),
          node_id: asset.id,
          kind: "asset",
          bytes: 0,
          created_at: created,
        },
      ],
      invalid_sources: [
        {
          rule: "invalid_sources",
          item_key: `s:${linux.id}:${hashA}`,
          cursor: cursor("invalid_sources", { h: hashA, i: linux.id }),
          page_id: linux.id,
          target: "Broken",
          target_cut: false,
          not_link: false,
        },
        {
          rule: "invalid_sources",
          item_key: `s:${linux.id}:${hashB}`,
          cursor: cursor("invalid_sources", { h: hashB, i: linux.id }),
          page_id: linux.id,
          target: "Broken",
          target_cut: false,
          not_link: true,
        },
      ],
    },
  };
}

function healthServer(mode: LintFacts["mode"] = "llm_wiki", answers: Record<string, Answer> = {}) {
  const value = facts(mode);
  const notebooks = new Map([[notebookJSON.id, value]]);
  const lint = lintRoutes(notebooks);
  const answer = lint[summaryRoute];
  if (answer === undefined) throw new Error("the lint fake must register its summary");
  const requests: Request[] = [];
  const state = { down: false };
  const server = pageServer({
    role: "reader",
    mode,
    sourceRootId: mode === "llm_wiki" ? raw.id : null,
    nodes,
    answers: {
      ...lint,
      [summaryRoute]: (request) => {
        requests.push(request);
        if (state.down) throw new TypeError("offline");
        return answer(request);
      },
      ...answers,
    },
  });
  return { server, value, notebooks, requests, state };
}

function health() {
  const section = screen.getByRole("heading", { level: 1, name: "Health" }).closest("section");
  if (section === null) throw new Error("Health needs its own section");
  return within(section);
}

function rule(name: string) {
  const section = health().getByRole("heading", { level: 2, name }).closest("section");
  if (section === null) throw new Error("a health rule needs its own section");
  return within(section);
}

test("a failed health summary explains the failure; Try again reads its nonempty results through the signed-in client", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  fixture.state.down = true;
  renderApp(healthPath, fixture.server.app);

  expect(await screen.findByRole("alert")).toHaveProperty(
    "textContent",
    "Cannot reach the server. Check the connection and try again."
  );
  expect(fixture.requests).toHaveLength(1);
  fixture.state.down = false;
  await user.click(health().getByRole("button", { name: "Try again" }));

  await screen.findByRole("heading", { name: "Wanted pages" });
  expect(rule("Orphan pages").getByRole("link", { name: "Guide/Install/Linux.md" })).toBeTruthy();
  expect(health().queryByRole("alert")).toBeNull();
  expect(fixture.requests).toHaveLength(2);
  expect(fixture.requests.map((request) => request.headers.get("Authorization"))).toEqual([
    "Bearer at-1",
    "Bearer at-1",
  ]);
});

test("Recheck reads changes made elsewhere and replaces the computation's full local date and time", async () => {
  const user = userEvent.setup();
  const fixture = healthServer("standard");
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "Wanted pages" });
  expect(health().getByText(/Oct 31, 2026, 9:10\sAM/)).toBeTruthy();

  fixture.notebooks.set(notebookJSON.id, {
    mode: "standard",
    computed_at: "2026-11-01T17:25:00Z",
    stale_pages: 0,
    rules: { orphans: fixture.value.rules.orphans },
  });
  await user.click(health().getByRole("button", { name: "Recheck" }));

  const disclosure = await screen.findByText("No issues (3 checks)");
  expect(disclosure.closest("details")?.open).toBe(false);
  expect(rule("Wanted pages").getByText("0 issues")).toBeTruthy();
  expect(health().queryByRole("list", { name: "Wanted pages" })).toBeNull();
  expect(rule("Orphan pages").getByRole("link", { name: "Guide/Install/Linux.md" })).toBeTruthy();
  expect(health().getByText(/Nov 1, 2026, 5:25\sPM/)).toBeTruthy();
  expect(health().queryByText(/Oct 31, 2026/)).toBeNull();
  expect(fixture.requests).toHaveLength(2);
});

test.each([
  ["standard", ["Wanted pages", "Orphan pages", "Ambiguous links", "Invalid frontmatter"]],
  [
    "llm_wiki",
    [
      "Wanted pages",
      "Orphan pages",
      "Ambiguous links",
      "Invalid frontmatter",
      "Missing type",
      "Missing summary",
      "Undigested sources",
      "Invalid sources",
    ],
  ],
] as const)("a reader opens Health before History, with exactly the %s rules in order", async (mode, names) => {
  const user = userEvent.setup();
  const fixture = healthServer(mode);
  renderApp(home, fixture.server.app);
  const places = await screen.findByRole("navigation", { name: "More of Plans" });
  expect(
    within(places)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["Health", "History", "Trash"]);
  await user.click(within(places).getByRole("link", { name: "Health" }));

  await screen.findByRole("heading", { name: "Wanted pages" });
  expect(
    health()
      .getAllByRole("heading", { level: 2 })
      .map((heading) => heading.textContent)
  ).toEqual(names);
  expect(within(places).getByRole("link", { name: "Health" }).getAttribute("aria-current")).toBe("page");
  expect(document.title).toBe("Health · Plans · Nerve Wiki");
  expect(fixture.requests).toHaveLength(1);
});

test("previews keep every item branch, same-looking identities, source bytes and original creation times", async () => {
  const fixture = healthServer();
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "Wanted pages" });

  const wanted = rule("Wanted pages").getByRole("list", { name: "Wanted pages" });
  expect(within(wanted).getAllByRole("listitem")).toHaveLength(2);
  expect(within(wanted).getAllByText(target)).toHaveLength(2);
  expect(within(wanted).getByText("Referenced by 3 pages")).toBeTruthy();
  expect(within(wanted).getByText("Referenced by 2 pages")).toBeTruthy();
  expect(within(wanted).getAllByText("Target shortened.")).toHaveLength(2);
  expect(rule("Ambiguous links").getByText("Shared")).toBeTruthy();
  expect(rule("Ambiguous links").getByRole("link", { name: "Guide/Install/Linux.md" })).toBeTruthy();
  expect(rule("Invalid frontmatter").getByRole("link", { name: "Notes.md" })).toBeTruthy();
  expect(rule("Missing type").getByRole("link", { name: "Guide/Install/Linux.md" })).toBeTruthy();
  expect(rule("Missing summary").getByRole("link", { name: "Notes.md" })).toBeTruthy();
  const pending = rule("Undigested sources");
  expect(pending.getByRole("link", { name: "raw/Folder/Source.md" })).toBeTruthy();
  expect(pending.getByRole("link", { name: "raw/Folder/source.pdf" }).getAttribute("href")).toBe(
    `${home}/pages/${folder.id}`
  );
  expect(pending.getByText("17 B")).toBeTruthy();
  expect(pending.getByText("0 B")).toBeTruthy();
  expect(pending.getByText("Page")).toBeTruthy();
  expect(pending.getByText("Attachment")).toBeTruthy();
  expect(pending.getAllByText(/Oct 30, 2026, 8:01\sAM/)).toHaveLength(2);
  const invalid = rule("Invalid sources");
  expect(invalid.getAllByRole("listitem")).toHaveLength(2);
  expect(invalid.getAllByText("Broken")).toHaveLength(2);
  expect(invalid.getAllByText("This value is not a single link.")).toHaveLength(1);
  expect(
    health().getByText("7 pages have an outdated index. Ask the server administrator to run nervewiki reindex.")
  ).toBeTruthy();
});

test("zero rules are collected in one closed disclosure and keep their names, explanations and zero counts", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  fixture.value.rules = {
    orphans: fixture.value.rules.orphans,
    undigested_sources: fixture.value.rules.undigested_sources,
  };
  renderApp(healthPath, fixture.server.app);
  const disclosure = await screen.findByText("No issues (6 checks)");
  expect(disclosure.closest("details")?.open).toBe(false);
  // jsdom's role queries do not exclude the contents of a closed native details.
  expect(
    health()
      .getAllByRole("heading", { level: 2 })
      .filter((heading) => heading.closest("details") === null)
      .map((heading) => heading.textContent)
  ).toEqual(["Orphan pages", "Undigested sources"]);
  await user.click(disclosure);

  const clear = disclosure.closest("details");
  if (clear === null) throw new Error("zero checks need a disclosure");
  expect(clear.open).toBe(true);
  expect(
    within(clear)
      .getAllByRole("heading", { level: 2 })
      .map((heading) => heading.textContent)
  ).toEqual([
    "Wanted pages",
    "Ambiguous links",
    "Invalid frontmatter",
    "Missing type",
    "Missing summary",
    "Invalid sources",
  ]);
  expect(within(clear).getAllByText("0 issues")).toHaveLength(6);
  expect(within(clear).getByText("Wiki pages that need a summary property.")).toBeTruthy();
  expect(within(clear).queryByRole("button")).toBeNull();
});

test("a completely clear notebook still names all eight applicable checks when expanded", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  fixture.value.rules = {};
  fixture.value.stale_pages = 0;
  renderApp(healthPath, fixture.server.app);
  const disclosure = await screen.findByText("No issues (8 checks)");
  expect(disclosure.closest("details")?.open).toBe(false);
  await user.click(disclosure);
  expect(
    health()
      .getAllByRole("heading", { level: 2 })
      .map((heading) => heading.textContent)
  ).toEqual([
    "Wanted pages",
    "Orphan pages",
    "Ambiguous links",
    "Invalid frontmatter",
    "Missing type",
    "Missing summary",
    "Undigested sources",
    "Invalid sources",
  ]);
  expect(health().getAllByText("0 issues")).toHaveLength(8);
  expect(health().queryByRole("list")).toBeNull();
});

test("the health snapshot and its explanations are available in Chinese, including complete dates", async () => {
  const fixture = healthServer();
  fixture.server.app.preferences.setLocale("zh-CN");
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "待建页面" });
  expect(screen.getByRole("heading", { level: 1, name: "健康" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "重新检查" })).toBeTruthy();
  expect(screen.getByText("2026年10月31日 09:10")).toBeTruthy();
  expect(screen.getAllByText("2026年10月30日 08:01")).toHaveLength(2);
  expect(screen.getAllByText("目标已截短。")).toHaveLength(2);
  expect(screen.getByText("这个值不是单个链接。")).toBeTruthy();
  expect(screen.getByText("7 篇页面的索引已过时，请服务器管理员运行 nervewiki reindex。")).toBeTruthy();
  expect(screen.getByRole("list", { name: "未消化的来源" })).toBeTruthy();
});

test("a pending tree does not hide the summary: target and counts are visible while node paths load", async () => {
  let release!: (response: Response) => void;
  const held = new Promise<Response>((resolve) => (release = resolve));
  const fixture = healthServer("llm_wiki", { [nodesRoute]: () => held });
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "Wanted pages" });
  expect(rule("Wanted pages").getAllByText(target)).toHaveLength(2);
  expect(rule("Orphan pages").getByText("1 issue")).toBeTruthy();
  expect(rule("Orphan pages").getByText("Loading…")).toBeTruthy();
  expect(rule("Orphan pages").queryByRole("link")).toBeNull();
  await act(async () => release(json({ data: nodes })));
  expect(await rule("Orphan pages").findByRole("link", { name: "Guide/Install/Linux.md" })).toBeTruthy();
  expect(fixture.requests).toHaveLength(1);
});

test("tree failure has its own retry and preserves health results without running lint again", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  fixture.server.nodesDown = true;
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "Wanted pages" });
  expect(await health().findByRole("alert")).toHaveProperty(
    "textContent",
    "Cannot reach the server. Check the connection and try again."
  );
  expect(rule("Wanted pages").getAllByRole("listitem")).toHaveLength(2);
  fixture.server.nodesDown = false;
  await user.click(health().getByRole("button", { name: "Try again" }));
  expect(await rule("Orphan pages").findByRole("link", { name: "Guide/Install/Linux.md" })).toBeTruthy();
  expect(health().queryByRole("alert")).toBeNull();
  expect(fixture.requests).toHaveLength(1);
});

test("different missing nodes share one tree reread; still absent results retain their items and counts without UUIDs", async () => {
  const user = userEvent.setup();
  let reads = 0;
  const fixture = healthServer("llm_wiki", {
    [nodesRoute]: () => {
      reads += 1;
      return json({ data: nodes.filter((node) => node.id !== linux.id && node.id !== asset.id) });
    },
  });
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "Wanted pages" });
  await waitFor(() => expect(reads).toBe(2));
  expect(rule("Orphan pages").getAllByRole("listitem")).toHaveLength(1);
  expect(rule("Orphan pages").getByText("1 issue")).toBeTruthy();
  expect(rule("Orphan pages").getByText("This node is no longer available.")).toBeTruthy();
  expect(rule("Undigested sources").getAllByRole("listitem")).toHaveLength(2);
  expect(rule("Undigested sources").getByText("This node is no longer available.")).toBeTruthy();
  expect(screen.queryByText(linux.id)).toBeNull();
  expect(screen.queryByText(asset.id)).toBeNull();
  await user.click(health().getByRole("button", { name: "Recheck" }));
  await waitFor(() => expect(fixture.requests).toHaveLength(2));
  expect(reads).toBe(2);
});

test("one shared tree reread resolves paths that were missing from the first tree", async () => {
  let reads = 0;
  const fixture = healthServer("llm_wiki", {
    [nodesRoute]: () => {
      reads += 1;
      return json({ data: reads === 1 ? nodes.filter((node) => node.id !== linux.id && node.id !== asset.id) : nodes });
    },
  });
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "Wanted pages" });
  expect(await rule("Orphan pages").findByRole("link", { name: "Guide/Install/Linux.md" })).toBeTruthy();
  expect(await rule("Undigested sources").findByRole("link", { name: "raw/Folder/source.pdf" })).toBeTruthy();
  expect(reads).toBe(2);
  expect(fixture.requests).toHaveLength(1);
});

test("a capped rule shows its localized lower bound while displaying the given five previews", async () => {
  const preview = ["a", "b", "c", "d", "e"].map((letter) => ({
    rule: "wanted" as const,
    item_key: `w:${letter.repeat(64)}`,
    cursor: cursor("wanted", { c: 2, k: wire(`Missing ${letter}`), h: letter.repeat(64) }),
    target: `Missing ${letter}`,
    target_cut: false,
    pages: 2,
  }));
  const fixture = healthServer("standard", {
    [summaryRoute]: () =>
      json({
        mode: "standard",
        computed_at: computed,
        stale_pages: 0,
        rules: [
          { rule: "wanted", count: 10000, count_capped: true, items: preview, next_cursor: preview[4]?.cursor },
          { rule: "orphans", count: 0, count_capped: false, items: [], next_cursor: null },
          { rule: "ambiguous", count: 0, count_capped: false, items: [], next_cursor: null },
          { rule: "invalid_frontmatter", count: 0, count_capped: false, items: [], next_cursor: null },
        ],
      }),
  });
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "Wanted pages" });
  expect(rule("Wanted pages").getByText("10,000+ issues")).toBeTruthy();
  expect(rule("Wanted pages").getAllByRole("listitem")).toHaveLength(5);
  expect(rule("Wanted pages").getByText("Missing e")).toBeTruthy();
});

test("the results output already exists while the summary is loading and stays for its announcement", async () => {
  let release!: (response: Response) => void;
  const held = new Promise<Response>((resolve) => (release = resolve));
  const fixture = healthServer("llm_wiki", { [summaryRoute]: () => held });
  renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { level: 1, name: "Health" });
  const output = health().getByRole("status", { name: "Health results" });
  const answer = lintRoutes(fixture.notebooks)[summaryRoute];
  if (answer === undefined) throw new Error("missing summary fake");
  await act(async () =>
    release(await answer(new Request(`http://nervewiki.test/api/v0/notebooks/${notebookJSON.id}/lint`)))
  );
  await waitFor(() => expect(output.textContent).toBe("8 checks completed."));
  expect(health().getByRole("status", { name: "Health results" })).toBe(output);
});
