import type { ApiClient, TreeNode } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectChangesetOf, expectContentWritten, expectRenamed } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { errorOf, mcpClient, resultOf, writtenOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  deleteNode,
  getPage,
  listNodes,
  postMove,
  postPage,
  readContent,
  renameNode,
  writeContent,
} from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageTree, renameDialog, renamePageWith, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// LW2, an LLM Wiki's protected pages (M10 design 4.9; M10/P1 design 5.2):
// AGENTS is not deleted, renamed or moved, though its content is written;
// raw, the source root, is not deleted, and only an admin renames it; no
// page at the root is named index. Each refusal leaves the tree as it was.

/** An edit's answer, as the tool's structuredContent has it. */
interface Edited {
  node_id: string;
  version: string;
  changed: boolean;
  at: string | null;
  changeset_id: string | null;
}

/** An LLM Wiki of the team's, open to its members as editors, with its template's raw and AGENTS. */
async function protectedWiki(api: ApiClient, pat: string, slug: string) {
  const notebook = await createNotebook(api, pat, slug, "Field notes", "editor", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  const nodes = await listNodes(api, pat, notebook.id);
  const [raw, agents] = nodes;
  if (raw?.name !== "raw" || agents?.name !== "AGENTS" || raw.id !== notebook.source_root_id) {
    throw new Error(`the LLM Wiki's root is ${JSON.stringify(nodes)}: want raw, its source root, then AGENTS`);
  }
  return { notebook, raw, agents, nodes };
}

/** The notebook's tree and the page's state, as a refusal must leave them. */
async function stateOf(api: ApiClient, pat: string, notebookId: string, page: TreeNode) {
  return { nodes: await listNodes(api, pat, notebookId), page: (await getPage(api, pat, page.id)).data };
}

test("LW2 (API): an LLM Wiki keeps AGENTS from deletion, rename and move, though its content is written; raw from deletion, and from a writer's rename, not an admin's; and no page at its root is named index", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const writerEmail = emailFor(testInfo, "writer");
  const writerPat = await joinAs(api, pat, workspace.slug, writerEmail, "member");
  const writerId = await accountIdOf(db, writerEmail);
  const { notebook, raw, agents } = await protectedWiki(api, pat, workspace.slug);

  // AGENTS: kept in its place, its content written as any page's.
  const agentsBefore = await stateOf(api, pat, notebook.id, agents);
  const kept = await Promise.all([
    deleteNode(api, pat, agents.id),
    renameNode(api, pat, agents.id, "Rules"),
    postMove(api, pat, agents.id, { parent_id: raw.id }),
  ]);
  expect(kept.map((a) => [a.response.status, a.error?.code])).toEqual([
    [409, "page.protected"],
    [409, "page.protected"],
    [409, "page.protected"],
  ]);
  expect(await stateOf(api, pat, notebook.id, agents)).toEqual(agentsBefore);
  const { content, revision } = await readContent(api, pat, agents.id);
  const rewritten = `${content}\nKeep a glossary.\n`;
  const written = await writeContent(api, writerPat, agents.id, { content: rewritten, base_revision: revision });
  await expectContentWritten(db, written, rewritten, writerId);

  // raw: kept from deletion, by anyone, and from a writer's rename.
  const rawBefore = await stateOf(api, pat, notebook.id, raw);
  const deleted = await deleteNode(api, pat, raw.id);
  expect([deleted.response.status, deleted.error?.code]).toEqual([409, "page.protected"]);
  const writers = await renameNode(api, writerPat, raw.id, "sources");
  expect([writers.response.status, writers.error?.code]).toEqual([403, "page.source_admin_only"]);
  expect(await stateOf(api, pat, notebook.id, raw)).toEqual(rawBefore);
  const renamed = await renameNode(api, pat, raw.id, "sources");
  expect(renamed.response.status).toBe(200);
  await expectRenamed(db, raw.id, "raw", "sources", adminId);

  // index at the root is the generated index's name.
  const treeBefore = await listNodes(api, pat, notebook.id);
  const index = await postPage(api, writerPat, notebook.id, { parent_id: null, title: "index" });
  expect([index.response.status, index.error?.code]).toEqual([409, "page.virtual_file"]);
  expect(await listNodes(api, pat, notebook.id)).toEqual(treeBefore);
});

test("LW2 (MCP): a writer's agent of an LLM Wiki cannot delete or move AGENTS.md, nor write under index at its root, though it edits AGENTS.md", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const writerEmail = emailFor(testInfo, "writer");
  const writerPat = await joinAs(api, pat, workspace.slug, writerEmail, "member");
  const writerId = await accountIdOf(db, writerEmail);
  const { notebook, agents, nodes } = await protectedWiki(api, pat, workspace.slug);
  const agent = await mcpClient(nervewiki.baseURL, writerPat, { notebook: notebook.id });

  const refused = await Promise.all([
    agent.call("delete", { path: "AGENTS.md" }),
    agent.call("move", { from: "AGENTS.md", to: "raw/AGENTS.md" }),
    agent.call("write", { path: "index/x.md", content: "x" }),
  ]);
  expect(refused.map((a, i) => errorOf(a, `call ${i}`).code)).toEqual([
    "page.protected",
    "page.protected",
    "page.virtual_file",
  ]);
  expect(await listNodes(api, pat, notebook.id)).toEqual(nodes);

  const edited = resultOf<Edited>(
    await agent.call("edit", {
      path: "AGENTS.md",
      old_string: "# How this LLM Wiki is kept",
      new_string: "# How we keep this wiki",
    }),
    "edit AGENTS"
  );
  const { content } = await readContent(api, pat, agents.id);
  expect(content.split("\n")[0]).toBe("# How we keep this wiki");
  await expectContentWritten(db, writtenOf(edited, "edit AGENTS"), content, writerId);
  await expectChangesetOf(db, String(edited.changeset_id), {
    client: "mcp:e2e-agent",
    byId: writerId,
    items: [],
    versions: [agents.id],
  });
});

