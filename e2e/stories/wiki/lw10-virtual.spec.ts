/* oxlint-disable no-await-in-loop -- Each scoped client and transfer consumes the preceding real response. */
import { expectImported, treeOf } from "../../fixtures/assert/transfer";
import { errorOf, mcpClient, readPage, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, listNodes, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { archiveAt, downloadArchive, endedJob, startExport, startImport } from "../../fixtures/transfer";
import { newTeam } from "../../fixtures/workspaces";
import { unzip } from "../../fixtures/zip";

// S1 exercises the actual API/MCP and downloaded archive. The tree links and
// browser navigation are added with the S3 routes, not simulated here.
test("LW10: both MCP entries read generated files and the real ZIP reimports only real pages", async ({
  api,
  db,
  nervewiki,
}, info) => {
  const team = await newTeam(api, info);
  const wiki = await createNotebook(api, team.pat, team.workspace.slug, "VirtualWiki", "none", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  const guide = await createPage(
    api,
    team.pat,
    wiki.id,
    "Guide",
    null,
    "---\ntype: note\nsummary: guide\n---\nreal guide"
  );
  await createPage(api, team.pat, wiki.id, "index", guide.id, "real nested index");
  for (const scope of ["notebook", "account"] as const) {
    const agent = await mcpClient(nervewiki.baseURL, team.pat, scope === "notebook" ? { notebook: wiki.id } : {});
    const target = scope === "account" ? { notebook: wiki.id } : {};
    const listing = resultOf<{ items: { path: string; virtual?: boolean; version?: string; byte_size?: number }[] }>(
      await agent.call("list", target),
      `${scope} root list`
    );
    expect(listing.items.slice(0, 2)).toEqual([
      { path: "index.md", kind: "page", has_children: false, virtual: true },
      { path: "log.md", kind: "page", has_children: false, virtual: true },
    ]);
    for (const name of ["index.md", "log.md"]) {
      const read = await readPage(agent, { ...target, path: name });
      expect(read.meta.virtual).toBe(true);
      expect(read.meta).not.toHaveProperty("version");
      expect(read.text).toContain("Guide.md");
      expect(errorOf(await agent.call("write", { ...target, path: name, content: "overwrite" }), name).code).toBe(
        "page.virtual_file"
      );
    }
    expect((await readPage(agent, { ...target, path: "Guide/index.md" })).text).toBe("real nested index");
    const grep = resultOf<{
      hits: { path: string }[];
      next_cursor: string | null;
      building: boolean;
      incomplete: boolean;
    }>(await agent.call("grep", { ...target, pattern: "^Showing|^real" }), `${scope} real-content grep`);
    expect(grep.hits.map((hit) => hit.path).toSorted()).toEqual(["Guide.md", "Guide/index.md"]);
    expect([grep.next_cursor, grep.building, grep.incomplete]).toEqual([null, false, false]);
  }
  const whole = await endedJob(api, team.pat, (await startExport(api, team.pat, wiki.id)).id);
  expect(whole.state).toBe("succeeded");
  if (!whole.download) throw new Error("the actual export has no signed download");
  const response = await downloadArchive(nervewiki.baseURL, whole.download.url);
  expect(response.status).toBe(200);
  const archive = Buffer.from(await response.arrayBuffer());
  const entries = new Map(unzip(archive).map((entry) => [entry.name, entry.data.toString("utf8")]));
  for (const name of ["index.md", "log.md"]) expect(entries.get(`VirtualWiki/${name}`)).toContain("](<Guide.md>)");
  expect(entries.get("VirtualWiki/Guide/index.md")).toBe("real nested index");
  const meta = JSON.parse(entries.get("VirtualWiki/.nerve/meta.json") ?? "null") as { contributed: string[] };
  expect(meta.contributed).toEqual(["index.md", "log.md"]);

  const subtree = await endedJob(api, team.pat, (await startExport(api, team.pat, wiki.id, guide.id)).id);
  expect(subtree.state).toBe("succeeded");
  if (!subtree.download) throw new Error("the subtree has no signed download");
  const sub = await archiveAt(nervewiki.baseURL, subtree.download.url);
  const subMeta = JSON.parse(
    sub.entries.find((e) => e.name === "Guide/.nerve/meta.json")?.data.toString("utf8") ?? "null"
  ) as { contributed?: string[] };
  expect(subMeta.contributed ?? []).toEqual([]);
  expect(sub.entries.some((e) => e.name === "Guide/index.md")).toBe(false);
  expect(sub.entries.find((e) => e.name === "Guide/Guide/index.md")?.data.toString("utf8")).toBe("real nested index");

  for (const mode of ["llm_wiki", "standard"] as const) {
    const destination = await createNotebook(api, team.pat, team.workspace.slug, `Reimport ${mode}`, "none", {
      mode,
      templateLocale: "en",
    });
    const imported = await endedJob(api, team.pat, (await startImport(api, team.pat, destination.id, archive)).id);
    expect(imported.state).toBe("succeeded");
    const nodes = await listNodes(api, team.pat, destination.id);
    expect(
      nodes
        .filter((n) => n.parent_id === null)
        .map((n) => n.name)
        .toSorted()
    ).toEqual(mode === "llm_wiki" ? ["AGENTS", "AGENTS 2", "Guide", "raw", "raw 2"] : ["AGENTS", "Guide", "raw"]);
    const importedGuide = nodes.find((n) => n.parent_id === null && n.name === "Guide");
    if (!importedGuide) throw new Error("the real Guide was not imported");
    expect((await readContent(api, team.pat, importedGuide.id)).content).toBe(
      "---\ntype: note\nsummary: guide\n---\nreal guide"
    );
    expect(
      (await treeOf(db, nervewiki.storageDir, destination.id)).filter(
        (entry) => entry.path === "Guide" || entry.path.startsWith("Guide/")
      )
    ).toEqual([
      { path: "Guide", kind: "page", content: "---\ntype: note\nsummary: guide\n---\nreal guide" },
      { path: "Guide/index", kind: "page", content: "real nested index" },
    ]);
    await expectImported(db, nervewiki.storageDir, imported.id, {
      creatorId: team.adminId,
      client: "api",
      root: null,
      state: "succeeded",
      failure: null,
      done: 4,
      total: 4,
      counts: { pages: 4, attachments: 0, renamed: mode === "llm_wiki" ? 2 : 0, skipped: 0 },
    });
  }
});

test("LW10: a standard notebook keeps its real index and log in reads and exports", async ({
  api,
  nervewiki,
}, info) => {
  const team = await newTeam(api, info);
  const notebook = await createNotebook(api, team.pat, team.workspace.slug, "Ordinary", "none");
  for (const name of ["index", "log"]) await createPage(api, team.pat, notebook.id, name, null, `real ${name}`);
  const agent = await mcpClient(nervewiki.baseURL, team.pat, { notebook: notebook.id });
  for (const name of ["index", "log"]) {
    const read = await readPage(agent, { path: `${name}.md` });
    expect(read.meta.virtual).not.toBe(true);
    expect(read.meta.version).toBeTruthy();
    expect(read.text).toBe(`real ${name}`);
  }
  const job = await endedJob(api, team.pat, (await startExport(api, team.pat, notebook.id)).id);
  expect(job.state).toBe("succeeded");
  if (!job.download) throw new Error("ordinary export has no archive");
  const { entries } = await archiveAt(nervewiki.baseURL, job.download.url);
  for (const name of ["index", "log"])
    expect(entries.find((e) => e.name === `Ordinary/${name}.md`)?.data.toString("utf8")).toBe(`real ${name}`);
});
