import { expect, test } from "vitest";

import { PageService } from "../services/page.service";
import { TransferService } from "../services/transfer.service";
import { notebookJSON } from "./fakes";
import { created, deleted, moved, wrote, renamed, restored } from "./history-server";
import { jobsServer } from "./jobs-server";
import { ada, assetNode, pageNode, pageServer } from "./page-server";
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

for (const entry of ["sync", "background"] as const) {
  for (const first of ["folder", "child"] as const) {
    for (const order of ["move-first", "rename-first"] as const) {
      for (const role of ["editor", "admin"] as const) {
        test(`${entry} history checks management at each placement (${first} UUID first, ${order}, ${role})`, async () => {
          const raw = pageNode(120, "raw");
          const folder = pageNode(first === "folder" ? 121 : 122, "Folder");
          const page = pageNode(first === "child" ? 121 : 122, "New", folder);
          const server = jobsServer({ role, mode: "llm_wiki", sourceRootId: raw.id, nodes: [raw, folder, page] });
          server.contents.set(page.id, { content: "unchanged body", revision: 1 });
          const changes = [moved(folder, raw.id), renamed(page, "Old")];
          let writes = 0;
          const change = server.log("moved ancestor out then renamed child", {
            changes: order === "move-first" ? changes : changes.toReversed(),
            undo: () => {
              writes += 1;
              server.nodes = [raw, { ...folder, parent_id: raw.id }, { ...page, name: "Old" }];
            },
          });
          const before = historyState(server);
          // Both ID orders end at raw/Folder/Old. Only the folder-first order
          // renames inside source; the history array itself is not an action list.
          if (first === "folder" && role === "editor") {
            const request =
              entry === "sync"
                ? new PageService(server.app.session.public).revertChangeset(change.id)
                : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
            await expect(request).rejects.toMatchObject({ code: "page.source_admin_only", status: 403 });
            expect(writes).toBe(0);
            expect(historyState(server)).toEqual(before);
          } else if (entry === "sync") {
            const result = await new PageService(server.app.session.public).revertChangeset(change.id);
            expect(result).toMatchObject({ changed: true });
            expect(writes).toBe(1);
            expect(server.nodes).toEqual([raw, { ...folder, parent_id: raw.id }, { ...page, name: "Old" }]);
            expect([...server.contents]).toEqual(before.contents);
            expect(server.trash.size).toBe(0);
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
    }
  }

  for (const direction of ["out of source", "into source"] as const) {
    test(`${entry} history checks a move's own Before permission (${direction})`, async () => {
      const raw = pageNode(123, "raw");
      const page = pageNode(124, "Evidence", direction === "out of source" ? raw : undefined);
      const parent = direction === "out of source" ? null : raw.id;
      const server = jobsServer({ role: "editor", mode: "llm_wiki", sourceRootId: raw.id, nodes: [raw, page] });
      server.contents.set(page.id, { content: "unchanged body", revision: 1 });
      let writes = 0;
      const change = server.log("moved page", {
        changes: [moved(page, parent)],
        undo: () => {
          writes += 1;
          server.nodes = [raw, { ...page, parent_id: parent }];
        },
      });
      const before = historyState(server);
      if (direction === "out of source") {
        const request =
          entry === "sync"
            ? new PageService(server.app.session.public).revertChangeset(change.id)
            : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
        await expect(request).rejects.toMatchObject({ code: "page.source_admin_only", status: 403 });
        expect(writes).toBe(0);
        expect(historyState(server)).toEqual(before);
      } else if (entry === "sync") {
        const result = await new PageService(server.app.session.public).revertChangeset(change.id);
        expect(result).toMatchObject({ changed: true });
        expect(writes).toBe(1);
        expect(server.nodes).toEqual([raw, { ...page, parent_id: raw.id }]);
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

// A preceding placement can remove one carried member before the large folder
// re-enters sources. The history array is intentionally the opposite order:
// checking only the initial or final tree loses one of these paired outcomes.
for (const entry of ["sync", "background"] as const) {
  for (const first of ["folder", "child"] as const) {
    test(`${entry} source entry counts the current subtree (${first} UUID first)`, async () => {
      const raw = pageNode(80, "raw");
      const outside = pageNode(81, "Outside");
      const folder = pageNode(first === "folder" ? 82 : 83, "Large");
      const child = assetNode(first === "child" ? 82 : 83, "Leaves.txt", folder);
      const rest = Array.from({ length: 9999 }, (_, i) => ({
        ...(i % 2 === 0 ? pageNode(84, `Empty ${i}`, folder) : assetNode(84, `File ${i}.txt`, folder)),
        id: `0199a2b4-0000-7000-8000-${(40000 + i).toString(16).padStart(12, "0")}`,
      }));
      const server = jobsServer({
        role: "admin",
        mode: "llm_wiki",
        sourceRootId: raw.id,
        nodes: [raw, outside, folder, child, ...rest],
      });
      for (const node of rest) {
        if (node.kind === "page") server.contents.set(node.id, { content: "", revision: 1 });
      }
      let writes = 0;
      const changes = [moved(folder, raw.id), moved(child, outside.id)];
      const change = server.log("large folder out, child in", {
        changes: first === "folder" ? changes.toReversed() : changes,
        undo: () => {
          writes += 1;
          server.nodes = server.nodes.map((node) =>
            node.id === folder.id
              ? { ...node, parent_id: raw.id }
              : node.id === child.id
                ? { ...node, parent_id: outside.id }
                : node
          );
        },
      });
      const before = historyState(server);
      const request =
        entry === "sync"
          ? new PageService(server.app.session.public).revertChangeset(change.id)
          : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
      if (first === "folder") {
        await expect(request).rejects.toMatchObject({ code: "page.source_too_large", status: 422 });
        expect(writes).toBe(0);
        expect(historyState(server)).toEqual(before);
      } else if (entry === "sync") {
        await expect(request).resolves.toMatchObject({ changed: true });
        expect(writes).toBe(1);
        expect(server.nodes.find((node) => node.id === folder.id)?.parent_id).toBe(raw.id);
        expect(server.nodes.find((node) => node.id === child.id)?.parent_id).toBe(outside.id);
        expect(server.nodes.filter((node) => node.parent_id === folder.id)).toHaveLength(9999);
      } else {
        await expect(request).resolves.toMatchObject({ state: "queued", target_changeset_id: change.id });
        expect(writes).toBe(0);
        expect(server.nodes).toEqual(before.nodes);
        expect(server.jobs).toHaveLength(1);
      }
    });
  }
}

// C2: restoring a deleted target parent is not a source-entry itself, but the
// waiting placement of an existing Large subtree afterwards is. The size is
// Large's current members, not the restore plan's count or a body-write phase.
for (const entry of ["sync", "background"] as const) {
  test.each([10000, 10001])(`${entry} source entry waits for its deleted target parent (%i nodes)`, async (size) => {
    const raw = pageNode(104, "raw");
    const parent = pageNode(105, "P", raw);
    const large = pageNode(106, "Large");
    const children = Array.from({ length: size - 1 }, (_, i) => ({
      ...(i % 2 === 0 ? pageNode(107, `Empty ${i}`, large) : assetNode(107, `File ${i}.txt`, large)),
      id: `0199a2b4-0000-7000-8000-${(100000 + i).toString(16).padStart(12, "0")}`,
    }));
    const server = jobsServer({
      role: "admin",
      mode: "llm_wiki",
      sourceRootId: raw.id,
      nodes: [raw, large, ...children],
    });
    for (const node of children) {
      if (node.kind === "page") server.contents.set(node.id, { content: "", revision: 1 });
    }
    server.contents.set(parent.id, { content: "", revision: 1 });
    server.trash.set(parent.id, trashed([raw, parent, large, ...children], [parent], 1, ada));
    let writes = 0;
    const change = server.log("moved Large out then deleted empty P", {
      changes: [moved(large, parent.id), deleted(parent)],
      undo: () => {
        writes += 1;
        server.nodes = [raw, parent, { ...large, parent_id: parent.id }, ...children];
        server.trash.delete(parent.id);
      },
    });
    const before = historyState(server);
    const request =
      entry === "sync"
        ? new PageService(server.app.session.public).revertChangeset(change.id)
        : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
    if (size === 10001) {
      await expect(request).rejects.toMatchObject({ status: 422, code: "page.source_too_large" });
      expect(writes).toBe(0);
      expect(historyState(server)).toEqual(before);
    } else if (entry === "sync") {
      const result = await request;
      expect(result).toMatchObject({ changed: true });
      expect(writes).toBe(1);
      expect(server.nodes).toEqual([raw, parent, { ...large, parent_id: parent.id }, ...children]);
      expect(server.trash.size).toBe(0);
      expect([...server.contents]).toEqual(before.contents);
      expect(server.changesets).toHaveLength(2);
      expect(server.jobs).toEqual([]);
    } else {
      const result = await request;
      expect(result).toMatchObject({ state: "queued", target_changeset_id: change.id });
      expect(writes).toBe(0);
      expect(historyState(server)).toEqual({ ...before, jobs: [result] });
    }
  });
}

// The restore phase returns every ready trash group before another placement
// round. A restored child of Large is carried too, even though it is not on the
// missing destination's ancestor chain.
for (const entry of ["sync", "background"] as const) {
  test.each([9999, 10000])(`${entry} source entry counts its restored child (%i live nodes)`, async (liveSize) => {
    const raw = pageNode(110, "raw");
    const parent = pageNode(111, "P", raw);
    const large = pageNode(112, "Large");
    const restoredChild = assetNode(113, "Restored.txt", large);
    const children = Array.from({ length: liveSize - 1 }, (_, i) => ({
      ...(i % 2 === 0 ? pageNode(114, `Empty ${i}`, large) : assetNode(114, `File ${i}.txt`, large)),
      id: `0199a2b4-0000-7000-8000-${(120000 + i).toString(16).padStart(12, "0")}`,
    }));
    const server = jobsServer({
      role: "admin",
      mode: "llm_wiki",
      sourceRootId: raw.id,
      nodes: [raw, large, ...children],
    });
    for (const node of children) {
      if (node.kind === "page") server.contents.set(node.id, { content: "", revision: 1 });
    }
    server.contents.set(parent.id, { content: "", revision: 1 });
    const all = [raw, parent, large, restoredChild, ...children];
    server.trash.set(parent.id, trashed(all, [parent], 1, ada));
    server.trash.set(restoredChild.id, trashed(all, [restoredChild], 2, ada));
    let writes = 0;
    const change = server.log("moved Large out then deleted P and a carried child", {
      changes: [moved(large, parent.id), deleted(parent), deleted(restoredChild)],
      undo: () => {
        writes += 1;
        server.nodes = [raw, parent, { ...large, parent_id: parent.id }, restoredChild, ...children];
        server.trash.delete(parent.id);
        server.trash.delete(restoredChild.id);
      },
    });
    const before = historyState(server);
    const request =
      entry === "sync"
        ? new PageService(server.app.session.public).revertChangeset(change.id)
        : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
    if (liveSize === 10000) {
      await expect(request).rejects.toMatchObject({ status: 422, code: "page.source_too_large" });
      expect(writes).toBe(0);
      expect(historyState(server)).toEqual(before);
    } else if (entry === "sync") {
      const result = await request;
      expect(result).toMatchObject({ changed: true });
      expect(writes).toBe(1);
      expect(server.nodes).toEqual([raw, parent, { ...large, parent_id: parent.id }, restoredChild, ...children]);
      expect(server.nodes.filter((node) => node.parent_id === large.id)).toHaveLength(9999);
      expect(server.trash.size).toBe(0);
      expect([...server.contents]).toEqual(before.contents);
      expect(server.changesets).toHaveLength(2);
      expect(server.jobs).toEqual([]);
    } else {
      const result = await request;
      expect(result).toMatchObject({ state: "queued", target_changeset_id: change.id });
      expect(writes).toBe(0);
      expect(historyState(server)).toEqual({ ...before, jobs: [result] });
    }
  });
}

for (const entry of ["sync", "background"] as const) {
  test(`${entry} history refuses bringing a duplicate source back before undo or scheduling`, async () => {
    const incoming = pageNode(811, "incoming");
    const server = jobsServer({
      role: "admin",
      mode: "llm_wiki",
      sourceRootId: root.id,
      nodes: [root, source, incoming],
    });
    server.contents.set(source.id, { content: "identical", revision: 1 });
    server.contents.set(incoming.id, { content: "identical", revision: 1 });
    let undos = 0;
    const change = server.log("moved source out", {
      changes: [moved(incoming, root.id)],
      undo: () => {
        undos += 1;
      },
    });
    const before = historyState(server);
    const request =
      entry === "sync"
        ? new PageService(server.app.session.public).revertChangeset(change.id)
        : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
    await expect(request).rejects.toMatchObject({
      status: 409,
      code: "page.source_duplicate",
      problem: { duplicate_of: source.id },
    });
    expect(undos).toBe(0);
    expect(historyState(server)).toEqual(before);
  });

  test(`${entry} history allows source replacement after the matching source moves out`, async () => {
    const incoming = pageNode(821, "incoming");
    const departing = pageNode(820, "departing", root);
    const server = jobsServer({
      role: "admin",
      mode: "llm_wiki",
      sourceRootId: root.id,
      nodes: [root, departing, incoming],
    });
    server.contents.set(departing.id, { content: "identical", revision: 1 });
    server.contents.set(incoming.id, { content: "identical", revision: 1 });
    // Undo first moves the baseline source out, then brings the candidate in.
    const change = server.log("exchanged places", { changes: [moved(departing, null), moved(incoming, root.id)] });
    const request =
      entry === "sync"
        ? new PageService(server.app.session.public).revertChangeset(change.id)
        : new TransferService(server.app.session.public).startRevert(notebookJSON.id, change.id);
    // A source moved out is no longer a collision in the active set.
    await expect(request).resolves.toBeDefined();
  });
}
