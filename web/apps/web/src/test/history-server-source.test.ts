import { expect, test } from "vitest";

import { PageService } from "../services/page.service";
import { TransferService } from "../services/transfer.service";
import { notebookJSON } from "./fakes";
import { created, deleted, moved, wrote, renamed, restored } from "./history-server";
import { jobsServer } from "./jobs-server";
import { ada, pageNode, pageServer } from "./page-server";
import { trashed } from "./trash-server";

const root = pageNode(40, "raw");
const source = pageNode(41, "source", root);

function fixture(role: "admin" | "editor" = "editor", mode: "llm_wiki" | "standard" = "llm_wiki") {
  const options = { role, mode, sourceRootId: root.id, nodes: [root, source] };
  const server = pageServer(options);
  return { server, page: new PageService(server.app.session.public) };
}

test("source history rejects content undo before locks and busy, without calling undo or writing history", async () => {
  const { server, page } = fixture("admin");
  let writes = 0;
  const change = server.log("source edit", {
    changes: [wrote(source, 1, 2)],
    undo: () => {
      writes += 1;
    },
  });
  server.revertsBusy = true;
  await expect(page.revertChangeset(change.id)).rejects.toMatchObject({ status: 409, code: "page.source_immutable" });
  expect(writes).toBe(0);
  expect(server.changesets).toEqual([change]);
  expect(change.revertedBy).toEqual([]);
  change.trimmed = true;
  await expect(page.revertChangeset(change.id)).rejects.toMatchObject({ code: "page.changeset_trimmed" });
});

test("source history uses tree management permissions and allows admin; unchanged edits stay no-ops", async () => {
  const { server, page } = fixture();
  const change = server.log("source rename", { changes: [renamed(source, "previous")] });
  await expect(page.revertChangeset(change.id)).rejects.toMatchObject({ status: 403, code: "page.source_admin_only" });
  const admin = fixture("admin");
  const managed = admin.server.log("source rename", { changes: [renamed(source, "previous")] });
  await expect(admin.page.revertChangeset(managed.id)).resolves.toMatchObject({ changed: true });
  const noop = server.log("unchanged", { changes: [wrote(source, 1, 2)], unchanged: true });
  await expect(page.revertChangeset(noop.id)).resolves.toEqual({ changed: false, changeset_id: null });
  const protectedRoot = admin.server.log("source root creation", { changes: [created(root)] });
  await expect(admin.page.revertChangeset(protectedRoot.id)).rejects.toMatchObject({ code: "page.protected" });
});

test("standard history accepts a source-shaped content change", async () => {
  const { server, page } = fixture("editor", "standard");
  const change = server.log("ordinary edit", { changes: [wrote(source, 1, 2)] });
  await expect(page.revertChangeset(change.id)).resolves.toMatchObject({ changed: true });
});

function historyState(server: ReturnType<typeof jobsServer>) {
  return structuredClone({
    nodes: server.nodes,
    contents: [...server.contents],
    trash: [...server.trash],
    history: server.changesets.map((change) => ({ ...change, undo: undefined })),
    jobs: server.jobs,
  });
}

