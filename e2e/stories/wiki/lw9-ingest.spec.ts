/* oxlint-disable no-await-in-loop -- The story follows real writes, a session's append order, and each scoped client. */
import { expectChangesetOf, expectIngestOf, expectOneSessionRevision } from "../../fixtures/assert/page";
import { bearer, emailFor } from "../../fixtures/auth";
import { changesetOfRevision, entryOfVirtualLog, getChangeset, listChangesets } from "../../fixtures/history";
import { joinAs } from "../../fixtures/invitations";
import { errorOf, mcpClient, postMcp, readPage, resultOf } from "../../fixtures/mcp";
import { createNotebook, updateNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode, endSession, openSession, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { archiveAt, endedJob, startExport } from "../../fixtures/transfer";
import { newTeam } from "../../fixtures/workspaces";

interface IngestRow {
  id: string;
  notebook_id: string;
  kind: string;
  client: string;
  ingest_count: number;
  ingest_sources: (string | null)[];
  ingest_sources_cut: boolean;
}
interface IngestLog {
  changesets: IngestRow[];
  next_cursor: string | null;
}
interface IngestDetail {
  changeset: IngestRow | null;
  next_cursor: string | null;
}
interface IngestBatch {
  changeset_id: string | null;
  changed: boolean;
  results_count: number;
  results: { node_id: string; changed: boolean }[];
}
interface PendingSources {
  mode: string;
  stale_pages: number;
  next_cursor: string | null;
  data: { rule: string; node_id: string; kind: string; path: string }[];
}

// S2 API/MCP stories: real producers and readers. S3 adds the browser history UI.
test("LW9 (API): one source becomes 25 in one edit session; history and log retain seq and missing positions", async ({
  api,
  db,
  nervewiki,
}, info) => {
  const team = await newTeam(api, info);
  const reader = await joinAs(api, team.pat, team.workspace.slug, emailFor(info, "reader"), "member");
  const wiki = await createNotebook(api, team.pat, team.workspace.slug, "Ingest history", "viewer", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  if (!wiki.source_root_id) throw new Error("the real template has no source root");
  const sources: { id: string; name: string }[] = [];
  for (let i = 0; i < 25; i++) {
    const name = `Source-${String(i).padStart(2, "0")}`;
    const page = await createPage(api, team.pat, wiki.id, name, wiki.source_root_id, `source ${i}\n`);
    sources.push({ id: page.id, name });
  }
  // UUIDs come from the real creates. Selecting the greatest first makes the
  // second Add's smaller UUIDs an independent counterexample to a global sort.
  const ascending = sources.toSorted((a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
  const first = ascending[24];
  if (!first) throw new Error("the 25-source fixture is incomplete");
  const ordered = [first, ...ascending.slice(0, 24)];
  const ids = ordered.map((s) => s.id);
  expect(ids).not.toEqual(ids.toSorted());
  const paths = ordered.map((s) => `raw/${s.name}.md`);
  expect(paths).not.toEqual(paths.toSorted());
  await createPage(api, team.pat, wiki.id, "Empty folder", wiki.source_root_id);
  const pendingIDs = async () => {
    const answer = await api.GET("/api/v0/notebooks/{notebook_id}/lint/{rule}", {
      params: { path: { notebook_id: wiki.id, rule: "undigested_sources" }, query: { limit: 100 } },
      headers: bearer(reader),
    });
    expect(answer.response.status).toBe(200);
    if (!answer.data) throw new Error("the real pending-source read has no data");
    expect(answer.data).toMatchObject({ mode: "llm_wiki", stale_pages: 0, next_cursor: null });
    return answer.data.data
      .map((item) => (item.rule === "undigested_sources" ? item.node_id : "unexpected rule"))
      .toSorted();
  };
  expect(await pendingIDs()).toEqual(ids.toSorted());
  const summary = await createPage(api, team.pat, wiki.id, "Summary", null, "before\n");
  const session = await openSession(api, team.pat, summary.id);
  const one = await writeContent(api, team.pat, summary.id, {
    content: `[[raw/${first.name}]]\n`,
    base_revision: summary.revision,
    edit_session_id: session.id,
  });
  const changeset = await changesetOfRevision(api, team.pat, summary.id, one.revision);
  await expectIngestOf(db, changeset, wiki.id, [first.id]);
  expect(await pendingIDs()).toEqual(ids.filter((id) => id !== first.id).toSorted());
  await expectChangesetOf(db, changeset, {
    client: "api",
    byId: team.adminId,
    items: [],
    versions: [summary.id],
    kind: "edit",
  });
  expect((await getChangeset(api, reader, changeset)).data).toMatchObject({
    id: changeset,
    kind: "edit",
    ingest_count: 1,
    ingest_sources: [first.id],
  });

  const all = await writeContent(api, team.pat, summary.id, {
    content:
      ordered
        .toReversed()
        .map((s) => `[[raw/${s.name}]]`)
        .join("\n") + "\n",
    base_revision: one.revision,
    edit_session_id: session.id,
  });
  expect(await changesetOfRevision(api, team.pat, summary.id, all.revision)).toBe(changeset);
  await expectIngestOf(db, changeset, wiki.id, ids);
  await expectOneSessionRevision(db, session.id, summary.id, summary.revision, all.revision);
  expect(await pendingIDs()).toEqual([]);
  // A later, distinct edit references an already consumed source and records zero.
  const again = await createPage(api, team.pat, wiki.id, "Again", null, "before again\n");
  const repeated = await writeContent(api, team.pat, again.id, {
    content: `[[raw/${first.name}]]\n`,
    base_revision: again.revision,
  });
  const repeatedID = await changesetOfRevision(api, team.pat, again.id, repeated.revision);
  expect(repeatedID).not.toBe(changeset);
  await expectIngestOf(db, repeatedID, wiki.id, []);
  expect((await getChangeset(api, reader, repeatedID)).data).toMatchObject({
    id: repeatedID,
    kind: "edit",
    ingest_count: 0,
    ingest_sources: [],
  });
  // Removing every reference does not erase this session's recorded ingestion.
  const removed = await writeContent(api, team.pat, summary.id, {
    content: "No references remain.\n",
    base_revision: all.revision,
    edit_session_id: session.id,
  });
  expect(await changesetOfRevision(api, team.pat, summary.id, removed.revision)).toBe(changeset);
  await expectIngestOf(db, changeset, wiki.id, ids);
  expect((await endSession(api, team.pat, session.id)).response.status).toBe(204);
  const third = ordered[2];
  if (!third) throw new Error("the third source is absent");
  expect((await deleteNode(api, team.pat, first.id)).response.status).toBe(204);
  expect((await deleteNode(api, team.pat, third.id)).response.status).toBe(204);
  expect((await readContent(api, team.pat, summary.id)).content).toBe("No references remain.\n");
  const previewIDs = ids.map((id, i) => (i === 0 || i === 2 ? null : id));
  const previewPaths = paths.map((path, i) => (i === 0 || i === 2 ? null : path));
  await expectIngestOf(db, changeset, wiki.id, ids);

  const filtered = await listChangesets(api, reader, wiki.id, { ingest: true });
  expect(filtered.response.status).toBe(200);
  expect(filtered.data?.data.map((row) => row.id)).toEqual([changeset]);
  expect(filtered.data?.next_cursor).toBeNull();
  expect(filtered.data?.data[0]).toMatchObject({
    id: changeset,
    kind: "edit",
    client: "api",
    ingest_count: 25,
    ingest_sources: previewIDs.slice(0, 3),
  });
  const detail = await getChangeset(api, reader, changeset);
  expect(detail.response.status).toBe(200);
  expect(detail.data).toMatchObject({
    id: changeset,
    kind: "edit",
    ingest_count: 25,
    ingest_sources: previewIDs.slice(0, 20),
  });
  for (const ingest of [undefined, false]) {
    const ordinary = await listChangesets(api, reader, wiki.id, { ingest, limit: 100 });
    expect(ordinary.response.status).toBe(200);
    expect(ordinary.data?.data.some((row) => row.id === changeset)).toBe(true);
    expect(ordinary.data?.data.some((row) => row.id !== changeset && row.ingest_count === 0)).toBe(true);
  }
  for (const scope of ["notebook", "account"] as const) {
    const agent = await mcpClient(nervewiki.baseURL, reader, scope === "notebook" ? { notebook: wiki.id } : {});
    const target = scope === "account" ? { notebook: wiki.id } : {};
    const log = resultOf<IngestLog>(await agent.call("log", { ...target, ingest: true }), `${scope} ingest log`);
    expect(log.changesets.map((row) => row.id)).toEqual([changeset]);
    expect(log.next_cursor).toBeNull();
    expect(log.changesets[0]).toMatchObject({
      id: changeset,
      kind: "edit",
      ingest_count: 25,
      ingest_sources: previewPaths.slice(0, 3),
      ingest_sources_cut: false,
    });
    const header = resultOf<IngestDetail>(
      await agent.call("changeset", { ...target, id: changeset }),
      `${scope} ingest detail`
    );
    expect(header.changeset).toMatchObject({
      id: changeset,
      kind: "edit",
      ingest_count: 25,
      ingest_sources: previewPaths.slice(0, 20),
      ingest_sources_cut: false,
    });
    const read = await readPage(agent, { ...target, path: "log.md" });
    expect([read.meta.virtual, read.meta.next_offset, read.meta.next_column]).toEqual([true, null, null]);
    const entry = entryOfVirtualLog(read.text, changeset);
    expect(entry[0]).toContain(" — edit; client: api;");
    expect(entry[0]).toMatch(/; ingests: 25$/);
    expect(entry.filter((line) => line.startsWith("  - source: "))).toEqual(
      previewPaths
        .slice(0, 5)
        .map((path) => (path === null ? "  - source: Missing source" : `  - source: [${path}](<${path}>)`))
    );
  }

  // The same persisted holes must survive the actual whole-export contributor.
  const exported = await endedJob(api, team.pat, (await startExport(api, team.pat, wiki.id)).id);
  expect(exported.state).toBe("succeeded");
  if (!exported.download) throw new Error("the marked notebook's whole export has no download");
  const archive = await archiveAt(nervewiki.baseURL, exported.download.url);
  const exportedLog = archive.entries.find((file) => file.name === "Ingest history/log.md")?.data.toString("utf8");
  if (exportedLog === undefined) throw new Error("the actual whole ZIP has no log.md");
  const exportedEntry = entryOfVirtualLog(exportedLog, changeset);
  expect(exportedEntry[0]).toMatch(/; ingests: 25$/);
  expect(exportedEntry.filter((line) => line.startsWith("  - source: "))).toEqual(
    previewPaths
      .slice(0, 5)
      .map((path) => (path === null ? "  - source: Missing source" : `  - source: [${path}](<${path}>)`))
  );

  // The same shape of real PUT in a standard notebook is a non-ingest edit.
  const standard = await createNotebook(api, team.pat, team.workspace.slug, "Ordinary ingestion", "viewer");
  const raw = await createPage(api, team.pat, standard.id, "raw");
  await createPage(api, team.pat, standard.id, "Source", raw.id, "ordinary source\n");
  const note = await createPage(api, team.pat, standard.id, "Summary", null, "before\n");
  const saved = await writeContent(api, team.pat, note.id, {
    content: "[[raw/Source]]\n",
    base_revision: note.revision,
  });
  const normal = await changesetOfRevision(api, team.pat, note.id, saved.revision);
  await expectIngestOf(db, normal, standard.id, []);
  expect((await getChangeset(api, reader, normal)).data).toMatchObject({
    id: normal,
    ingest_count: 0,
    ingest_sources: [],
  });
  const empty = await listChangesets(api, reader, standard.id, { ingest: true });
  expect([empty.response.status, empty.data?.data, empty.data?.next_cursor]).toEqual([200, [], null]);

  // Hide the actually marked notebook after the reader has successfully read it.
  expect((await updateNotebook(api, team.pat, wiki.id, { workspace_access: "none" })).response.status).toBe(200);
  const hiddenLog = await listChangesets(api, reader, wiki.id, { ingest: true });
  const hiddenDetail = await getChangeset(api, reader, changeset);
  expect([hiddenLog.response.status, hiddenLog.error?.code]).toEqual([404, "notebook.not_found"]);
  expect([hiddenDetail.response.status, hiddenDetail.error?.code]).toEqual([404, "page.changeset_not_found"]);
  const account = await mcpClient(nervewiki.baseURL, reader);
  expect(errorOf(await account.call("log", { notebook: wiki.id, ingest: true }), "hidden account log").code).toBe(
    "notebook.not_found"
  );
  const hidden = await postMcp(
    nervewiki.baseURL,
    reader,
    {
      jsonrpc: "2.0",
      id: 1,
      method: "tools/call",
      params: { name: "log", arguments: { ingest: true } },
    },
    wiki.id
  );
  expect(hidden.status).toBe(404);
  expect(await hidden.json()).toMatchObject({ code: "notebook.not_found" });
});

for (const scope of ["notebook", "account"] as const) {
  test(`LW9 (MCP ${scope}): a real batch records exactly one new source in one edit changeset`, async ({
    api,
    db,
    nervewiki,
  }, info) => {
    const team = await newTeam(api, info);
    const wiki = await createNotebook(api, team.pat, team.workspace.slug, `Ingest ${scope}`, "none", {
      mode: "llm_wiki",
      templateLocale: "en",
    });
    if (!wiki.source_root_id) throw new Error("the template has no source root");
    const source = await createPage(api, team.pat, wiki.id, "Paper", wiki.source_root_id, `paper for ${scope}\n`);
    await createPage(api, team.pat, wiki.id, "Empty folder", wiki.source_root_id);
    const summary = await createPage(api, team.pat, wiki.id, "Summary", null, "before\n");
    const note = await createPage(api, team.pat, wiki.id, "Note", null, "before note\n");
    const agent = await mcpClient(nervewiki.baseURL, team.pat, scope === "notebook" ? { notebook: wiki.id } : {});
    const target = scope === "account" ? { notebook: wiki.id } : {};
    const pendingBefore = resultOf<PendingSources>(
      await agent.call("pending_sources", target),
      `${scope} pending before`
    );
    expect(pendingBefore).toMatchObject({ mode: "llm_wiki", stale_pages: 0, next_cursor: null });
    expect(pendingBefore.data.map((item) => [item.rule, item.node_id, item.kind, item.path])).toEqual([
      ["undigested_sources", source.id, "page", "raw/Paper.md"],
    ]);
    const batch = resultOf<IngestBatch>(
      await agent.call("batch", {
        ...target,
        message: `Use one paper through ${scope}`,
        ops: [
          {
            op: "write",
            path: "Summary.md",
            base_version: `${summary.id}:${summary.revision}`,
            content: "[[raw/Paper]]\n[[raw/Paper]]\n",
          },
          {
            op: "write",
            path: "Note.md",
            base_version: `${note.id}:${note.revision}`,
            content: "The batch also keeps this ordinary note.\n",
          },
        ],
      }),
      `${scope} real producer`
    );
    expect([batch.changed, batch.results_count, batch.results.map((row) => [row.node_id, row.changed])]).toEqual([
      true,
      2,
      [
        [summary.id, true],
        [note.id, true],
      ],
    ]);
    if (!batch.changeset_id) throw new Error("the nonempty real batch has no changeset");
    const changeset = batch.changeset_id;
    await expectChangesetOf(db, changeset, {
      client: "mcp:e2e-agent",
      byId: team.adminId,
      items: [],
      versions: [summary.id, note.id],
      message: `Use one paper through ${scope}`,
      kind: "edit",
    });
    await expectIngestOf(db, changeset, wiki.id, [source.id]);
    const summaryNow = await readContent(api, team.pat, summary.id);
    expect(summaryNow.content).toBe("[[raw/Paper]]\n[[raw/Paper]]\n");
    const pendingAfter = resultOf<PendingSources>(
      await agent.call("pending_sources", target),
      `${scope} pending after`
    );
    expect(pendingAfter).toMatchObject({ mode: "llm_wiki", stale_pages: 0, next_cursor: null, data: [] });
    const repeat = resultOf<IngestBatch>(
      await agent.call("batch", {
        ...target,
        message: `Use the same paper again through ${scope}`,
        ops: [
          {
            op: "write",
            path: "Summary.md",
            base_version: `${summary.id}:${summaryNow.revision}`,
            content: "[[raw/Paper]]\n[[raw/Paper]]\nA second use.\n",
          },
        ],
      }),
      `${scope} reference the same source again`
    );
    expect([repeat.changed, repeat.results_count, repeat.results.map((row) => [row.node_id, row.changed])]).toEqual([
      true,
      1,
      [[summary.id, true]],
    ]);
    if (!repeat.changeset_id) throw new Error("the second nonempty batch has no changeset");
    expect(repeat.changeset_id).not.toBe(changeset);
    await expectIngestOf(db, repeat.changeset_id, wiki.id, []);
    expect((await getChangeset(api, team.pat, repeat.changeset_id)).data).toMatchObject({
      id: repeat.changeset_id,
      kind: "edit",
      ingest_count: 0,
      ingest_sources: [],
    });
    const listed = await listChangesets(api, team.pat, wiki.id, { ingest: true });
    expect(listed.response.status).toBe(200);
    expect(listed.data?.data.map((row) => row.id)).toEqual([changeset]);
    expect(listed.data?.data[0]).toMatchObject({
      id: changeset,
      kind: "edit",
      ingest_count: 1,
      ingest_sources: [source.id],
    });
    expect((await getChangeset(api, team.pat, changeset)).data).toMatchObject({
      id: changeset,
      kind: "edit",
      ingest_count: 1,
      ingest_sources: [source.id],
    });
    const log = resultOf<IngestLog>(await agent.call("log", { ...target, ingest: true }), `${scope} log`);
    expect(log.changesets.map((row) => row.id)).toEqual([changeset]);
    expect(log.next_cursor).toBeNull();
    expect(log.changesets[0]).toMatchObject({
      id: changeset,
      kind: "edit",
      ingest_count: 1,
      ingest_sources: ["raw/Paper.md"],
      ingest_sources_cut: false,
    });
    const detail = resultOf<IngestDetail>(
      await agent.call("changeset", { ...target, id: changeset }),
      `${scope} detail`
    );
    expect(detail.changeset).toMatchObject({
      id: changeset,
      kind: "edit",
      ingest_count: 1,
      ingest_sources: ["raw/Paper.md"],
      ingest_sources_cut: false,
    });
    const read = await readPage(agent, { ...target, path: "log.md" });
    expect([read.meta.virtual, read.meta.next_offset, read.meta.next_column]).toEqual([true, null, null]);
    const entry = entryOfVirtualLog(read.text, changeset);
    expect(entry[0]).toContain(" — edit; client: mcp:e2e-agent;");
    expect(entry[0]).toMatch(/; ingests: 1;/);
    expect(entry.filter((line) => line.startsWith("  - source: "))).toEqual([
      "  - source: [raw/Paper.md](<raw/Paper.md>)",
    ]);
  });
}
