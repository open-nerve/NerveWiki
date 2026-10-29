import type { Asset, Page as WikiPage } from "@nervewiki/api-client";

import { expectUploaded, storedBlobs } from "../../fixtures/assert/asset";
import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNewPage, expectSourceHashes, pageStateOf } from "../../fixtures/assert/page";
import { pngBytes, postAsset, uploadAsset, utf8 } from "../../fixtures/assets";
import { createToken, emailFor } from "../../fixtures/auth";
import { answerTo, countAnswers, failedToLoad } from "../../fixtures/browser";
import { holdStream } from "../../fixtures/events";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode, moveNode, postPage, readContent } from "../../fixtures/pages";
import { expect, test, watchOf } from "../../fixtures/test";
import { choosePageAction, pageHeading, pageTree, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// LW3's API and both MCP entries use real writers. UUID provenance for old nodes entering, restoring, receiving
// their first content and being rewritten is additionally exercised by the Go full-chain tests (P2 section 5).
test("LW3 (API): exact bytes deduplicate only live sources of the same kind and notebook; empty pages remain distinct", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const writerEmail = emailFor(testInfo, "writer");
  const writer = await joinAs(api, pat, workspace.slug, writerEmail, "member");
  const writerId = await accountIdOf(db, writerEmail);
  const notebook = await createNotebook(api, pat, workspace.slug, "Sources", "editor", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  const other = await createNotebook(api, pat, workspace.slug, "Other sources", "editor", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  if (!notebook.source_root_id || !other.source_root_id) throw new Error("no configured source roots");
  const raw = notebook.source_root_id;
  const left = await createPage(api, writer, notebook.id, "Left", raw);
  const right = await createPage(api, writer, notebook.id, "Right", raw, "");
  const bytes = "精确的来源字节\r\nsecond line\n";
  const known = await createPage(api, writer, notebook.id, "Known", left.id, bytes);
  await expectNewPage(db, known, writerId, "api", bytes);
  // oxlint-disable no-await-in-loop -- each refusal must finish before its state and the next path are checked
  for (const parent_id of [raw, right.id]) {
    const before = await pageStateOf(db, notebook.id);
    const duplicate = await postPage(api, writer, notebook.id, { parent_id, title: "Different title", content: bytes });
    expect([duplicate.response.status, duplicate.error?.code, duplicate.error?.duplicate_of]).toEqual([
      409,
      "page.source_duplicate",
      known.id,
    ]);
    expect(await pageStateOf(db, notebook.id)).toEqual(before);
  }
  // oxlint-enable no-await-in-loop
  // A line ending change is a byte change; no textual normalization is part of this contract.
  const changed = await createPage(api, writer, notebook.id, "LF", raw, bytes.replaceAll("\r\n", "\n"));
  await expectNewPage(db, changed, writerId, "api", bytes.replaceAll("\r\n", "\n"));
  const outside = await createPage(api, writer, notebook.id, "Outside", null, bytes);
  await expectNewPage(db, outside, writerId, "api", bytes);
  const elsewhere = await createPage(api, writer, other.id, "Same in another book", other.source_root_id, bytes);
  await expectNewPage(db, elsewhere, writerId, "api", bytes);
  const asset = await uploadAsset(api, writer, notebook.id, { name: "same.txt", bytes: utf8(bytes) }, raw);
  await expectUploaded(db, nervewiki.storageDir, asset, utf8(bytes), writerId, "api");
  expect((await deleteNode(api, pat, known.id)).response.status).toBe(204);
  const replacement = await createPage(api, writer, notebook.id, "After deletion", raw, bytes);
  await expectNewPage(db, replacement, writerId, "api", bytes);
  expect(replacement.id).not.toBe(known.id);
  await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
  await expectSourceHashes(db, nervewiki.storageDir, other.id);
});

test("LW3 (API): zero-byte attachments deduplicate across titles and paths, and a rejected upload leaves no file", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const writerEmail = emailFor(testInfo, "writer");
  const writer = await joinAs(api, pat, workspace.slug, writerEmail, "member");
  const writerId = await accountIdOf(db, writerEmail);
  const notebook = await createNotebook(api, pat, workspace.slug, "Sources", "editor", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  const other = await createNotebook(api, pat, workspace.slug, "Other", "editor", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  if (!notebook.source_root_id || !other.source_root_id) throw new Error("no source roots");
  const folder = await createPage(api, writer, notebook.id, "Folder", notebook.source_root_id);
  await createPage(api, writer, notebook.id, "Another empty page", notebook.source_root_id);
  const empty = { name: "empty.bin", bytes: new Uint8Array(0) };
  const known = await uploadAsset(api, writer, notebook.id, empty, notebook.source_root_id);
  await expectUploaded(db, nervewiki.storageDir, known, empty.bytes, writerId, "api");
  // oxlint-disable no-await-in-loop -- each upload refusal must finish before its row and file rollback are checked
  for (const parent of [notebook.source_root_id, folder.id]) {
    const before = await pageStateOf(db, notebook.id);
    const files = storedBlobs(nervewiki.storageDir);
    const duplicate = await postAsset(api, writer, notebook.id, { ...empty, name: "renamed.bin" }, parent);
    expect([duplicate.response.status, duplicate.error?.code, duplicate.error?.duplicate_of]).toEqual([
      409,
      "page.source_duplicate",
      known.id,
    ]);
    expect(await pageStateOf(db, notebook.id)).toEqual(before);
    expect(storedBlobs(nervewiki.storageDir)).toEqual(files);
  }
  // oxlint-enable no-await-in-loop
  // oxlint-disable no-await-in-loop -- each successful upload is checked before the next fixture mutation
  for (const [book, parent] of [
    [notebook.id, undefined],
    [other.id, other.source_root_id],
  ] as const) {
    const allowed = await uploadAsset(api, writer, book, empty, parent);
    await expectUploaded(db, nervewiki.storageDir, allowed, empty.bytes, writerId, "api");
  }
  // oxlint-enable no-await-in-loop
  expect((await deleteNode(api, pat, known.id)).response.status).toBe(204);
  const replacement = await uploadAsset(api, writer, notebook.id, empty, folder.id);
  await expectUploaded(db, nervewiki.storageDir, replacement, empty.bytes, writerId, "api");
  await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
  await expectSourceHashes(db, nervewiki.storageDir, other.id);
});

for (const scope of ["notebook", "account"] as const) {
  test(`LW3 (${scope} MCP): committed duplicates identify the source; two new identical sources roll back the whole batch without an unusable UUID`, async ({
    api,
    db,
    nervewiki,
  }, testInfo) => {
    const { pat, workspace } = await newTeam(api, testInfo);
    const writerEmail = emailFor(testInfo, "writer");
    const writer = await joinAs(api, pat, workspace.slug, writerEmail, "member");
    const writerId = await accountIdOf(db, writerEmail);
    const notebook = await createNotebook(api, pat, workspace.slug, "Sources", "editor", {
      mode: "llm_wiki",
      templateLocale: "en",
    });
    if (!notebook.source_root_id) throw new Error("no source root");
    const token = await createToken(api, writer, { name: `lw3-${scope}` });
    const clientName = `lw3-${scope}-writer`;
    const client = await mcpClient(nervewiki.baseURL, token.token, {
      ...(scope === "notebook" ? { notebook: notebook.id } : {}),
      client: clientName,
    });
    const args = (body: Record<string, unknown>) => (scope === "account" ? { ...body, notebook: notebook.id } : body);
    const known = resultOf<{ node_id: string }>(
      await client.call("write", args({ path: "raw/Known.md", content: "committed bytes\n" })),
      "write the committed source"
    );
    const [stored] = await db.query<{
      id: string;
      notebook_id: string;
      parent_id: string;
      name: string;
      created_at: string;
    }>("SELECT id, notebook_id, parent_id, name, created_at::text FROM nodes WHERE id = $1", [known.node_id]);
    if (!stored) throw new Error("the MCP source was not persisted");
    await expectNewPage(db, stored, writerId, `mcp:${clientName}`, "committed bytes\n");
    await createPage(api, writer, notebook.id, "Folder", notebook.source_root_id);
    // oxlint-disable no-await-in-loop -- each MCP refusal is followed by its own unchanged-state check
    for (const path of ["raw/Other.md", "raw/Folder/Other.md"]) {
      const before = await pageStateOf(db, notebook.id);
      const error = errorOf(await client.call("write", args({ path, content: "committed bytes\n" })), path);
      expect([error.code, error.duplicate_of]).toEqual(["page.source_duplicate", known.node_id]);
      expect(await pageStateOf(db, notebook.id)).toEqual(before);
    }
    // oxlint-enable no-await-in-loop
    // oxlint-disable no-await-in-loop -- each batch must fully roll back before the next batch reuses its paths
    for (const committed of [false, true]) {
      const before = await pageStateOf(db, notebook.id);
      const error = errorOf(
        await client.call(
          "batch",
          args({
            message: "Verify atomic source admission",
            ops: [
              { op: "write", path: "Ordinary.md", content: "also rolls back\n" },
              { op: "write", path: "raw/First.md", content: committed ? "unique first\n" : "new duplicate\n" },
              { op: "write", path: "raw/Second.md", content: committed ? "committed bytes\n" : "new duplicate\n" },
            ],
          })
        ),
        "atomic duplicate batch"
      );
      expect([error.code, error.op], JSON.stringify(error)).toEqual(["page.source_duplicate", 2]);
      if (committed) expect(error.duplicate_of).toBe(known.node_id);
      else expect(Object.hasOwn(error, "duplicate_of")).toBe(false);
      expect(await pageStateOf(db, notebook.id)).toEqual(before);
    }
    // oxlint-enable no-await-in-loop
    // oxlint-disable no-await-in-loop -- create each empty source in the same notebook in a defined order
    for (const path of ["raw/Empty A.md", "raw/Empty B.md"]) {
      resultOf(await client.call("write", args({ path, content: "" })), "empty pages are independent");
    }
    // oxlint-enable no-await-in-loop
    await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
  });
}

for (const change of ["created", "moved in"] as const) {
  test(`LW3 (page): a duplicate source ${change} after the tree was read is refreshed once and its complete path opens the real page`, async ({
    api,
    db,
    nervewiki,
    signedInPage,
  }, testInfo) => {
    const { pat, workspace } = await newOnboardedTeam(api, testInfo);
    const tokens = await joinOnboarded(api, pat, workspace.slug, emailFor(testInfo, "writer"), "member");
    const notebook = await createNotebook(api, pat, workspace.slug, "Sources", "editor", {
      mode: "llm_wiki",
      templateLocale: "en",
    });
    if (!notebook.source_root_id) throw new Error("no configured source root");
    const archive = await createPage(api, pat, notebook.id, "Archive", notebook.source_root_id);
    const inbox = await createPage(api, pat, notebook.id, "Inbox", notebook.source_root_id);
    const outside =
      change === "moved in"
        ? await createPage(api, pat, notebook.id, "Known", null, "The committed source.\n")
        : undefined;
    const page = await signedInPage(tokens);
    const releaseEvents = await holdStream(page);
    const nodesPath = `/api/v0/notebooks/${notebook.id}/nodes`;
    const createsPath = `/api/v0/notebooks/${notebook.id}/pages`;
    const nodeAnswers = countAnswers(page, "GET", nodesPath);
    try {
      await page.goto(wikiPagePath(workspace.slug, notebook.id, inbox.id));
      await expect(pageHeading(page, "Inbox")).toBeVisible();
      await choosePageAction(page, notebook.name, "Inbox", "Paste Markdown");
      const dialog = page.getByRole("dialog", { name: "Paste Markdown under Inbox", exact: true });
      await dialog.getByRole("textbox", { name: "Title", exact: true }).fill("A new title");
      await dialog.getByRole("textbox", { name: "Markdown", exact: true }).fill("The committed source.\n");
      const initialReads = nodeAnswers();
      expect(initialReads).toBeGreaterThan(0);

      // The browser has read its tree, but receives none of this real API change's events yet.
      const known =
        outside ?? (await createPage(api, pat, notebook.id, "Known", archive.id, "The committed source.\n"));
      if (outside !== undefined) await moveNode(api, pat, known.id, { parent_id: archive.id });
      const before = await pageStateOf(db, notebook.id);
      let refused = false;
      const reads: string[] = [];
      page.on("response", (response) => {
        if (response.request().method() === "POST" && new URL(response.url()).pathname === createsPath) {
          refused = response.status() === 409;
        }
      });
      await page.route(`**${nodesPath}`, async (route) => {
        if (route.request().method() !== "GET" || !refused) {
          await route.continue();
          return;
        }
        // The create's ordinary re-read fails in transport, leaving the genuine old tree. The duplicate helper
        // must make one further real request. No successful API response or node list is fabricated here.
        reads.push(reads.length === 0 ? "failed ordinary refresh" : "real target refresh");
        if (reads.length === 1) await route.abort("failed");
        else await route.continue();
      });
      watchOf(page).expectConsole({ errors: [failedToLoad(409), "Failed to load resource: net::ERR_FAILED"] });
      const rejected = answerTo(page, "POST", createsPath);
      await dialog.getByRole("button", { name: "Add source", exact: true }).click();
      const response = await rejected;
      expect(response.status()).toBe(409);
      expect(await response.json()).toMatchObject({ code: "page.source_duplicate", duplicate_of: known.id });
      expect(await pageStateOf(db, notebook.id)).toEqual(before);

      const target = dialog.getByRole("link", { name: "raw/Archive/Known.md", exact: true });
      await expect(target).toBeVisible();
      expect(reads).toEqual(["failed ordinary refresh", "real target refresh"]);
      expect(nodeAnswers()).toBe(initialReads + 1);
      await expect(dialog).not.toContainText(known.id);
      await target.click();
      await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, known.id));
      await expect(pageHeading(page, "Known")).toBeVisible();
      await expect(page.getByRole("article", { name: "Known", exact: true })).toContainText("The committed source.");
      expect(reads).toEqual(["failed ordinary refresh", "real target refresh"]);
      await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
    } finally {
      releaseEvents();
    }
  });
}

test("LW3 (page): paste creates the initial body once, and mixed uppercase Markdown preserves its bytes after closing the upload dialog and navigating away", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { pat, workspace } = await newOnboardedTeam(api, testInfo);
  const writerEmail = emailFor(testInfo, "writer");
  const tokens = await joinOnboarded(api, pat, workspace.slug, writerEmail, "member");
  const writerId = await accountIdOf(db, writerEmail);
  const notebook = await createNotebook(api, pat, workspace.slug, "Sources", "editor", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  if (!notebook.source_root_id) throw new Error("no configured source root");
  const inbox = await createPage(api, pat, notebook.id, "Inbox", notebook.source_root_id);
  const later = await createPage(api, pat, notebook.id, "Later", null, "Keep reading here.\n");
  const page = await signedInPage(tokens);
  const createsPath = `/api/v0/notebooks/${notebook.id}/pages`;
  const uploadsPath = `/api/v0/notebooks/${notebook.id}/assets`;
  const creates = countAnswers(page, "POST", createsPath);
  const uploads = countAnswers(page, "POST", uploadsPath);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, inbox.id));
  await expect(pageHeading(page, "Inbox")).toBeVisible();
  await choosePageAction(page, notebook.name, "Inbox", "Paste Markdown");
  const paste = page.getByRole("dialog", { name: "Paste Markdown under Inbox", exact: true });
  const pastedBody = "# Pasted source\n\nThe initial body, with a final newline.\n";
  await paste.getByRole("textbox", { name: "Title", exact: true }).fill("Pasted");
  await paste.getByRole("textbox", { name: "Markdown", exact: true }).fill(pastedBody);
  const pasteAnswer = answerTo(page, "POST", createsPath);
  await paste.getByRole("button", { name: "Add source", exact: true }).click();
  const pastedResponse = await pasteAnswer;
  expect(pastedResponse.status()).toBe(201);
  const pasted = (await pastedResponse.json()) as WikiPage;
  expect([pasted.name, pasted.parent_id]).toEqual(["Pasted", inbox.id]);
  await expectNewPage(db, pasted, writerId, "web", pastedBody);
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, pasted.id));
  await expect(pageHeading(page, "Pasted")).toBeVisible();
  expect(creates()).toBe(1);
  expect(watchOf(page).apiRequests.filter((line) => line.startsWith("PUT "))).toEqual([]);

  // These bytes are hand-written, independently of the application's TextDecoder. They include a BOM,
  // a multibyte character, CRLF, a standalone CR, LF and the last newline, all of which must reach storage.
  const markdownBytes = Buffer.from([
    0xef, 0xbb, 0xbf, 0x23, 0x20, 0xe4, 0xb8, 0xad, 0x0d, 0x0a, 0x41, 0x0d, 0x42, 0x0a, 0x43, 0x0a,
  ]);
  const markdownBody = "\uFEFF# 中\r\nA\rB\nC\n";
  await pageTree(page, notebook.name).locator(`a[href$="/pages/${inbox.id}"]`).click();
  await expect(pageHeading(page, "Inbox")).toBeVisible();
  await choosePageAction(page, notebook.name, "Inbox", "Upload files");
  const dialog = page.getByRole("dialog", { name: "Upload sources under Inbox", exact: true });
  let releaseCreate!: () => void;
  const heldCreate = new Promise<void>((resolve) => (releaseCreate = resolve));
  let observedCreate!: () => void;
  const createStarted = new Promise<void>((resolve) => (observedCreate = resolve));
  await page.route(`**${createsPath}`, async (route) => {
    if (route.request().method() === "POST") {
      observedCreate();
      await heldCreate;
    }
    await route.continue();
  });
  try {
    const pageAnswer = answerTo(page, "POST", createsPath);
    const assetAnswer = answerTo(page, "POST", uploadsPath);
    const choosing = page.waitForEvent("filechooser");
    await dialog.getByRole("button", { name: "Choose files", exact: true }).click();
    const chooser = await choosing;
    expect(chooser.isMultiple()).toBe(true);
    await chooser.setFiles([
      { name: "exact.MD", mimeType: "text/markdown", buffer: markdownBytes },
      { name: "scan.png", mimeType: "image/png", buffer: Buffer.from(pngBytes) },
    ]);
    await createStarted;
    await expect(dialog.getByRole("list", { name: "Uploads", exact: true })).toContainText("Creating…");
    // The attachment completes through the real server while the Markdown request is held in transport.
    const assetResponse = await assetAnswer;
    expect(assetResponse.status()).toBe(201);
    const asset = (await assetResponse.json()) as Asset;
    expect([asset.name, asset.parent_id]).toEqual(["scan.png", inbox.id]);
    await expectUploaded(db, nervewiki.storageDir, asset, pngBytes, writerId, "web");
    expect(creates()).toBe(1);
    await dialog.getByRole("button", { name: "Close", exact: true }).click();
    await expect(dialog).toBeHidden();
    await pageTree(page, notebook.name).locator(`a[href$="/pages/${later.id}"]`).click();
    await expect(pageHeading(page, "Later")).toBeVisible();
    releaseCreate();
    const sourceResponse = await pageAnswer;
    expect(sourceResponse.status()).toBe(201);
    const source = (await sourceResponse.json()) as WikiPage;
    expect([source.name, source.parent_id]).toEqual(["exact", inbox.id]);
    await expectNewPage(db, source, writerId, "web", markdownBody);
    const stored = await readContent(api, pat, source.id);
    expect(stored.content).toBe(markdownBody);
    expect(Buffer.from(stored.content, "utf8")).toEqual(markdownBytes);
    await expect(
      pageTree(page, notebook.name).getByRole("button", { name: "Actions for exact", exact: true })
    ).toBeVisible();
    await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, later.id));
    await expect(pageHeading(page, "Later")).toBeVisible();
    expect([creates(), uploads()]).toEqual([2, 1]);
    expect(watchOf(page).apiRequests.filter((line) => line.startsWith("PUT "))).toEqual([]);
    await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
  } finally {
    releaseCreate();
  }
});

