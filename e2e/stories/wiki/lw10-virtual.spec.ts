/* oxlint-disable no-await-in-loop -- Each scoped client and transfer consumes the preceding real response. */
import { expectIngestOf, expectNoIngestIn } from "../../fixtures/assert/page";
import { pngBytes, uploadAsset } from "../../fixtures/assets";
import { changesetOfRevision, entryOfVirtualLog, listChangesets } from "../../fixtures/history";
import { expectImported, sha256Of, treeOf } from "../../fixtures/assert/transfer";
import { errorOf, mcpClient, readPage, resultOf } from "../../fixtures/mcp";
import { bearer, emailFor } from "../../fixtures/auth";
import { joinOnboarded } from "../../fixtures/invitations";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, listNodes, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { archiveAt, downloadArchive, endedJob, startExport, startImport } from "../../fixtures/transfer";
import { pageHeading, pageTree, wikiPagePath } from "../../fixtures/wiki-pages";
import { downloadFrom, exportWith, jobRow } from "../../fixtures/wiki-transfer";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";
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

test("LW10: real ingestion reaches log.md and whole export", async ({ api, db, nervewiki }, info) => {
  const team = await newTeam(api, info);
  const wiki = await createNotebook(api, team.pat, team.workspace.slug, "IngestExport", "none", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  if (!wiki.source_root_id) throw new Error("the template has no source root");
  const raw = wiki.source_root_id;
  const roots = await listNodes(api, team.pat, wiki.id);
  const agents = roots.find((node) => node.parent_id === null && node.name === "AGENTS");
  if (!agents) throw new Error("the template has no real AGENTS page");
  await createPage(api, team.pat, wiki.id, "N", null, "sibling file\n");
  const source = await createPage(api, team.pat, wiki.id, "Leaf", raw, "page source\n");
  const asset = await uploadAsset(api, team.pat, wiki.id, { name: "clip.png", bytes: pngBytes }, raw);
  // With no body or inbound link to the source root itself, export uses a
  // folder. Its real N.md title collides with the sibling page's N.md file.
  await writeContent(api, team.pat, raw, {
    content: "",
    base_revision: (await readContent(api, team.pat, raw)).revision,
  });
  await writeContent(api, team.pat, agents.id, {
    content: "Instructions without links.\n",
    base_revision: (await readContent(api, team.pat, agents.id)).revision,
  });
  const writer = await mcpClient(nervewiki.baseURL, team.pat, { notebook: wiki.id });
  resultOf(await writer.call("move", { from: "raw.md", to: "N.md.md" }), "rename the real source root");
  const summary = await createPage(api, team.pat, wiki.id, "Summary", null, "before\n");
  const written = resultOf<{ changed: boolean; changeset_id: string | null; node_id: string }>(
    await writer.call("write", {
      path: "Summary.md",
      base_version: `${summary.id}:${summary.revision}`,
      content: "[[N.md/Leaf]]\n![](N.md/clip.png)\n",
    }),
    "produce a page and an asset mark"
  );
  expect([written.changed, written.node_id]).toEqual([true, summary.id]);
  if (!written.changeset_id) throw new Error("the real source write has no changeset");
  const changeset = written.changeset_id;
  const ids = [source.id, asset.id].toSorted();
  await expectIngestOf(db, changeset, wiki.id, ids);
  const readPaths = new Map([
    [source.id, "N.md/Leaf.md"],
    [asset.id, "N.md/clip.png"],
  ]);
  const zipPaths = new Map([
    [source.id, "N.md 2/Leaf.md"],
    [asset.id, "N.md 2/clip.png"],
  ]);
  for (const scope of ["notebook", "account"] as const) {
    const agent = await mcpClient(nervewiki.baseURL, team.pat, scope === "notebook" ? { notebook: wiki.id } : {});
    const target = scope === "account" ? { notebook: wiki.id } : {};
    const read = await readPage(agent, { ...target, path: "log.md" });
    expect([read.meta.virtual, read.meta.next_offset, read.meta.next_column]).toEqual([true, null, null]);
    const entry = entryOfVirtualLog(read.text, changeset);
    expect(entry[0]).toMatch(/; ingests: 2$/);
    expect(entry.filter((line) => line.startsWith("  - source: "))).toEqual(
      ids.map((id) => {
        const path = readPaths.get(id);
        return `  - source: [${path}](<${path}>)`;
      })
    );
  }
  const whole = await endedJob(api, team.pat, (await startExport(api, team.pat, wiki.id)).id);
  expect(whole.state).toBe("succeeded");
  if (!whole.download) throw new Error("the real ingest export has no download");
  const response = await downloadArchive(nervewiki.baseURL, whole.download.url);
  expect(response.status).toBe(200);
  const archive = Buffer.from(await response.arrayBuffer());
  const files = new Map(unzip(archive).map((entry) => [entry.name, entry.data]));
  expect(files.get("IngestExport/N.md")?.toString("utf8")).toBe("sibling file\n");
  expect(files.has("IngestExport/N.md 2/")).toBe(true);
  expect(files.get("IngestExport/N.md 2/Leaf.md")?.toString("utf8")).toBe("page source\n");
  expect(files.get("IngestExport/N.md 2/clip.png")).toEqual(Buffer.from(pngBytes));
  const log = files.get("IngestExport/log.md")?.toString("utf8");
  if (log === undefined) throw new Error("the real whole ZIP has no log contribution");
  const entry = entryOfVirtualLog(log, changeset);
  expect(entry[0]).toMatch(/; ingests: 2$/);
  expect(entry.filter((line) => line.startsWith("  - source: "))).toEqual(
    ids.map((id) => {
      const path = zipPaths.get(id);
      return `  - source: [${path}](<${path}>)`;
    })
  );
  const meta = JSON.parse(files.get("IngestExport/.nerve/meta.json")?.toString("utf8") ?? "null") as {
    contributed: string[];
  };
  expect(meta.contributed).toEqual(["index.md", "log.md"]);
  expect(files.get("IngestExport/index.md")?.toString("utf8")).toContain("Source area: N.md 2/ — 2 sources.");

  for (const mode of ["llm_wiki", "standard"] as const) {
    const destination = await createNotebook(api, team.pat, team.workspace.slug, `Ingest reimport ${mode}`, "none", {
      mode,
      templateLocale: "en",
    });
    const imported = await endedJob(api, team.pat, (await startImport(api, team.pat, destination.id, archive)).id);
    expect(imported.state).toBe("succeeded");
    const nodes = await listNodes(api, team.pat, destination.id);
    expect(
      nodes
        .filter((node) => node.parent_id === null)
        .map((node) => node.name)
        .toSorted()
    ).toEqual(
      mode === "llm_wiki"
        ? ["AGENTS", "AGENTS 2", "N", "N.md 2", "Summary", "raw"]
        : ["AGENTS", "N", "N.md 2", "Summary"]
    );
    expect(
      (await treeOf(db, nervewiki.storageDir, destination.id))
        .filter((node) => node.path === "N.md 2" || node.path.startsWith("N.md 2/"))
        .toSorted((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0))
    ).toEqual([
      { path: "N.md 2", kind: "page", content: "" },
      { path: "N.md 2/Leaf", kind: "page", content: "page source\n" },
      { path: "N.md 2/clip.png", kind: "asset", mime: "image/png", sha256: sha256Of(pngBytes) },
    ]);
    await expectImported(db, nervewiki.storageDir, imported.id, {
      creatorId: team.adminId,
      client: "api",
      root: null,
      state: "succeeded",
      failure: null,
      done: 6,
      total: 6,
      counts: { pages: 5, attachments: 1, renamed: mode === "llm_wiki" ? 1 : 0, skipped: 0 },
    });
    const history = await listChangesets(api, team.pat, destination.id, { limit: 100 });
    expect(history.response.status).toBe(200);
    expect(history.data?.data.some((row) => row.kind === "import")).toBe(true);
    for (const row of history.data?.data ?? []) expect([row.ingest_count, row.ingest_sources]).toEqual([0, []]);
    await expectNoIngestIn(db, destination.id);
  }
});

// EW02: actual reader anchors and ordinary tree Links; the writer downloads
// the real contributor's ZIP through the existing export dialog and job row.
test("LW10 (page): virtual tree links and unresolved reader links open Directory and History, and a real ZIP contains both", async ({
  api,
  db,
  signedInPage,
  anotherPage,
}, info) => {
  const team = await newOnboardedTeam(api, info);
  const wiki = await createNotebook(api, team.pat, team.workspace.slug, "Browser virtual", "none", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  if (wiki.source_root_id === null) throw new Error("the LLM template has no source root");
  const evidence = await createPage(api, team.pat, wiki.id, "Evidence", wiki.source_root_id, "Evidence bytes.\n");
  const body =
    "---\ntype: note\nsummary: real guide\n---\nRead [[raw/Evidence]].\n\n[[/index|Open directory]] · [[/log|Open history]]\n";
  const guide = await createPage(api, team.pat, wiki.id, "Guide", null, body);
  const changeset = await changesetOfRevision(api, team.pat, guide.id, guide.revision);
  await expectIngestOf(db, changeset, wiki.id, [evidence.id]);
  const readerTokens = await joinOnboarded(api, team.pat, team.workspace.slug, emailFor(info, "reader"), "guest");
  const me = await api.GET("/api/v0/me", { headers: bearer(readerTokens.access_token) });
  if (!me.data) throw new Error("the reader has no user identity");
  await addedNotebookMember(api, team.pat, wiki.id, me.data.id, "reader");
  const reader = await anotherPage(readerTokens);
  const forbiddenCalls: string[] = [];
  reader.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    if (
      path.endsWith("/link-landing") ||
      (request.method() === "POST" && path === `/api/v0/notebooks/${wiki.id}/pages`)
    )
      forbiddenCalls.push(`${request.method()} ${path}`);
  });
  const home = `/${team.workspace.slug}/notebooks/${wiki.id}`;
  await reader.goto(wikiPagePath(team.workspace.slug, wiki.id, guide.id));
  await expect(pageHeading(reader, "Guide")).toBeVisible();
  const tree = pageTree(reader, wiki.name);
  // Edit waits for the tree as well as the title: asked sooner, a writer has none either.
  await expect(tree.getByRole("link", { name: "Guide", exact: true })).toBeVisible();
  await expect(reader.getByRole("main").getByRole("button", { name: "Edit", exact: true })).toHaveCount(0);
  const directory = tree.getByRole("link", { name: "Directory", exact: true });
  const history = tree.getByRole("link", { name: "History", exact: true });
  await expect(directory).toHaveAttribute("href", `${home}/index`);
  await expect(history).toHaveAttribute("href", `${home}/history`);
  expect(
    await tree.getByRole("link").evaluateAll((links) => links.slice(0, 2).map((link) => link.textContent))
  ).toEqual(["Directory", "History"]);
  await directory.click();
  await expect(reader, "EW02_ROUTE: the registered index route opens").toHaveURL(`${home}/index`);
  await expect(
    reader.getByRole("main").getByRole("heading", { level: 1, name: "Directory", exact: true })
  ).toBeVisible();
  await expect(
    reader
      .getByRole("region", { name: "Directory pages", exact: true })
      .getByRole("link", { name: "Guide", exact: true })
  ).toHaveAttribute("href", `${home}/pages/${guide.id}`);
  await history.click();
  await expect(reader).toHaveURL(`${home}/history`);
  await expect(reader.getByRole("main").getByRole("heading", { level: 1, name: "History", exact: true })).toBeVisible();
  await tree.getByRole("link", { name: "Guide", exact: true }).click();
  const article = reader.getByRole("article", { name: "Guide", exact: true });
  await article.getByRole("button", { name: "Open directory", exact: true }).click();
  await expect(reader, "EW02_READER: unresolved root index works for a reader").toHaveURL(`${home}/index`);
  await expect(
    reader.getByRole("main").getByRole("heading", { level: 1, name: "Directory", exact: true })
  ).toBeVisible();
  await tree.getByRole("link", { name: "Guide", exact: true }).click();
  await article.getByRole("button", { name: "Open history", exact: true }).click();
  await expect(reader).toHaveURL(`${home}/history`);
  await expect(reader.getByRole("main").getByRole("heading", { level: 1, name: "History", exact: true })).toBeVisible();
  expect(forbiddenCalls, "EW02_READER: virtual navigation never asks permission to create").toEqual([]);

  const writer = await signedInPage(team.tokens);
  await writer.goto(notebookPath(team.workspace.slug, wiki.id, "transfer"));
  await writer.getByRole("button", { name: "Export the whole notebook", exact: true }).click();
  const job = await exportWith(writer, wiki.id, "Export Browser virtual?");
  expect(job.client).toBe("web");
  const archive = await downloadFrom(writer, jobRow(writer, "Export of the whole notebook"));
  const files = new Map(archive.entries.map((entry) => [entry.name, entry.data.toString("utf8")]));
  expect(files.get("Browser virtual/Guide.md")).toBe(body);
  expect(files.get("Browser virtual/raw/Evidence.md")).toBe("Evidence bytes.\n");
  expect(files.get("Browser virtual/index.md")).toContain("](<Guide.md>)");
  const log = files.get("Browser virtual/log.md");
  if (log === undefined) throw new Error("EW02_ZIP: the actual downloaded archive has no log.md");
  const entry = entryOfVirtualLog(log, changeset);
  expect(entry[0]).toMatch(/; ingests: 1$/);
  expect(entry.filter((line) => line.startsWith("  - source: "))).toEqual([
    "  - source: [raw/Evidence.md](<raw/Evidence.md>)",
  ]);
  const meta = JSON.parse(files.get("Browser virtual/.nerve/meta.json") ?? "null") as { contributed: string[] };
  expect(meta.contributed).toEqual(["index.md", "log.md"]);
});
