import type { TransferJob } from "@nervewiki/api-client";

import { storedBlobs } from "../../fixtures/assert/asset";
import { expectSourceHashes, pageStateOf } from "../../fixtures/assert/page";
import { uploadAsset, utf8 } from "../../fixtures/assets";
import { bearer, emailFor } from "../../fixtures/auth";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import { notebookPath } from "../../fixtures/notebook-pages";
import { revertChangeset } from "../../fixtures/history";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { endedJob, listJobs, runningJob, startImport, startTrashRestore } from "../../fixtures/transfer";
import { zipOf } from "../../fixtures/zip-write";
import { restoreNode, restoreWith as restoreTrashWith, trashPath, trashStatus } from "../../fixtures/trash";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";
import {
  changesetPath,
  moveDialog,
  movePageWith,
  pageHeading,
  restoreWith as restoreVersionWith,
  revertDialog,
  revertWith,
  versionPath,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { jobRow, openReport } from "../../fixtures/wiki-transfer";

// LW6 uses the API: MCP has no restore or background-history tools. Real worker pre-plan, post-plan and post-first-
// commit interleavings are also covered by TestSourceBackgroundRefusalsNameTheirActualLayer in bootstrap, with production guards intact.
for (const kind of ["page", "asset"] as const) {
  test(`LW6 (API): duplicate ${kind} restoration and undo roll back the entire subtree; both background starts refuse without a job`, async ({
    api,
    db,
    nervewiki,
  }, testInfo) => {
    const { pat, workspace } = await newTeam(api, testInfo);
    const writer = await joinAs(api, pat, workspace.slug, emailFor(testInfo, "writer"), "member");
    const notebook = await createNotebook(api, pat, workspace.slug, "History sources", "editor", {
      mode: "llm_wiki",
      templateLocale: "en",
    });
    if (!notebook.source_root_id) throw new Error("no source root");
    const raw = notebook.source_root_id;
    const folder = await createPage(api, writer, notebook.id, "Archived", raw);
    await createPage(api, writer, notebook.id, "Unique child", folder.id, "unique retained child\n");
    const body = "archived duplicate bytes\n";
    const original =
      kind === "page"
        ? await createPage(api, writer, notebook.id, "Original", folder.id, body)
        : await uploadAsset(api, writer, notebook.id, { name: "original.txt", bytes: utf8(body) }, folder.id);
    expect((await deleteNode(api, pat, folder.id)).response.status).toBe(204);
    const [deletion] = await db.query<{ id: string }>(
      `SELECT s.id FROM changesets s JOIN changeset_items i ON i.changeset_id = s.id
        WHERE i.node_id = $1 AND i.before_name IS NOT NULL AND i.after_name IS NULL
        ORDER BY s.created_at DESC, s.id DESC LIMIT 1`,
      [folder.id]
    );
    if (!deletion) throw new Error("no actual deletion changeset");
    const known =
      kind === "page"
        ? await createPage(api, writer, notebook.id, "Known", raw, body)
        : await uploadAsset(api, writer, notebook.id, { name: "known.txt", bytes: utf8(body) }, raw);
    const queued = () =>
      db.query(
        `SELECT id::text, args FROM river_job WHERE kind IN ('transfer.trash_restore', 'transfer.revert') ORDER BY id`
      );
    // oxlint-disable no-await-in-loop -- check each refusal and its unchanged rows, jobs and files before the next operation
    for (const operation of ["restore", "undo", "start restore", "start undo"] as const) {
      const before = await pageStateOf(db, notebook.id);
      const jobs = await db.query("SELECT to_jsonb(j) AS row FROM transfer_jobs j WHERE notebook_id = $1 ORDER BY id", [
        notebook.id,
      ]);
      const queue = await queued();
      const files = storedBlobs(nervewiki.storageDir);
      const answer =
        operation === "restore"
          ? await restoreNode(api, pat, folder.id)
          : operation === "undo"
            ? await revertChangeset(api, pat, deletion.id)
            : operation === "start restore"
              ? await api.POST("/api/v0/notebooks/{notebook_id}/trash-restores", {
                  params: { path: { notebook_id: notebook.id } },
                  headers: bearer(pat),
                  body: { node_id: folder.id },
                })
              : await api.POST("/api/v0/notebooks/{notebook_id}/reverts", {
                  params: { path: { notebook_id: notebook.id } },
                  headers: bearer(pat),
                  body: { changeset_id: deletion.id },
                });
      expect([answer.response.status, answer.error?.code, answer.error?.duplicate_of], operation).toEqual([
        409,
        "page.source_duplicate",
        known.id,
      ]);
      expect(await pageStateOf(db, notebook.id), operation).toEqual(before);
      expect(
        await db.query("SELECT to_jsonb(j) AS row FROM transfer_jobs j WHERE notebook_id = $1 ORDER BY id", [
          notebook.id,
        ])
      ).toEqual(jobs);
      expect(await queued()).toEqual(queue);
      expect(storedBlobs(nervewiki.storageDir)).toEqual(files);
      expect(
        await db.query("SELECT deleted_at IS NOT NULL AS deleted FROM nodes WHERE id = $1", [original.id])
      ).toEqual([{ deleted: true }]);
    }
    // oxlint-enable no-await-in-loop
    // Resolving the conflict makes the same public restore possible; the original parent, bytes and asset file
    // remain available after all four refusals. No administrator exemption permits the duplicate itself.
    expect((await deleteNode(api, pat, known.id)).response.status).toBe(204);
    expect((await restoreNode(api, pat, folder.id)).response.status).toBe(200);
    expect(
      await db.query("SELECT parent_id, deleted_at IS NULL AS live FROM nodes WHERE id = $1", [original.id])
    ).toEqual([{ parent_id: folder.id, live: true }]);
    await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
  });
}

// A row lock holds the first real batch after it has taken the notebook's tree lock. A real content request then
// queues for notebook SHARE before the worker can ask for its second batch. PostgreSQL's observed lock queue,
// rather than a sleep or a server test hook, places that write between the two committed units.
for (const entry of ["API", "page"] as const) {
  test(`LW6 (${entry}): a duplicate committed between restore batches refuses the current batch and preserves the earlier batch and its report`, async ({
    api,
    db,
    nervewiki,
    signedInPage,
    pageWatch,
  }, testInfo) => {
    const browserTeam = entry === "page" ? await newOnboardedTeam(api, testInfo) : undefined;
    const { pat, workspace } = browserTeam ?? (await newTeam(api, testInfo));
    const page = browserTeam === undefined ? undefined : await signedInPage(browserTeam.tokens);
    const writer = await joinAs(api, pat, workspace.slug, emailFor(testInfo, "writer"), "member");
    const notebook = await createNotebook(api, pat, workspace.slug, "Batched sources", "editor", {
      mode: "llm_wiki",
      templateLocale: "en",
    });
    if (!notebook.source_root_id) throw new Error("no source root");
    const raw = notebook.source_root_id;
    const folder = await createPage(api, writer, notebook.id, "Archive", raw);
    const known = await createPage(api, writer, notebook.id, "Late competitor", raw);
    const archive = zipOf(
      Array.from({ length: 200 }, (_, i) => ({ name: `${String(i).padStart(3, "0")}.md`, data: `source ${i}\n` }))
    );
    const imported = await startImport(api, writer, notebook.id, archive, folder.id);
    expect((await endedJob(api, writer, imported.id)).state).toBe("succeeded");
    const [target] = await db.query<{ id: string }>("SELECT id FROM nodes WHERE parent_id = $1 AND name = '150'", [
      folder.id,
    ]);
    if (!target) throw new Error("missing later-batch source");
    expect((await deleteNode(api, pat, folder.id)).response.status).toBe(204);
    const pending = await db.query<{ id: string; row: unknown }>(
      "SELECT id, to_jsonb(n) - 'trash_root' - 'trash_pages' - 'trash_assets' - 'trash_height' AS row FROM nodes n WHERE (id = $1 OR parent_id = $1) AND deleted_at IS NOT NULL ORDER BY id",
      [folder.id]
    );
    expect(pending).toHaveLength(201);
    const historyOf = (ids: string[]) =>
      db.query<{ node_id: string; kind: string; id: string; row: unknown }>(
        `WITH stored AS (
       SELECT c.node_id, 'content' AS kind, c.node_id AS id, to_jsonb(c) AS row
         FROM page_contents c WHERE c.node_id = ANY($1::uuid[])
       UNION ALL
       SELECT r.node_id, 'revision', r.id, to_jsonb(r) FROM page_revisions r WHERE r.node_id = ANY($1::uuid[])
       UNION ALL
       SELECT i.node_id, 'item', i.id, to_jsonb(i) FROM changeset_items i WHERE i.node_id = ANY($1::uuid[])
     ) SELECT * FROM stored ORDER BY node_id, kind, id`,
        [ids]
      );
    const pendingHistory = await historyOf(pending.map((node) => node.id));
    expect(pendingHistory.length).toBeGreaterThanOrEqual(201 * 3);
    if (page !== undefined) {
      pageWatch.expectConsole({ errors: [failedToLoad(422)] });
      await page.goto(trashPath(workspace.slug, notebook.id, folder.id));
      expect((await restoreTrashWith(page, folder.id, "Archive", "From: raw")).status()).toBe(422);
      await expect(page.getByRole("main").getByRole("button", { name: /^Restore .* in the background/ })).toBeVisible();
    }
    const release = await db.hold("SELECT 1 FROM nodes WHERE id = $1 FOR NO KEY UPDATE /* lw6-first-batch */", [
      folder.id,
    ]);
    let writing: ReturnType<typeof writeContent> | undefined;
    let jobId: string | undefined;
    try {
      const [holder] = await db.query<{ pid: number }>(
        "SELECT pid FROM pg_stat_activity WHERE datname = current_database() AND state = 'idle in transaction' AND query LIKE '%lw6-first-batch%'"
      );
      if (!holder) throw new Error("missing owned first-batch row holder");
      let queued: TransferJob;
      if (page === undefined) queued = await startTrashRestore(api, pat, notebook.id, folder.id);
      else {
        const answer = answerTo(page, "POST", `/api/v0/notebooks/${notebook.id}/trash-restores`);
        await page
          .getByRole("main")
          .getByRole("button", { name: /^Restore .* in the background/ })
          .click();
        const response = await answer;
        expect(response.status()).toBe(202);
        queued = (await response.json()) as TransferJob;
      }
      jobId = queued.id;
      await runningJob(api, pat, queued.id);
      let workerPid = 0;
      await expect
        .poll(
          async () => {
            const rows = await db.query<{ pid: number }>(
              `SELECT pid FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'
          AND $1::int = ANY(pg_blocking_pids(pid)) AND query LIKE '%UPDATE nodes%'`,
              [holder.pid]
            );
            workerPid = rows[0]?.pid ?? 0;
            return rows.length;
          },
          { message: "the actual first restore batch waits for its node after taking the notebook lock" }
        )
        .toBe(1);
      writing = writeContent(api, writer, known.id, { content: "source 150\n", base_revision: 1 });
      // Install a handler immediately so cleanup still awaits a request that fails before the waiter assertion.
      void writing.catch(() => undefined);
      await expect
        .poll(
          async () => {
            const rows = await db.query<{ pid: number }>(
              `SELECT pid FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'
          AND $1::int = ANY(pg_blocking_pids(pid)) AND query LIKE '%FOR SHARE%'`,
              [workerPid]
            );
            return rows.length;
          },
          { message: "the real first content write queues behind the first batch" }
        )
        .toBe(1);
      expect(
        await db.query(
          "SELECT count(*)::int AS live FROM nodes WHERE (id = $1 OR parent_id = $1) AND deleted_at IS NULL",
          [folder.id]
        )
      ).toEqual([{ live: 0 }]);
      await release();
      const written = await writing;
      expect(written.revision).toBe(2);
      const job = await endedJob(api, pat, queued.id);
      expect([job.state, job.progress]).toEqual(["failed", { done: 100, total: 201 }]);
      expect(job.report?.failure).toBe("refused");
      expect(job.report?.refusal).toMatchObject({
        code: "page.source_duplicate",
        node_id: target.id,
        duplicate_of: known.id,
      });
      expect(job.report?.counts.pages).toBe(100);
      const live = await db.query<{ id: string }>(
        "SELECT id FROM nodes WHERE (id = $1 OR parent_id = $1) AND deleted_at IS NULL ORDER BY id",
        [folder.id]
      );
      expect(live).toHaveLength(100);
      const committed = new Set(live.map((node) => node.id));
      expect(committed.has(folder.id)).toBe(true);
      expect(committed.has(target.id)).toBe(false);
      expect(
        await db.query(
          "SELECT id, to_jsonb(n) - 'trash_root' - 'trash_pages' - 'trash_assets' - 'trash_height' AS row FROM nodes n WHERE (id = $1 OR parent_id = $1) AND deleted_at IS NOT NULL ORDER BY id",
          [folder.id]
        )
      ).toEqual(pending.filter((node) => !committed.has(node.id)));
      // Marking the remaining children as independent trash items is the first batch's intended effect. Their
      // content, revisions and old changeset items must still be byte-for-byte unchanged after the refused batch.
      expect(await historyOf(pending.filter((node) => !committed.has(node.id)).map((node) => node.id))).toEqual(
        pendingHistory.filter((row) => !committed.has(row.node_id))
      );
      expect(
        await db.query(
          `SELECT count(*)::int AS roots FROM nodes WHERE parent_id = $1 AND deleted_at IS NOT NULL
        AND trash_root AND trash_pages = 1 AND trash_assets = 0 AND trash_height = 1`,
          [folder.id]
        )
      ).toEqual([{ roots: 101 }]);
      expect(
        await db.query(
          `SELECT count(i.id)::int AS items FROM transfer_jobs j JOIN changeset_items i ON i.changeset_id = j.changeset_id WHERE j.id = $1`,
          [job.id]
        )
      ).toEqual([{ items: 100 }]);
      const [stored] = await db.query<{ report: { failure: string; refusal: unknown } }>(
        "SELECT report FROM transfer_jobs WHERE id = $1",
        [job.id]
      );
      expect(stored?.report.failure).toBe("refused");
      expect(stored?.report.refusal).toEqual(job.report?.refusal);
      expect((await listJobs(api, pat, notebook.id)).find((item) => item.id === job.id)?.report).toEqual(job.report);
      await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
      expect(job.client).toBe(page === undefined ? "api" : "web");
      if (page !== undefined) {
        await page.goto(notebookPath(workspace.slug, notebook.id, "transfer"));
        const row = jobRow(page, "Restore of Archive from the trash");
        await openReport(row);
        await expect(row).toContainText(/earlier completed batches are kept/i);
        await expect(row).toContainText(/this batch was not written/i);
        // The refused node remains in the trash; its unknown current location must not become a live-page link.
        await expect(row.getByRole("link", { name: target.id, exact: true })).toHaveCount(0);
        await row.getByRole("link", { name: "raw/Late competitor.md", exact: true }).click();
        await expect(page).toHaveURL(
          new URL(wikiPagePath(workspace.slug, notebook.id, known.id), nervewiki.baseURL).href
        );
        await expect(pageHeading(page, "Late competitor")).toBeFocused();
      }
    } finally {
      await release();
      if (writing) await writing.catch(() => undefined);
      if (jobId) await endedJob(api, pat, jobId);
    }
  });
}

for (const kind of ["page", "asset"] as const) {
  test(`LW6 (page): trash list, preview and undo name the duplicate ${kind}, link to it, and preserve the refused subtree`, async ({
    api,
    db,
    nervewiki,
    signedInPage,
    pageWatch,
  }, testInfo) => {
    const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
    const notebook = await createNotebook(api, pat, workspace.slug, "Source history", "editor", { mode: "llm_wiki" });
    const raw = notebook.source_root_id;
    if (!raw) throw new Error("no source root");
    const folder = await createPage(api, pat, notebook.id, "Archived", raw);
    await createPage(api, pat, notebook.id, "Unique child", folder.id, "unique restored child\n");
    const body = "identical archived source\n";
    const original =
      kind === "page"
        ? await createPage(api, pat, notebook.id, "Original", folder.id, body)
        : await uploadAsset(api, pat, notebook.id, { name: "original.txt", bytes: utf8(body) }, folder.id);
    expect((await deleteNode(api, pat, folder.id)).response.status).toBe(204);
    const [deletion] = await db.query<{ id: string }>(
      `SELECT s.id FROM changesets s JOIN changeset_items i ON i.changeset_id = s.id
       WHERE i.node_id = $1 AND i.before_name IS NOT NULL AND i.after_name IS NULL
       ORDER BY s.created_at DESC, s.id DESC LIMIT 1`,
      [folder.id]
    );
    if (!deletion) throw new Error("missing real deletion changeset");
    const known =
      kind === "page"
        ? await createPage(api, pat, notebook.id, "Known", raw, body)
        : await uploadAsset(api, pat, notebook.id, { name: "known.txt", bytes: utf8(body) }, raw);
    const page = await signedInPage(tokens);
    pageWatch.expectConsole({ errors: [failedToLoad(409), failedToLoad(409), failedToLoad(409)] });
    const before = await pageStateOf(db, notebook.id);
    const files = storedBlobs(nervewiki.storageDir);
    const path = kind === "page" ? "raw/Known.md" : "raw/known.txt";
    // Each real UI request must be refused atomically before opening the current visible target.
    // oxlint-disable no-await-in-loop -- each refusal must be checked before navigating to the next entry point
    for (const surface of ["list", "preview", "undo"] as const) {
      await page.goto(
        surface === "undo"
          ? changesetPath(workspace.slug, notebook.id, deletion.id)
          : trashPath(workspace.slug, notebook.id, surface === "preview" ? folder.id : undefined)
      );
      const response =
        surface === "undo"
          ? await revertWith(page, deletion.id)
          : await restoreTrashWith(page, folder.id, "Archived", "From: raw");
      expect(response.status()).toBe(409);
      expect(await response.json()).toMatchObject({ code: "page.source_duplicate", duplicate_of: known.id });
      expect(await pageStateOf(db, notebook.id)).toEqual(before);
      expect(storedBlobs(nervewiki.storageDir)).toEqual(files);
      const alert =
        surface === "undo" ? revertDialog(page).getByRole("alert") : page.getByRole("main").getByRole("alert");
      await expect(alert).toContainText("This operation was not written.");
      await alert.getByRole("link", { name: path, exact: true }).click();
      await expect(page).toHaveURL(
        new URL(wikiPagePath(workspace.slug, notebook.id, kind === "page" ? known.id : raw), nervewiki.baseURL).href
      );
      if (kind === "page") await expect(pageHeading(page, "Known")).toBeFocused();
      else await expect(page.locator(`[data-attachment="${known.id}"]`)).toBeFocused();
    }
    // oxlint-enable no-await-in-loop
    // Retry from the list after resolving the conflict; all three refusal surfaces were checked above.
    await page.goto(trashPath(workspace.slug, notebook.id));
    expect((await deleteNode(api, pat, known.id)).response.status).toBe(204);
    expect((await restoreTrashWith(page, folder.id, "Archived", "From: raw")).status()).toBe(200);
    await expect(page.getByRole("main").getByRole("heading", { level: 1, name: "Trash", exact: true })).toBeFocused();
    await expect(trashStatus(page)).toHaveText("Restored “Archived” under “raw”. Open");
    expect(
      await db.query("SELECT parent_id, deleted_at IS NULL AS live FROM nodes WHERE id = $1", [original.id])
    ).toEqual([{ parent_id: folder.id, live: true }]);
    await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
  });
}

test("LW6 (page): even an admin restores source content outside the area before moving it back, and resolves duplicates before retrying", async ({
  api,
  db,
  nervewiki,
  signedInPage,
  pageWatch,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Recover source content", "editor", {
    mode: "llm_wiki",
  });
  const raw = notebook.source_root_id;
  if (!raw) throw new Error("no source root");
  const outside = await createPage(api, pat, notebook.id, "Working");
  const source = await createPage(api, pat, notebook.id, "Draft", outside.id, "first source bytes\n");
  await writeContent(api, pat, source.id, { content: "second source bytes\n", base_revision: 1 });
  const [revision] = await db.query<{ changeset_id: string }>(
    "SELECT changeset_id FROM page_revisions WHERE node_id = $1 AND revision = 2",
    [source.id]
  );
  if (!revision) throw new Error("missing original write changeset");
  const known = await createPage(api, pat, notebook.id, "Known first", raw, "first source bytes\n");
  const page = await signedInPage(tokens);
  pageWatch.expectConsole({ errors: [failedToLoad(409), failedToLoad(409)] });
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  expect((await movePageWith(page, notebook.name, source.id, "Draft", "raw", "Last")).status()).toBe(200);
  await page.goto(versionPath(workspace.slug, notebook.id, source.id, 1));
  await expect(page.getByRole("main").getByRole("button", { name: "Restore this version", exact: true })).toHaveCount(
    0
  );
  await page.goto(changesetPath(workspace.slug, notebook.id, revision.changeset_id));
  const before = await pageStateOf(db, notebook.id);
  const refused = await revertWith(page, revision.changeset_id);
  expect(refused.status()).toBe(409);
  expect(await refused.json()).toMatchObject({ code: "page.source_immutable" });
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  await expect(revertDialog(page).getByRole("alert")).toContainText("outside the source area");
  await expect(revertDialog(page).getByRole("alert")).toContainText("Administrators");

  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  expect((await movePageWith(page, notebook.name, source.id, "Draft", "Working", "Last")).status()).toBe(200);
  await page.goto(versionPath(workspace.slug, notebook.id, source.id, 1));
  expect((await restoreVersionWith(page, source.id, 1)).status()).toBe(200);
  expect((await readContent(api, pat, source.id)).content).toBe("first source bytes\n");
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  const restored = await pageStateOf(db, notebook.id);
  const moved = await movePageWith(page, notebook.name, source.id, "Draft", "raw", "Last");
  expect(moved.status()).toBe(409);
  expect(await moved.json()).toMatchObject({ code: "page.source_duplicate", duplicate_of: known.id });
  expect(await pageStateOf(db, notebook.id)).toEqual(restored);
  await expect(moveDialog(page, "Draft").getByRole("link", { name: "raw/Known first.md", exact: true })).toBeVisible();
  expect((await deleteNode(api, pat, known.id)).response.status).toBe(204);
  expect((await movePageWith(page, notebook.name, source.id, "Draft", "raw", "Last")).status()).toBe(200);
  expect(
    await db.query("SELECT parent_id, source_hash IS NOT NULL AS hashed FROM nodes WHERE id = $1", [source.id])
  ).toEqual([{ parent_id: raw, hashed: true }]);
  expect((await readContent(api, pat, source.id)).content).toBe("first source bytes\n");
  await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
});
