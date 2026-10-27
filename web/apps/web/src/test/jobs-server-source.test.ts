import { expect, test } from "vitest";

import { AssetService } from "../services/asset.service";
import { PageService } from "../services/page.service";
import { TransferService } from "../services/transfer.service";
import { notebookJSON } from "./fakes";
import { moved, wrote, renamed } from "./history-server";
import { jobsServer } from "./jobs-server";
import { assetNode, pageNode } from "./page-server";

const root = pageNode(50, "raw");
const source = pageNode(51, "source", root);

function fixture(role: "admin" | "editor" = "editor") {
  const options = { role, mode: "llm_wiki" as const, sourceRootId: root.id, nodes: [root, source] };
  const server = jobsServer(options);
  return { server, transfer: new TransferService(server.app.session.public) };
}

test("background source refusal happens after admission and domain failures but before queue-full, without a job", async () => {
  const { server, transfer } = fixture("admin");
  let writes = 0;
  const change = server.log("source edit", {
    changes: [wrote(source, 1, 2)],
    undo: () => {
      writes += 1;
    },
  });
  const start = () => transfer.startRevert(notebookJSON.id, change.id);
  server.busy = true;
  await expect(start()).rejects.toMatchObject({ code: "transfer.busy" });
  server.busy = false;
  change.purged = true;
  await expect(start()).rejects.toMatchObject({ code: "page.changeset_trimmed" });
  change.purged = false;
  server.queueFull = true;
  await expect(start()).rejects.toMatchObject({ code: "page.source_immutable", status: 409 });
  expect(server.jobs).toEqual([]);
  expect(server.changesets).toEqual([change]);
  expect(writes).toBe(0);
});

test("background source management refuses writer and admits admin", async () => {
  const { server, transfer } = fixture();
  const change = server.log("source rename", { changes: [renamed(source, "before")] });
  await expect(transfer.startRevert(notebookJSON.id, change.id)).rejects.toMatchObject({
    code: "page.source_admin_only",
    status: 403,
  });
  expect(server.jobs).toEqual([]);
  const admin = fixture("admin");
  const managed = admin.server.log("source rename", { changes: [renamed(source, "before")] });
  await expect(admin.transfer.startRevert(notebookJSON.id, managed.id)).resolves.toMatchObject({
    state: "queued",
    kind: "revert",
  });
  expect(admin.server.jobs).toHaveLength(1);
});

// A source-entry refusal must retain 422 at the background route, after its
// existing admission/domain checks but before locks and queue capacity.
test("background source entry rejects 10001 nodes before locks and queue-full without a job", async () => {
  const raw = pageNode(90, "raw");
  const folder = pageNode(91, "Large");
  const children = Array.from({ length: 10000 }, (_, i) => ({
    ...assetNode(92, `File ${i}.txt`, folder),
    id: `0199a2b4-0000-7000-8000-${(60000 + i).toString(16).padStart(12, "0")}`,
  }));
  const server = jobsServer({
    role: "admin",
    mode: "llm_wiki",
    sourceRootId: raw.id,
    nodes: [raw, folder, ...children],
  });
  let writes = 0;
  const change = server.log("large source moved out", {
    changes: [moved(folder, raw.id)],
    lockedBy: { page: folder.id, holder: { user_id: raw.id, display_name: "Editor" } },
    undo: () => {
      writes += 1;
    },
  });
  const start = () => new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
  server.busy = true;
  await expect(start()).rejects.toMatchObject({ code: "transfer.busy" });
  server.busy = false;
  change.trimmed = true;
  await expect(start()).rejects.toMatchObject({ code: "page.changeset_trimmed" });
  change.trimmed = false;
  server.queueFull = true;
  await expect(start()).rejects.toMatchObject({ code: "page.source_too_large", status: 422 });
  expect(server.jobs).toEqual([]);
  expect(server.changesets).toEqual([change]);
  expect(change.revertedBy).toEqual([]);
  expect(server.nodes).toEqual([raw, folder, ...children]);
  expect(writes).toBe(0);
});

for (const entry of ["sync", "background"] as const) {
  test(`${entry} restore rejects source duplicates and retains trash, data and jobs`, async () => {
    const returning = pageNode(831, "returning", root);
    const server = jobsServer({
      role: "admin",
      mode: "llm_wiki",
      sourceRootId: root.id,
      nodes: [root, source, returning],
    });
    const page = new PageService(server.app.session.public);
    server.contents.set(source.id, { content: "same", revision: 1 });
    server.contents.set(returning.id, { content: "same", revision: 1 });
    await page.deleteNode(returning.id);
    const before = structuredClone({
      nodes: server.nodes,
      contents: [...server.contents],
      trash: [...server.trash],
      jobs: server.jobs,
    });
    const request =
      entry === "sync"
        ? page.restoreNode(returning.id)
        : new TransferService(server.app.session.public).startTrashRestore(notebookJSON.id, returning.id);
    await expect(request).rejects.toMatchObject({
      code: "page.source_duplicate",
      status: 409,
      problem: { duplicate_of: source.id },
    });
    expect({
      nodes: server.nodes,
      contents: [...server.contents],
      trash: [...server.trash],
      jobs: server.jobs,
    }).toEqual(before);
  });

  test(`${entry} restore with missing source parent falls back outside without duplicate refusal`, async () => {
    const removedParent = pageNode(841, "removed", root),
      returning = pageNode(842, "returning", removedParent);
    const server = jobsServer({
      role: "admin",
      mode: "llm_wiki",
      sourceRootId: root.id,
      nodes: [root, source, removedParent, returning],
    });
    const page = new PageService(server.app.session.public);
    server.contents.set(source.id, { content: "same", revision: 1 });
    server.contents.set(returning.id, { content: "same", revision: 1 });
    await page.deleteNode(returning.id);
    await page.deleteNode(removedParent.id);
    const request =
      entry === "sync"
        ? page.restoreNode(returning.id)
        : new TransferService(server.app.session.public).startTrashRestore(notebookJSON.id, returning.id);
    await expect(request).resolves.toBeDefined();
  });
}

test.each(["sync", "background"] as const)(
  "%s restore rejects a zero-byte source attachment and retains trash, data and jobs",
  async (entry) => {
    const { server, transfer } = fixture("admin");
    const page = new PageService(server.app.session.public);
    const uploads = new AssetService(server.app.session.public, server.app.transfer);
    const returning = await uploads.upload(notebookJSON.id, {
      name: "returning-empty.bin",
      parent: root.id,
      file: new Blob([]),
    });
    await page.deleteNode(returning.id);
    const live = await uploads.upload(notebookJSON.id, {
      name: "live-empty.bin",
      parent: root.id,
      file: new Blob([]),
    });
    const before = structuredClone({
      nodes: server.nodes,
      bytes: [...server.sourceAssetBytes],
      contents: [...server.contents],
      versions: [...server.versions],
      trash: [...server.trash],
      history: server.changesets,
      jobs: server.jobs,
    });
    const request =
      entry === "sync" ? page.restoreNode(returning.id) : transfer.startTrashRestore(notebookJSON.id, returning.id);
    await expect(request).rejects.toMatchObject({
      status: 409,
      code: "page.source_duplicate",
      problem: { duplicate_of: live.id },
    });
    expect({
      nodes: server.nodes,
      bytes: [...server.sourceAssetBytes],
      contents: [...server.contents],
      versions: [...server.versions],
      trash: [...server.trash],
      history: server.changesets,
      jobs: server.jobs,
    }).toEqual(before);
  }
);
