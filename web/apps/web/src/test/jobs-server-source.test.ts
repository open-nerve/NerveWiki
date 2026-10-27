import { expect, test } from "vitest";

import { TransferService } from "../services/transfer.service";
import { notebookJSON } from "./fakes";
import { wrote, renamed } from "./history-server";
import { jobsServer } from "./jobs-server";
import { pageNode } from "./page-server";

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
