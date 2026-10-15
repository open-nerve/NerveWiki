import { expect, test } from "vitest";

import type { SearchPage, TreeNode } from "../services/page.service";
import { byRoute } from "./fakes";
import { assetJSON, assetNode, pageNode, pageServer } from "./page-server";
import { searchRoutes } from "./search-server";

// The fake search (search-server.ts) answers as the server does where the pages look (M8/P5 design 4.5): its codes in
// the server's order, its two tiers, and a walk that leaves a page short.

/** changed is node, changed minute minutes past 8 on Oct 5, 2026. */
const changed = (node: TreeNode, minute: number): TreeNode => ({
  ...node,
  updated_at: `2026-10-05T08:${minute.toString().padStart(2, "0")}:00Z`,
});

/** searchOf answers GET search?query of server's workspace slug. */
async function searchOf(server: ReturnType<typeof pageServer>, query: string, slug = "lab"): Promise<Response> {
  const answer = byRoute(searchRoutes(server, (node) => assetJSON(node)));
  return answer(new Request(`http://nervewiki.test/api/v0/workspaces/${slug}/search?${query}`));
}

/** problemOf is the status and code of a refusal, and its field's code. */
async function problemOf(answer: Response): Promise<string> {
  const body = (await answer.json()) as { code: string; errors?: { field: string; code: string }[] };
  const field = body.errors?.[0];
  return [answer.status.toString(), body.code, ...(field === undefined ? [] : [`${field.field} ${field.code}`])].join(
    " "
  );
}

/** namesOf is the names of a page of hits, and its cursor. */
async function namesOf(answer: Response): Promise<{ names: string[]; next: string | null; page: SearchPage }> {
  const page = (await answer.json()) as SearchPage;
  return { names: page.data.map((hit) => hit.node.name), next: page.next_cursor, page };
}

test("the codes come in the server's order: the binding's 400, the cursor's, q's 422, the workspace's 404, the notebook's, the limit's 422", async () => {
  const server = pageServer();
  const other = "0199a2b4-0000-7000-8000-0000000000c9";
  // Each step mends the first problem of the one before.
  const steps: [string, string, string][] = [
    ["else", "q=&cursor=bad&notebook_id=nope&limit=x", "400 bad_request"],
    ["else", `q=&cursor=bad&notebook_id=${other}&limit=x`, "400 bad_request"],
    ["else", `q=&cursor=bad&notebook_id=${other}&limit=0`, "400 bad_request"],
    ["else", `q=&notebook_id=${other}&limit=0`, "422 validation_failed q required"],
    ["else", `q=a&notebook_id=${other}&limit=0`, "404 workspace.not_found"],
    ["lab", `q=a&notebook_id=${other}&limit=0`, "404 notebook.not_found"],
    ["lab", "q=a&limit=0", "422 validation_failed limit out_of_range"],
    ["lab", "q=a&limit=51", "422 validation_failed limit out_of_range"],
  ];
  const codes = await Promise.all(steps.map(async ([slug, query]) => problemOf(await searchOf(server, query, slug))));
  expect(codes).toEqual(steps.map(([, , code]) => code));
  const qs: [string, string][] = [
    ["", "required"],
    ["q=%20%E3%80%80", "required"],
    ["q=a%00", "invalid_format"],
    [`q=${"数".repeat(86)}`, "too_long"],
    ["q=a b c d e f g h i", "too_long"],
    [`q=${"a".repeat(65)}`, "too_long"],
    ["q=!!%20%E2%80%94", "not_allowed"],
  ];
  const refused = await Promise.all(qs.map(async ([query]) => problemOf(await searchOf(server, query))));
  expect(refused).toEqual(qs.map(([, code]) => `422 validation_failed q ${code}`));
  // At the bounds: 256 bytes, 8 terms (one of them twice, in another case), 64 letters.
  const bounds = [`q=${"数".repeat(85)}a`, "q=a b c d e f g h A", `q=${"a".repeat(64)}`];
  const answers = await Promise.all(bounds.map((query) => searchOf(server, query)));
  expect(answers.map((answer) => answer.status)).toEqual([200, 200, 200]);
});