test("LW2 (page): an LLM Wiki's tree protects AGENTS and its source root; an admin renames the source root, while index stays reserved", async ({
  api,
  db,
  signedInPage,
  anotherPage,
  pageWatch,
}, testInfo) => {
  const { adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const writer = await anotherPage(
    await joinOnboarded(api, pat, workspace.slug, emailFor(testInfo, "writer"), "member")
  );
  const { notebook, raw, agents } = await protectedWiki(api, pat, workspace.slug);
  const notes = await createPage(api, pat, notebook.id, "Notes");
  await writer.goto(wikiPagePath(workspace.slug, notebook.id, agents.id));
  const writersTree = pageTree(writer, notebook.name);
  const writersAgents = writersTree.getByRole("link", { name: "AGENTS (instructions for agents)", exact: true });
  const writersRaw = writersTree.getByRole("link", { name: "raw (source area)", exact: true });
  await expect(writersAgents).toBeVisible();
  await expect(writersRaw).toBeVisible();
  await expect(writersAgents.locator("..")).not.toHaveAttribute("draggable");
  await expect(writersRaw.locator("..")).not.toHaveAttribute("draggable");

  await writersTree.getByRole("button", { name: "Actions for AGENTS", exact: true }).click();
  const writersMenu = writer.getByRole("menu");
  await expect(writersMenu.getByRole("menuitem")).toHaveText(["New subpage"]);
  await expect(writersMenu.getByRole("menuitem", { name: "Rename", exact: true })).toHaveCount(0);
  await expect(writersMenu.getByRole("menuitem", { name: "Move to…", exact: true })).toHaveCount(0);
  await expect(writersMenu.getByRole("menuitem", { name: "Delete", exact: true })).toHaveCount(0);
  await writer.keyboard.press("Escape");
  await writersTree.getByRole("button", { name: "Actions for raw", exact: true }).click();
  await expect(writersMenu.getByRole("menuitem")).toHaveText(["New subpage"]);
  await expect(writersMenu.getByRole("menuitem", { name: "Rename", exact: true })).toHaveCount(0);
  await expect(writersMenu.getByRole("menuitem", { name: "Move to…", exact: true })).toHaveCount(0);
  await expect(writersMenu.getByRole("menuitem", { name: "Delete", exact: true })).toHaveCount(0);
  await writer.keyboard.press("Escape");

  const admin = await signedInPage(tokens);
  await admin.goto(wikiPagePath(workspace.slug, notebook.id, raw.id));
  const adminsTree = pageTree(admin, notebook.name);
  const adminsRaw = adminsTree.getByRole("link", { name: "raw (source area)", exact: true });
  await expect(adminsRaw).toBeVisible();
  await expect(adminsRaw.locator("..")).toHaveAttribute("draggable", "true");
  await adminsTree.getByRole("button", { name: "Actions for raw", exact: true }).click();
  const adminsMenu = admin.getByRole("menu");
  await expect(adminsMenu.getByRole("menuitem")).toHaveText(["New subpage", "Rename", "Move to…"]);
  await expect(adminsMenu.getByRole("menuitem", { name: "Delete", exact: true })).toHaveCount(0);
  await admin.keyboard.press("Escape");
  expect((await renamePageWith(admin, notebook.name, raw.id, "raw", "sources")).status()).toBe(200);
  await expect(renameDialog(admin, "raw")).toBeHidden();
  await expect(adminsTree.getByRole("link", { name: "sources (source area)", exact: true })).toBeVisible();
  await expectRenamed(db, raw.id, "raw", "sources", adminId);

  const before = await stateOf(api, pat, notebook.id, notes);
  const refused = await renamePageWith(admin, notebook.name, notes.id, "Notes", "index");
  expect(refused.status()).toBe(409);
  expect((await refused.json()) as { code: string }).toMatchObject({ code: "page.virtual_file" });
  await expect(renameDialog(admin, "Notes").getByRole("alert")).toHaveText(
    "This file is generated by the system: the index updates itself as pages change, and the log is the notebook's changes. No page or attachment at the root may be named index or log."
  );
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  expect(await stateOf(api, pat, notebook.id, notes)).toEqual(before);
  await admin.keyboard.press("Escape");
  await expect(renameDialog(admin, "Notes")).toBeHidden();
  await expect(adminsTree.getByRole("link", { name: "Notes", exact: true })).toBeVisible();
});
