import { expectIndexedLinks } from "../../fixtures/assert/links";
import { failedToLoad } from "../../fixtures/browser";
import { expectImported, idsOf, sha256Of, storedImports, treeOf } from "../../fixtures/assert/transfer";
import { pngBytes, utf8 } from "../../fixtures/assets";
import { notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { endedJob, holdImport, startImport } from "../../fixtures/transfer";
import { importWith, jobRow, openReport } from "../../fixtures/wiki-transfer";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";
import { zipOf } from "../../fixtures/zip-write";

// TR2, an Obsidian vault imported (M7 design 9; M7/P6 design 3.18): a job
// that runs in the background, the vault's folder, its settings and
// macOS's files left out. A page is its .md, a folder its children, a
// folder without its .md a page without content; an attachment is a file
// under the page of its folder; a name that is no title is mended, and one
// that clashes after it numbered, the name as the vault had it kept; the
// report says so. The links resolve as in the vault, and those of the
// pages there before reach the pages imported. At the notebook's root,
// after its pages, and under a page.

const pdfBytes = utf8("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n");

/** A vault as Obsidian keeps it, zipped with its folder as macOS's Finder zips it. */
function vault(): Buffer {
  return zipOf([
    { name: "Vault/" },
    { name: "Vault/.obsidian/" },
    { name: "Vault/.obsidian/app.json", data: "{}" },
    { name: "Vault/Home.md", data: "# Home\n\n[[Alpha]], [[Projects/Beta|Beta]], ![[diagram.png]], [[What_ Why]]\n" },
    { name: "Vault/Projects/" },
    { name: "Vault/Projects/Alpha.md", data: "Back to [[Home]].\n" },
    { name: "Vault/Projects/Alpha/" },
    { name: "Vault/Projects/Alpha/spec.pdf", data: pdfBytes },
    { name: "Vault/Projects/Beta.md", data: "![[spec.pdf]]\n" },
    { name: "Vault/diagram.png", data: pngBytes, method: 0 },
    { name: "Vault/What? Why.md", data: "mended\n" },
    { name: "Vault/What_ Why.md", data: "named so\n" },
    { name: "__MACOSX/Vault/._Home.md", data: "\x00\x05\x16\x07" },
  ]);
}

/** The vault's nodes as a notebook holds them under prefix, in their order. */
function imported(prefix: string) {
  return [
    { path: `${prefix}diagram.png`, kind: "asset", mime: "image/png", sha256: sha256Of(pngBytes) },
    {
      path: `${prefix}Home`,
      kind: "page",
      content: "# Home\n\n[[Alpha]], [[Projects/Beta|Beta]], ![[diagram.png]], [[What_ Why]]\n",
    },
    { path: `${prefix}Projects`, kind: "page", content: "" },
    { path: `${prefix}Projects/Alpha`, kind: "page", content: "Back to [[Home]].\n" },
    { path: `${prefix}Projects/Alpha/spec.pdf`, kind: "asset", mime: "application/pdf", sha256: sha256Of(pdfBytes) },
    { path: `${prefix}Projects/Beta`, kind: "page", content: "![[spec.pdf]]\n" },
    { path: `${prefix}What_ Why`, kind: "page", content: "named so\n" },
    { path: `${prefix}What_ Why 2`, kind: "page", content: "mended\n" },
  ];
}

test("TR2 (API): a vault imports at a notebook's root after its pages and under a page: its pages, folders and attachments, names mended and numbered, links resolved as in the vault", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, adminId, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Notes");
  const index = await createPage(api, pat, notebook.id, "Index", null, "[[Alpha]]\n");
  await expectIndexedLinks(db, index.id, [{ kind: "wikilink", property: null, target: "Alpha", resolved: null }]);

  const queued = await startImport(api, pat, notebook.id, vault());
  expect([queued.kind, queued.state, queued.name, queued.root_id, queued.client]).toEqual([
    "import",
    "queued",
    "vault.zip",
    null,
    "api",
  ]);
  const job = await endedJob(api, pat, queued.id);
  expect([job.state, job.progress]).toEqual(["succeeded", { done: 8, total: 8 }]);
  expect(job.report).toEqual({
    failure: null,
    counts: { pages: 6, attachments: 2, renamed: 1, missing: 0, skipped: 0 },
  });
  expect(job.problems).toEqual([{ path: "What? Why.md", code: "renamed", to: "What_ Why 2" }]);
  expect(await treeOf(db, nervewiki.storageDir, notebook.id)).toEqual([
    { path: "Index", kind: "page", content: "[[Alpha]]\n" },
    ...imported(""),
  ]);
  await expectImported(db, nervewiki.storageDir, job.id, {
    creatorId: adminId,
    client: "api",
    root: null,
    state: "succeeded",
    failure: null,
    done: 8,
    total: 8,
    counts: { pages: 6, attachments: 2, renamed: 1, skipped: 0 },
  });
  // The links resolve as in the vault: the name it had kept by its page;
  // the page there before reaches the page imported.
  const ids = await idsOf(db, notebook.id);
  await expectIndexedLinks(db, index.id, [
    { kind: "wikilink", property: null, target: "Alpha", resolved: ids.get("Projects/Alpha") ?? "" },
  ]);
  await expectIndexedLinks(db, ids.get("Home") ?? "", [
    { kind: "wikilink", property: null, target: "Alpha", resolved: ids.get("Projects/Alpha") ?? "" },
    { kind: "wikilink", property: null, target: "Projects/Beta", resolved: ids.get("Projects/Beta") ?? "" },
    { kind: "embed", property: null, target: "diagram.png", resolved: ids.get("diagram.png") ?? "" },
    { kind: "wikilink", property: null, target: "What_ Why", resolved: ids.get("What_ Why") ?? "" },
  ]);
  await expectIndexedLinks(db, ids.get("Projects/Alpha") ?? "", [
    { kind: "wikilink", property: null, target: "Home", resolved: ids.get("Home") ?? "" },
  ]);
  await expectIndexedLinks(db, ids.get("Projects/Beta") ?? "", [
    { kind: "embed", property: null, target: "spec.pdf", resolved: ids.get("Projects/Alpha/spec.pdf") ?? "" },
  ]);

  // Under a page of another notebook: the report's paths from the page.
  const archive = await createNotebook(api, pat, workspace.slug, "Archive");
  const imports = await createPage(api, pat, archive.id, "Imports");
  const under = await endedJob(api, pat, (await startImport(api, pat, archive.id, vault(), imports.id)).id);
  expect([under.state, under.root_id, under.problems]).toEqual([
    "succeeded",
    imports.id,
    [{ path: "What? Why.md", code: "renamed", to: "What_ Why 2" }],
  ]);
  expect(await treeOf(db, nervewiki.storageDir, archive.id)).toEqual([
    { path: "Imports", kind: "page", content: "" },
    ...imported("Imports/"),
  ]);
  await expectImported(db, nervewiki.storageDir, under.id, {
    creatorId: adminId,
    client: "api",
    root: imports.id,
    state: "succeeded",
    failure: null,
    done: 8,
    total: 8,
    counts: { pages: 6, attachments: 2, renamed: 1, skipped: 0 },
  });
  const underIds = await idsOf(db, archive.id);
  await expectIndexedLinks(db, underIds.get("Imports/Projects/Alpha") ?? "", [
    { kind: "wikilink", property: null, target: "Home", resolved: underIds.get("Imports/Home") ?? "" },
  ]);
  expect(storedImports(nervewiki.storageDir)).toEqual([]);
});