test("the names' hits come first, then the contents', each newest first, a page of limit at a time; the first page's count", async () => {
  const titled = [changed(pageNode(11, "Go 数据库", undefined), 10), changed(pageNode(12, "数据库 notes"), 30)];
  const asset = changed(assetNode(13, "数据库.pdf", titled[0]), 20);
  const bodies = [
    changed(pageNode(14, "Storage"), 50),
    changed(pageNode(15, "Index"), 40),
    changed(pageNode(16, "Else"), 45),
  ];
  const server = pageServer({ nodes: [...titled, asset, ...bodies] });
  server.contents.set(bodies[0]?.id ?? "", { content: "数据库的索引", revision: 1 });
  server.contents.set(bodies[1]?.id ?? "", { content: "全文检索与数据库", revision: 1 });

  const first = await namesOf(await searchOf(server, "q=数据库&limit=2"));
  expect(first.names).toEqual(["数据库 notes", "数据库.pdf"]);
  expect(first.page.count).toEqual({ value: 5, kind: "about" });
  const second = await namesOf(
    await searchOf(server, `q=数据库&limit=2&cursor=${encodeURIComponent(first.next ?? "")}`)
  );
  expect(second.names).toEqual(["Go 数据库", "Storage"]);
  expect(second.page.count).toBeNull();
  const third = await namesOf(
    await searchOf(server, `q=数据库&limit=2&cursor=${encodeURIComponent(second.next ?? "")}`)
  );
  expect(third.names).toEqual(["Index"]);
  expect(third.next).toBeNull();
  // All of it on one page: exact.
  expect((await namesOf(await searchOf(server, "q=数据库"))).page.count).toEqual({ value: 5, kind: "exact" });
});

test("a single CJK character is looked for in names alone; letters from three as a substring, one or two as a whole word", async () => {
  const nodes = [
    changed(pageNode(11, "仓库"), 10),
    changed(pageNode(12, "Notes"), 20),
    changed(pageNode(13, "English"), 30),
    changed(pageNode(14, "Words"), 40),
  ];
  const server = pageServer({ nodes });
  server.contents.set(nodes[1]?.id ?? "", { content: "代码在仓库里。", revision: 1 });
  server.contents.set(nodes[2]?.id ?? "", { content: "A rapid tool, with AI.", revision: 1 });
  server.contents.set(nodes[3]?.id ?? "", { content: "He said so.", revision: 1 });

  expect((await namesOf(await searchOf(server, "q=库"))).names).toEqual(["仓库"]);
  expect((await namesOf(await searchOf(server, "q=仓库"))).names).toEqual(["仓库", "Notes"]);
  expect((await namesOf(await searchOf(server, "q=api"))).names).toEqual(["English"]);
  expect((await namesOf(await searchOf(server, "q=ai"))).names).toEqual(["English"]);
  expect((await namesOf(await searchOf(server, "q=aid"))).names).toEqual(["Words"]);
});

test("a walk short of the candidates leaves a page short of limit, even empty, with a cursor below the last walked", async () => {
  const nodes = [
    changed(pageNode(11, "One"), 30),
    changed(pageNode(12, "Two"), 20),
    changed(pageNode(13, "Three"), 10),
  ];
  const server = pageServer({ nodes });
  server.contents.set(nodes[1]?.id ?? "", { content: "检索", revision: 1 });
  server.contents.set(nodes[2]?.id ?? "", { content: "检索", revision: 1 });
  server.searchWalk = 1;

  const first = await namesOf(await searchOf(server, "q=检索"));
  expect(first.names).toEqual([]);
  expect(first.next).not.toBeNull();
  expect(first.page.count).toBeNull();
  const second = await namesOf(await searchOf(server, `q=检索&cursor=${encodeURIComponent(first.next ?? "")}`));
  expect(second.names).toEqual(["Two"]);
  expect(second.next).not.toBeNull();
  const third = await namesOf(await searchOf(server, `q=检索&cursor=${encodeURIComponent(second.next ?? "")}`));
  expect(third.names).toEqual(["Three"]);
  expect(third.next).toBeNull();
});
