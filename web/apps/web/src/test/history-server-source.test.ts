import { expect, test } from "vitest";

import { PageService } from "../services/page.service";
import { created, wrote, renamed } from "./history-server";
import { pageNode, pageServer } from "./page-server";

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
