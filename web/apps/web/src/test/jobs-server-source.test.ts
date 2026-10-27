import { expect, test } from "vitest";

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