test("LW3 (page): a duplicate attachment dropped onto a source opens its complete path and focuses the real attachment", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { pat, adminId, workspace } = await newOnboardedTeam(api, testInfo);
  const tokens = await joinOnboarded(api, pat, workspace.slug, emailFor(testInfo, "writer"), "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Sources", "editor", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  if (!notebook.source_root_id) throw new Error("no configured source root");
  const archive = await createPage(api, pat, notebook.id, "Archive", notebook.source_root_id);
  const inbox = await createPage(api, pat, notebook.id, "Inbox", notebook.source_root_id, "Drop new sources here.\n");
  const known = await uploadAsset(api, pat, notebook.id, { name: "known.png", bytes: pngBytes }, archive.id);
  await expectUploaded(db, nervewiki.storageDir, known, pngBytes, adminId, "api");
  const before = await pageStateOf(db, notebook.id);
  const files = storedBlobs(nervewiki.storageDir);
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, inbox.id));
  await expect(pageHeading(page, "Inbox")).toBeVisible();
  watchOf(page).expectConsole({ errors: [failedToLoad(409)] });
  const uploadsPath = `/api/v0/notebooks/${notebook.id}/assets`;
  const uploads = countAnswers(page, "POST", uploadsPath);
  const rejected = answerTo(page, "POST", uploadsPath);
  const dropped = await page.getByRole("article", { name: "Inbox", exact: true }).evaluate(
    (article, bytes) => {
      const data = new DataTransfer();
      data.items.add(new File([new Uint8Array(bytes)], "different-name.png", { type: "image/png" }));
      const over = new DragEvent("dragover", { bubbles: true, cancelable: true, dataTransfer: data });
      article.dispatchEvent(over);
      const drop = new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: data });
      article.dispatchEvent(drop);
      return { over: over.defaultPrevented, drop: drop.defaultPrevented };
    },
    [...pngBytes]
  );
  expect(dropped).toEqual({ over: true, drop: true });
  const response = await rejected;
  expect(response.status()).toBe(409);
  expect(await response.json()).toMatchObject({ code: "page.source_duplicate", duplicate_of: known.id });
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  expect(storedBlobs(nervewiki.storageDir)).toEqual(files);
  const queue = page
    .getByRole("region", { name: "Attachments", exact: true })
    .getByRole("list", { name: "Uploads", exact: true });
  const target = queue.getByRole("link", { name: "raw/Archive/known.png", exact: true });
  await expect(target).toBeVisible();
  await expect(queue).not.toContainText(known.id);
  expect(uploads()).toBe(1);
  await target.click();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, archive.id));
  await expect(pageHeading(page, "Archive")).toBeVisible();
  const found = page
    .getByRole("region", { name: "Attachments", exact: true })
    .locator(`[data-attachment="${known.id}"]`);
  await expect(found).toHaveAccessibleName("known.png (source) (opens in a new tab)");
  await expect(found).toBeFocused();
  expect(uploads()).toBe(1);
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  expect(storedBlobs(nervewiki.storageDir)).toEqual(files);
  await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
});
