import { storedBlobs } from "../../fixtures/assert/asset";
import { accountIdOf } from "../../fixtures/assert/identity";
import { expectSourceHashes } from "../../fixtures/assert/page";
import { expectImported, sha256Of, treeOf } from "../../fixtures/assert/transfer";
import { uploadAsset, utf8 } from "../../fixtures/assets";
import { emailFor } from "../../fixtures/auth";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { endedJob, listJobs, startImport } from "../../fixtures/transfer";
import { newTeam } from "../../fixtures/workspaces";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { importWith, jobRow, openReport } from "../../fixtures/wiki-transfer";
import { zipOf } from "../../fixtures/zip-write";

// Both entry points import the same real archive and check the complete persisted result. MCP has no import tool;
// Go additionally uses single-node batches to prove duplicate parents keep their descendants across batches.
for (const entryPoint of ["API", "page"] as const) {
  test(`LW7 (${entryPoint}): duplicate source leaves are skipped, duplicate parents become empty folders, and unique descendants and attachments continue`, async ({
    api,
    db,
    nervewiki,
    signedInPage,
  }, testInfo) => {
    const { pat, workspace } = await newTeam(api, testInfo);
    const writerEmail = emailFor(testInfo, "writer");
    const writerTokens =
      entryPoint === "page" ? await joinOnboarded(api, pat, workspace.slug, writerEmail, "member") : undefined;
    const writer = writerTokens?.access_token ?? (await joinAs(api, pat, workspace.slug, writerEmail, "member"));
    const page = writerTokens === undefined ? undefined : await signedInPage(writerTokens);
    const writerId = await accountIdOf(db, writerEmail);
    const notebook = await createNotebook(api, pat, workspace.slug, "Imported sources", "editor", {
      mode: "llm_wiki",
      templateLocale: "en",
    });
    if (!notebook.source_root_id) throw new Error("no source root");
    const raw = notebook.source_root_id;
    const duplicate = "already committed source\n";
    const known = await createPage(api, writer, notebook.id, "Known", raw, duplicate);
    const knownAsset = await uploadAsset(
      api,
      writer,
      notebook.id,
      { name: "known.bin", bytes: new Uint8Array(0) },
      raw
    );
    const before = await treeOf(db, nervewiki.storageDir, notebook.id);
    const files = storedBlobs(nervewiki.storageDir);
    const archive = zipOf([
      { name: "Parent.md", data: duplicate },
      { name: "Parent/Child.md", data: "unique child\n" },
      { name: "Leaf.md", data: duplicate },
      { name: "emptyA.md", data: "" },
      { name: "emptyB.md", data: "" },
      { name: "copy.bin", data: "" },
      { name: "Fresh.md", data: "fresh source\n" },
      { name: "Next.md", data: "fresh source\n" },
      { name: "A.bin", data: "asset bytes" },
      { name: "B.bin", data: "asset bytes" },
    ]);
    if (page !== undefined) await page.goto(notebookPath(workspace.slug, notebook.id, "transfer"));
    const queued =
      page === undefined
        ? await startImport(api, writer, notebook.id, archive, raw)
        : await importWith(page, notebook.id, archive, { name: "sources.zip", place: "raw" });
    const job = await endedJob(api, writer, queued.id);
    expect([job.state, job.progress]).toEqual(["succeeded", { done: 10, total: 10 }]);
    expect(job.report).toEqual({
      failure: null,
      counts: {
        pages: 4,
        attachments: 1,
        renamed: 0,
        skipped: 5,
        missing: 0,
        restored: 0,
        deleted: 0,
        put_back: 0,
        written: 0,
      },
      placed: null,
      lock: null,
      refusal: null,
    });
    expect(job.problems).toHaveLength(5);
    expect(job.problems.every((problem) => problem.code === "duplicate_source")).toBe(true);
    expect(Object.fromEntries(job.problems.map((problem) => [problem.path, problem.to]))).toEqual({
      "Parent.md": "raw/Known.md",
      "Leaf.md": "raw/Known.md",
      "copy.bin": "raw/known.bin",
      "Next.md": "raw/Fresh.md",
      "B.bin": "raw/A.bin",
    });
    const after = await treeOf(db, nervewiki.storageDir, notebook.id);
    const existing = new Set(before.map((entry) => entry.path));
    expect(after.filter((entry) => existing.has(entry.path))).toEqual(before);
    expect(
      after
        .filter((entry) => !existing.has(entry.path))
        .toSorted((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0))
    ).toEqual([
      { path: "raw/A.bin", kind: "asset", mime: "application/octet-stream", sha256: sha256Of(utf8("asset bytes")) },
      { path: "raw/Fresh", kind: "page", content: "fresh source\n" },
      { path: "raw/Parent", kind: "page", content: "" },
      { path: "raw/Parent/Child", kind: "page", content: "unique child\n" },
      { path: "raw/emptyA", kind: "page", content: "" },
      { path: "raw/emptyB", kind: "page", content: "" },
    ]);
    // Ten processed entries include five skipped bodies and one retained empty parent: six persisted nodes, five
    // actual page revisions. Progress is not an assertion that ten changeset items were written.
    await expectImported(db, nervewiki.storageDir, job.id, {
      creatorId: writerId,
      client: entryPoint === "API" ? "api" : "web",
      root: raw,
      state: "succeeded",
      failure: null,
      done: 10,
      total: 10,
      written: 6,
      counts: { pages: 4, attachments: 1, renamed: 0, skipped: 5 },
    });
    expect(
      await db.query(
        `SELECT count(r.id)::int AS versions, bool_and(NOT r.rewrite) AS ordinary
       FROM transfer_jobs j JOIN page_revisions r ON r.changeset_id = j.changeset_id WHERE j.id = $1`,
        [job.id]
      )
    ).toEqual([{ versions: 5, ordinary: true }]);
    expect(storedBlobs(nervewiki.storageDir).filter((file) => !files.includes(file))).toHaveLength(1);
    const [persisted] = await db.query<{ report: unknown }>("SELECT report FROM transfer_jobs WHERE id = $1", [job.id]);
    expect(persisted?.report).toMatchObject({ counts: job.report?.counts, problems: job.problems });
    expect((await listJobs(api, writer, notebook.id)).find((item) => item.id === job.id)?.report).toEqual(job.report);
    await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
    if (page !== undefined) {
      const row = jobRow(page, "Import of sources.zip");
      await expect(row.getByText("Done", { exact: true })).toBeVisible();
      const report = await openReport(row);
      await expect(report.terms).toHaveText(["Pages", "Attachments", "Renamed", "Skipped"]);
      await expect(report.counts).toHaveText(["4", "1", "0", "5"]);
      await expect(report.problems).toHaveCount(5);
      const pageTargets = row.getByRole("link", { name: "raw/Known.md", exact: true });
      await expect(pageTargets).toHaveCount(2);
      await pageTargets.first().click();
      await expect(page).toHaveURL(
        new URL(wikiPagePath(workspace.slug, notebook.id, known.id), nervewiki.baseURL).href
      );
      await expect(pageHeading(page, "Known")).toBeFocused();
      await expect(page.getByRole("article", { name: "Known", exact: true })).toContainText(duplicate.trimEnd());

      await page.goto(notebookPath(workspace.slug, notebook.id, "transfer"));
      await openReport(jobRow(page, "Import of sources.zip"));
      await jobRow(page, "Import of sources.zip").getByRole("link", { name: "raw/known.bin", exact: true }).click();
      await expect(page).toHaveURL(new URL(wikiPagePath(workspace.slug, notebook.id, raw), nervewiki.baseURL).href);
      await expect(page.locator(`[data-attachment="${knownAsset.id}"]`)).toBeFocused();
      await expect(page.locator(`[data-attachment="${knownAsset.id}"]`)).toContainText("known.bin");
    }
  });
}
