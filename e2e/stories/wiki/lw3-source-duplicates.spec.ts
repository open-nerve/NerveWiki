import { expectUploaded, storedBlobs } from "../../fixtures/assert/asset";
import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNewPage, expectSourceHashes, pageStateOf } from "../../fixtures/assert/page";
import { postAsset, uploadAsset, utf8 } from "../../fixtures/assets";
import { createToken, emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode, postPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

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