// These are the simple reverse placements the fixture represents, before its
// writeback. Background assertions cover admission only; the real worker's
// ordering and durable writes are checked by the bootstrap tests.
for (const entry of ["sync", "background"] as const) {
  test(`${entry} history leaves a cycle-blocked placement waiting during writeback`, async () => {
    const raw = pageNode(107, "raw");
    const c = pageNode(108, "C");
    const b = pageNode(110, "B", raw);
    const a = pageNode(109, "A", b);
    const server = jobsServer({ role: "admin", mode: "llm_wiki", sourceRootId: raw.id, nodes: [raw, a, b] });
    server.contents.set(a.id, { content: "changed outside", revision: 2 });
    server.trash.set(c.id, trashed([c], [c], 1, ada));
    let writes = 0;
    const change = server.log("edited A, moved B then A into source, deleted C", {
      changes: [{ ...moved(a, c.id), content: { base: 1, revision: 2 } }, moved(b, a.id), deleted(c)],
      undo: () => {
        writes += 1;
      },
    });
    const before = historyState(server);
    let parentReads = 0;
    // Bound observation of the unchanged A parent. A bad partial projection
    // makes B point to A while A still points to B: fail on repeated walking
    // rather than hang in isUnder and mistake a timeout for a business refusal.
    Object.defineProperty(a, "parent_id", {
      get: () => {
        parentReads += 1;
        if (parentReads > 30) {
          throw new Error("cyclic projected parent walk");
        }
        return b.id;
      },
    });
    const request =
      entry === "sync"
        ? new PageService(server.app.session.public).revertChangeset(change.id)
        : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
    await expect(request).rejects.toMatchObject({ code: "page.source_immutable", status: 409 });
    expect(writes).toBe(0);
    expect(historyState(server)).toEqual(before);
    expect(parentReads).toBeLessThan(30);
  });

  for (const shape of [
    { label: "page moves back into source", ancestor: false, inside: true, restore: false, refused: true },
    { label: "page moves back out of source", ancestor: false, inside: false, restore: false, refused: false },
    { label: "ancestor moves back into source", ancestor: true, inside: true, restore: false, refused: true },
    { label: "ancestor moves back out of source", ancestor: true, inside: false, restore: false, refused: false },
    {
      label: "page waits for its deleted source ancestor",
      ancestor: false,
      inside: true,
      restore: true,
      refused: false,
    },
    {
      label: "page waits for its deleted outside ancestor",
      ancestor: false,
      inside: false,
      restore: true,
      refused: true,
    },
  ]) {
    test(`${entry} history checks writeback when ${shape.label}`, async () => {
      const raw = pageNode(91, "renamed source root");
      const folder = pageNode(92, "Folder", shape.inside ? undefined : raw);
      const page = pageNode(93, "Evidence", shape.ancestor ? folder : shape.inside ? undefined : raw);
      const nodes = shape.restore ? [raw, page] : [raw, folder, page];
      const server = jobsServer({ role: "admin", mode: "llm_wiki", sourceRootId: raw.id, nodes });
      server.contents.set(page.id, { content: "changed outside", revision: 2 });
      const restoredFolder = { ...folder, parent_id: shape.inside ? raw.id : null };
      if (shape.restore) {
        server.trash.set(folder.id, trashed([raw, restoredFolder, page], [restoredFolder], 1, ada));
      }
      const parent = shape.restore ? folder.id : shape.inside ? raw.id : null;
      const restoredNodes = shape.restore
        ? [raw, restoredFolder, { ...page, parent_id: folder.id }]
        : shape.ancestor
          ? [raw, { ...folder, parent_id: parent }, page]
          : [raw, folder, { ...page, parent_id: parent }];
      let writes = 0;
      const changes = shape.ancestor
        ? [wrote(page, 1, 2), moved(folder, parent)]
        : [{ ...moved(page, parent), content: { base: 1, revision: 2, rewrite: false } }];
      // History rows are not actions: the descendant's content row deliberately
      // precedes the ancestor's structural row.
      if (shape.restore) {
        changes.push(deleted(restoredFolder));
      }
      const change = server.log(shape.label, {
        changes,
        undo: () => {
          writes += 1;
          server.nodes = restoredNodes;
          server.trash.delete(folder.id);
          server.contents.set(page.id, { content: "original outside", revision: 3 });
        },
      });
      const before = historyState(server);
      // A missing target parent defers the place until a later scheduler round.
      // The still-live page writes at its current position first; entering the
      // source later succeeds, but leaving it later cannot permit that write.
      if (shape.refused) {
        const request =
          entry === "sync"
            ? new PageService(server.app.session.public).revertChangeset(change.id)
            : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
        await expect(request).rejects.toMatchObject({ code: "page.source_immutable", status: 409 });
        expect(writes).toBe(0);
        expect(historyState(server)).toEqual(before);
      } else if (entry === "sync") {
        const result = await new PageService(server.app.session.public).revertChangeset(change.id);
        expect(result).toMatchObject({ changed: true });
        expect(writes).toBe(1);
        expect(server.nodes).toEqual(restoredNodes);
        expect(server.contents.get(page.id)).toEqual({ content: "original outside", revision: 3 });
        expect(server.changesets).toHaveLength(2);
        expect(change.revertedBy).toEqual([result.changeset_id]);
        expect(server.changesets[1]).toMatchObject({ id: result.changeset_id, reverts: change.id });
        expect(server.jobs).toEqual([]);
      } else {
        const job = await new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
        expect(job).toMatchObject({ state: "queued", kind: "revert", target_changeset_id: change.id });
        expect(server.jobs).toEqual([job]);
        expect(writes).toBe(0);
        expect(historyState(server)).toEqual({ ...before, jobs: [job] });
      }
    });
  }

  for (const parentChange of [created, restored]) {
    test(`${entry} history moves a surviving child before undoing its parent's ${parentChange.name}`, async () => {
      const raw = pageNode(94, "raw");
      const folder = pageNode(95, "New outside folder");
      const page = pageNode(96, "Evidence", folder);
      const server = jobsServer({ role: "admin", mode: "llm_wiki", sourceRootId: raw.id, nodes: [raw, folder, page] });
      server.contents.set(page.id, { content: "changed outside", revision: 2 });
      let writes = 0;
      const change = server.log("source moved to new outside folder and edited", {
        changes: [parentChange(folder), { ...moved(page, raw.id), content: { base: 1, revision: 2 } }],
        undo: () => {
          writes += 1;
        },
      });
      const before = historyState(server);
      const request =
        entry === "sync"
          ? new PageService(server.app.session.public).revertChangeset(change.id)
          : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
      await expect(request).rejects.toMatchObject({ code: "page.source_immutable", status: 409 });
      expect(writes).toBe(0);
      expect(historyState(server)).toEqual(before);
    });
  }

  for (const boundary of ["root body", "system rewrite", "standard"] as const) {
    test(`${entry} history preserves ${boundary} writeback at its reverse position`, async () => {
      const raw = pageNode(97, "renamed source root");
      const page = boundary === "root body" ? raw : pageNode(98, "Evidence");
      const mode = boundary === "standard" ? "standard" : "llm_wiki";
      const server = jobsServer({
        role: "admin",
        mode,
        sourceRootId: raw.id,
        nodes: page === raw ? [raw] : [raw, page],
      });
      server.contents.set(page.id, { content: "current body", revision: 2 });
      let writes = 0;
      const change = server.log(boundary, {
        changes: [
          {
            ...(boundary === "root body" ? renamed(raw, "raw") : moved(page, raw.id)),
            content: { base: 1, revision: 2, rewrite: boundary === "system rewrite" },
          },
        ],
        undo: () => {
          writes += 1;
          server.contents.set(page.id, { content: "original body", revision: 3 });
          server.nodes = boundary === "root body" ? [{ ...raw, name: "raw" }] : [raw, { ...page, parent_id: raw.id }];
        },
      });
      const before = historyState(server);
      if (entry === "sync") {
        await expect(new PageService(server.app.session.public).revertChangeset(change.id)).resolves.toMatchObject({
          changed: true,
        });
        expect(writes).toBe(1);
        expect(server.contents.get(page.id)).toEqual({ content: "original body", revision: 3 });
        expect(server.changesets).toHaveLength(2);
      } else {
        const job = await new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
        expect(job).toMatchObject({ state: "queued", target_changeset_id: change.id });
        expect(writes).toBe(0);
        expect(historyState(server)).toEqual({ ...before, jobs: [job] });
      }
    });
  }
}

