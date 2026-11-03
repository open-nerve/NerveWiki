import { expect, test } from "vitest";

import { PageService } from "../services/page.service";
import { byRoute, notebookJSON } from "./fakes";
import { created, historyRoutes } from "./history-server";
import { ada, assetNode, bob, pageNode, pageServer } from "./page-server";
import { trashed } from "./trash-server";

const emptyFilter = { actor: "", client: "", page: "" };

test("ingest previews retain their first seq positions, including trashed, purged and foreign nodes", async () => {
  const sources = Array.from({ length: 25 }, (_, i) => assetNode(100 + i, "Source " + i.toString()));
  const output = pageNode(130, "Output");
  const server = pageServer({ role: "reader", nodes: [...sources, output] });
  const page = new PageService(server.app.session.public);
  const sourceIds = sources.map((source) => source.id);
  const marked = server.log("ingested", { ingestCount: 25, ingestSources: sourceIds, changes: [created(output)] });
  const ordinary = server.log("ordinary");

  expect((await page.changeset(ordinary.id)).ingest_sources).toEqual([]);
  expect((await page.changeset(ordinary.id)).ingest_count).toBe(0);
  server.nodes = server.nodes.filter((node) => node.id !== sources[0]?.id && node.id !== sources[2]?.id);
  const first = sources[0];
  if (first === undefined) throw new Error("missing source fixture");
  server.trash.set(first.id, trashed([first], [first], 1, ada));
  const foreign = sources[4];
  if (foreign === undefined) throw new Error("missing foreign source fixture");
  foreign.notebook_id = "0199a2b4-0000-7000-8000-000000000999";
  const expected = sourceIds.map((id, i) => ([0, 2, 4].includes(i) ? null : id));

  const list = await page.changesets(notebookJSON.id, emptyFilter);
  expect(list.data.map((row) => row.id)).toEqual([ordinary.id, marked.id]);
  expect(list.data[0]).toMatchObject({ ingest_count: 0, ingest_sources: [] });
  expect(list.data[1]).toMatchObject({ kind: "edit", ingest_count: 25, ingest_sources: expected.slice(0, 3) });
  expect(await page.changeset(marked.id)).toMatchObject({
    kind: "edit",
    ingest_count: 25,
    ingest_sources: expected.slice(0, 20),
  });
  expect(marked.ingestSources).toEqual(sourceIds);
  expect(marked.ingestCount).toBe(25);
});

test("ingest=true filters counts and intersects actor, client and changed page, while false and omitted keep all", async () => {
  const source = assetNode(140, "Source");
  const output = pageNode(141, "Output");
  const server = pageServer({ role: "reader", nodes: [source, output] });
  const page = new PageService(server.app.session.public);
  const marked = server.log("marked", {
    ingestCount: 1,
    ingestSources: [source.id],
    client: "mcp:claude",
    changes: [created(output)],
  });
  const imported = server.log("ordinary import", { kind: "import", changes: [created(output)] });
  const other = server.log("Bob marked", {
    ingestCount: 1,
    ingestSources: [source.id],
    by: bob,
    client: "api",
    changes: [created(output)],
  });
  const ordinary = server.log("ordinary edit", { changes: [created(output)] });
  const unfiltered = await Promise.all([
    page.changesets(notebookJSON.id, emptyFilter),
    page.changesets(notebookJSON.id, { ...emptyFilter, ingest: false }),
  ]);
  for (const result of unfiltered) {
    expect(result.data.map((row) => row.id)).toEqual([ordinary.id, other.id, imported.id, marked.id]);
  }
  expect((await page.changesets(notebookJSON.id, { ...emptyFilter, ingest: true })).data.map((row) => row.id)).toEqual([
    other.id,
    marked.id,
  ]);
  expect(
    (
      await page.changesets(notebookJSON.id, { actor: ada.user_id, client: "mcp", page: output.id, ingest: true })
    ).data.map((row) => row.id)
  ).toEqual([marked.id]);
  expect((await page.changesets(notebookJSON.id, { ...emptyFilter, page: source.id, ingest: true })).data).toEqual([]);
});

for (const value of ["1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false", "False"]) {
  test("fake HTTP parses Go's boolean spelling " + JSON.stringify(value), async () => {
    const server = pageServer({ role: "reader" });
    const marked = server.log("marked", { ingestCount: 1 });
    const ordinary = server.log("ordinary");
    const answer = byRoute(historyRoutes(server, "reader"));
    const response = await answer(
      new Request("http://nervewiki.test/api/v0/notebooks/" + notebookJSON.id + "/changesets?ingest=" + value)
    );
    expect(response.status).toBe(200);
    const body: { data: { id: string }[] } = await response.json();
    expect(body.data.map((row) => row.id)).toEqual(
      ["1", "t", "T", "TRUE", "true", "True"].includes(value) ? [marked.id] : [ordinary.id, marked.id]
    );
  });
}
for (const value of ["", "yes", "2", "Truee"]) {
  test("fake HTTP refuses invalid ingest before notebook lookup: " + JSON.stringify(value), async () => {
    const server = pageServer({ role: "reader" });
    server.notebookGone = true;
    const response = await byRoute(historyRoutes(server, "reader"))(
      new Request("http://nervewiki.test/api/v0/notebooks/" + notebookJSON.id + "/changesets?ingest=" + value)
    );
    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({ code: "bad_request" });
  });
}