test("TR2 (page): a vault imports from the notebook's settings, at its root and under a page, its row focused, its report read; an import of the notebook under way refuses another", async ({
  api,
  db,
  nervewiki,
  signedInPage,
  pageWatch,
}, testInfo) => {
  const { tokens, adminId, pat, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  const notebook = await createNotebook(api, pat, workspace.slug, "Notes");
  await createPage(api, pat, notebook.id, "Index", null, "[[Alpha]]\n");

  await page.goto(notebookPath(workspace.slug, notebook.id, "transfer"));
  await expect(page.getByText("No jobs yet.")).toBeVisible();
  const job = await importWith(page, notebook.id, vault());
  const row = jobRow(page, "Import of vault.zip");
  await expect(row).toBeFocused();
  await expect(row.getByText("Done", { exact: true })).toBeVisible({ timeout: 15_000 });
  const report = await openReport(row);
  await expect(report.terms).toHaveText(["Pages", "Attachments", "Renamed", "Skipped"]);
  await expect(report.counts).toHaveText(["6", "2", "1", "0"]);
  await expect(report.problems).toHaveText([
    "What? Why.md was imported as What_ Why 2: links to its old name do not reach it.",
  ]);
  expect(await treeOf(db, nervewiki.storageDir, notebook.id)).toEqual([
    { path: "Index", kind: "page", content: "[[Alpha]]\n" },
    ...imported(""),
  ]);
  await expectImported(db, nervewiki.storageDir, job.id, {
    creatorId: adminId,
    client: "web",
    root: null,
    state: "succeeded",
    failure: null,
    done: 8,
    total: 8,
    counts: { pages: 6, attachments: 2, renamed: 1, skipped: 0 },
  });

  // Under a page, chosen by its path.
  const archive = await createNotebook(api, pat, workspace.slug, "Archive");
  const imports = await createPage(api, pat, archive.id, "Imports");
  await page.goto(notebookPath(workspace.slug, archive.id, "transfer"));
  const under = await importWith(page, archive.id, vault(), { place: "Imports" });
  expect(under.root_id).toBe(imports.id);
  await expect(jobRow(page, "Import of vault.zip").getByText("Done", { exact: true })).toBeVisible({ timeout: 15_000 });
  expect(await treeOf(db, nervewiki.storageDir, archive.id)).toEqual([
    { path: "Imports", kind: "page", content: "" },
    ...imported("Imports/"),
  ]);

  // Another's upload into the notebook under way: the server refuses before it reads the file.
  // Two uploads of the API's: whichever reaches the server second is refused at once, the other holding the notebook.
  const uploads = [holdImport(nervewiki.baseURL, pat, archive.id), holdImport(nervewiki.baseURL, pat, archive.id)];
  const refused = await Promise.race(uploads.map(async (upload, at) => ({ status: await upload.answered, at })));
  expect(refused.status).toBe(409);
  uploads[refused.at]?.stop();
  const held = uploads[1 - refused.at] as ReturnType<typeof holdImport>;
  await page.getByRole("button", { name: "Import a zip…" }).click();
  const dialog = page.getByRole("dialog", { name: "Import into Archive" });
  await dialog
    .getByLabel("Zip archive")
    .setInputFiles({ name: "again.zip", mimeType: "application/zip", buffer: vault() });
  await dialog.getByRole("button", { name: "Import", exact: true }).click();
  // Chromium reads the refusal the server answers before the file, though the body still goes.
  await expect(dialog.getByRole("alert")).toHaveText(
    "An import into this notebook is under way, perhaps someone else's. Wait for it to end, then try again."
  );
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  held.stop();
  expect(await held.answered).toBe(0);
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("list", { name: "Recent jobs" }).getByRole("listitem")).toHaveCount(1);
  expect(storedImports(nervewiki.storageDir)).toEqual([]);
});