for (const entry of ["sync", "background"] as const) {
  for (const order of ["created-first", "moved-first", "restored-first"] as const) {
    test(`${entry} history moves the source root before deleting its parent (${order})`, async () => {
      const parent = pageNode(101, "A");
      const raw = pageNode(102, "renamed raw", parent);
      const page = pageNode(103, "Evidence", raw);
      const server = jobsServer({ role: "admin", mode: "llm_wiki", sourceRootId: raw.id, nodes: [parent, raw, page] });
      server.contents.set(parent.id, { content: "ordinary parent", revision: 1 });
      server.contents.set(raw.id, { content: "source instructions", revision: 1 });
      server.contents.set(page.id, { content: "original source\r\n", revision: 1 });
      const changes = [order === "restored-first" ? restored(parent) : created(parent), moved(raw, null)];
      let writes = 0;
      const change = server.log("parent gained source root", {
        changes: order === "moved-first" ? changes.toReversed() : changes,
        undo: () => {
          writes += 1;
          server.nodes = server.nodes.map((node) => (node.id === raw.id ? { ...node, parent_id: null } : node));
          server.trashNode(parent.id);
        },
      });
      const before = historyState(server);
      if (entry === "sync") {
        const result = await new PageService(server.app.session.public).revertChangeset(change.id);
        expect(result).toMatchObject({ changed: true });
        expect(writes).toBe(1);
        expect(server.nodes).toEqual([{ ...raw, parent_id: null }, page]);
        expect(server.trash.get(parent.id)?.members).toEqual([parent]);
        expect([...server.contents]).toEqual(before.contents);
        expect(server.changesets).toHaveLength(2);
        expect(change.revertedBy).toEqual([result.changeset_id]);
        expect(server.jobs).toEqual([]);
      } else {
        const job = await new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
        expect(job).toMatchObject({ state: "queued", target_changeset_id: change.id });
        expect(writes).toBe(0);
        expect(historyState(server)).toEqual({ ...before, jobs: [job] });
      }
    });
  }

  for (const boundary of ["root deletion", "ancestor still carries root", "editor moves root"] as const) {
    test(`${entry} history keeps protection for ${boundary}`, async () => {
      const parent = pageNode(104, "A");
      const raw = pageNode(105, "raw", parent);
      const page = pageNode(106, "Evidence", raw);
      const role = boundary === "editor moves root" ? "editor" : "admin";
      const server = jobsServer({ role, mode: "llm_wiki", sourceRootId: raw.id, nodes: [parent, raw, page] });
      let writes = 0;
      const change = server.log(boundary, {
        changes:
          boundary === "root deletion"
            ? [restored(raw)]
            : boundary === "ancestor still carries root"
              ? [created(parent)]
              : [moved(raw, null)],
        undo: () => {
          writes += 1;
        },
      });
      const before = historyState(server);
      const request =
        entry === "sync"
          ? new PageService(server.app.session.public).revertChangeset(change.id)
          : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
      const code = boundary === "editor moves root" ? "page.source_admin_only" : "page.protected";
      await expect(request).rejects.toMatchObject({ code, status: code === "page.source_admin_only" ? 403 : 409 });
      expect(writes).toBe(0);
      expect(historyState(server)).toEqual(before);
    });
  }
}
