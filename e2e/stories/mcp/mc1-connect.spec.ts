import { readFileSync } from "node:fs";
import path from "node:path";

import { Client } from "@modelcontextprotocol/sdk/client";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";

import { uploadAsset, utf8 } from "../../fixtures/assets";
import { createToken, emailFor, login, register } from "../../fixtures/auth";
import { expectTokenClient } from "../../fixtures/assert/identity";
import { mcpClient, mcpURL, postMcp } from "../../fixtures/mcp";
import { createNotebook, deleteNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode, renameNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { createWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// MC1, an agent connects and follows its notebook's instructions (M9 design
// 9; M9/P1 design 4.3): the handshake's instructions from the AGENTS page,
// the tools of each entry as the contract lists them, the account's
// notebooks, a notebook opened, the refusals before the SDK, and the
// official TypeScript client, each client's name recorded on its token.

/** Each entry's tools, as the contract's file lists them. */
const toolLists = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, "../../../api/mcp/tools.json"), "utf8")
) as Record<"notebook" | "account", { name: string }[]>;

/** tools by name: the file's order is not the server's. */
function byName(tools: { name: string }[]): Record<string, unknown> {
  return Object.fromEntries(tools.map((t) => [t.name, t]));
}

/** The built-in instructions of a notebook without an AGENTS page start so. */
const builtIn = "This is a notebook of Nerve Wiki";

/** open_notebook's answer, as much as the story reads. */
interface Opened {
  notebook: string;
  instructions: string;
  instructions_from: string | null;
  overview: {
    pages: number;
    attachments: number;
    last_changed_at: string | null;
    root: { path: string; kind: string }[];
  };
  truncated: boolean;
}

test("MC1: an agent connects with a personal access token, gets its notebook's AGENTS instructions and the contract's tools, lists its notebooks and opens one", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const team = await newTeam(api, testInfo);
  const session = await login(api, team.adminEmail);
  const first = await createToken(api, session.access_token, { name: "agent one" });
  const second = await createToken(api, session.access_token, { name: "agent two" });
  const handbook = await createNotebook(api, team.pat, team.workspace.slug, "Handbook");
  const agentsPage = await createPage(
    api,
    team.pat,
    handbook.id,
    "AGENTS",
    null,
    "---\ntags: [rules]\n---\n\n# Conventions\nWrite in English.\n"
  );
  const guide = await createPage(api, team.pat, handbook.id, "Guide");
  await createPage(api, team.pat, handbook.id, "Notes");
  await uploadAsset(api, team.pat, handbook.id, { name: "chart.txt", bytes: utf8("chart\n") }, guide.id);
  const rules = "# Conventions\nWrite in English.";

  // 1. The notebook entry: the AGENTS page without its frontmatter, the notebook entry's tools, the client's name.
  const agent = await mcpClient(nervewiki.baseURL, first.token, { notebook: handbook.id });
  expect(agent.protocolVersion).toBe("2025-11-25");
  expect(agent.instructions).toBe(rules);
  expect(byName(await agent.listTools())).toEqual(byName(toolLists.notebook));
  await expectTokenClient(db, first.id, "e2e-agent");

  // 2. The account entry: its tools, the account's notebooks in its two workspaces and no other team's, one opened:
  // its pages and attachments counted, its last change.
  const lab = await createWorkspace(api, team.pat, "Lab", slugFor(testInfo, "lab"));
  const research = await createNotebook(api, team.pat, lab.slug, "Research");
  const other = await register(api, emailFor(testInfo, "other-admin"));
  const otherWorkspace = await createWorkspace(api, other.access_token, "Other", slugFor(testInfo, "other"));
  const otherNotebook = await createNotebook(api, other.access_token, otherWorkspace.slug, "Secret");
  const account = await mcpClient(nervewiki.baseURL, first.token);
  expect(account.instructions?.startsWith("This is Nerve Wiki"), account.instructions).toBe(true);
  expect(byName(await account.listTools())).toEqual(byName(toolLists.account));
  const listed = await account.call("notebooks");
  expect(listed.isError, listed.text.join(" ")).toBe(false);
  const mine = [
    {
      id: handbook.id,
      name: "Handbook",
      workspace: { slug: team.workspace.slug, name: team.workspace.name },
      role: "admin",
      mode: "standard",
    },
    { id: research.id, name: "Research", workspace: { slug: lab.slug, name: "Lab" }, role: "admin", mode: "standard" },
  ].toSorted((a, b) => (a.workspace.slug < b.workspace.slug ? -1 : 1));
  expect(listed.structured).toEqual({ notebooks: mine, truncated: false });
  const opened = await account.call("open_notebook", { notebook: handbook.id });
  expect(opened.isError, opened.text.join(" ")).toBe(false);
  const overview = (opened.structured as Opened).overview;
  expect(opened.structured).toMatchObject({
    notebook: handbook.id,
    instructions: rules,
    instructions_from: "AGENTS.md",
  });
  expect(overview.pages).toBe(3);
  expect(overview.attachments).toBe(1);
  expect(overview.last_changed_at).not.toBeNull();
  expect(overview.root.map((n) => n.path)).toContain("AGENTS.md");

  // 3. Another AGENTS page: the next handshake has its text; with none, the built-in instructions.
  expect((await renameNode(api, team.pat, agentsPage.id, "Old rules")).response.status).toBe(200);
  const replaced = await createPage(api, team.pat, handbook.id, "AGENTS", null, "Second rules.");
  expect((await mcpClient(nervewiki.baseURL, first.token, { notebook: handbook.id })).instructions).toBe(
    "Second rules."
  );
  expect((await deleteNode(api, team.pat, replaced.id)).response.status).toBe(204);
  const now = (await mcpClient(nervewiki.baseURL, first.token, { notebook: handbook.id })).instructions;
  expect(now?.startsWith(builtIn), now).toBe(true);

  // 4. The refusals before the SDK: a session's access token, another team's notebook, a deleted one.
  const gone = await createNotebook(api, team.pat, team.workspace.slug, "Gone");
  expect((await deleteNotebook(api, team.pat, gone.id)).response.status).toBe(204);
  const initialize = {
    jsonrpc: "2.0",
    id: 1,
    method: "initialize",
    params: { protocolVersion: "2025-11-25", capabilities: {}, clientInfo: { name: "e2e-agent", version: "1" } },
  };
  for (const [credential, notebook, status, code] of [
    [session.access_token, handbook.id, 403, "forbidden"],
    [first.token, otherNotebook.id, 404, "notebook.not_found"],
    [first.token, gone.id, 404, "notebook.not_found"],
  ] as const) {
    // oxlint-disable-next-line no-await-in-loop -- one refusal at a time
    const refused = await postMcp(nervewiki.baseURL, credential, initialize, notebook);
    expect(refused.status, `${code} at ${notebook}`).toBe(status);
    expect(refused.headers.get("content-type")).toBe("application/problem+json");
    expect(refused.headers.get("cache-control")).toBe("no-store");
    // oxlint-disable-next-line no-await-in-loop -- its body
    expect(((await refused.json()) as { code: string }).code).toBe(code);
  }

  // 4a. A notebook never written in: nothing counted, no last change, the built-in instructions, no tool error.
  const empty = await createNotebook(api, team.pat, team.workspace.slug, "Empty");
  const openedEmpty = await account.call("open_notebook", { notebook: empty.id });
  expect(openedEmpty.isError, openedEmpty.text.join(" ")).toBe(false);
  const emptied = openedEmpty.structured as Opened;
  expect(emptied.overview).toEqual({ pages: 0, attachments: 0, last_changed_at: null, root: [] });
  expect(emptied.instructions_from).toBeNull();
  expect(emptied.instructions.startsWith(builtIn), emptied.instructions).toBe(true);

  // 5. The official TypeScript client, of the second token: the instructions now, the tools, a list, its name.
  const client = new Client({ name: "e2e-sdk-agent", version: "1.0.0" });
  await client.connect(
    new StreamableHTTPClientTransport(new URL(mcpURL(nervewiki.baseURL, handbook.id)), {
      requestInit: { headers: { Authorization: `Bearer ${second.token}` } },
    })
  );
  expect(client.getInstructions()).toBe(now);
  expect((await client.listTools()).tools.map((t) => t.name).toSorted()).toEqual(
    toolLists.notebook.map((t) => t.name).toSorted()
  );
  const root = await client.callTool({ name: "list", arguments: {} });
  expect(root.isError ?? false, JSON.stringify(root.content)).toBe(false);
  const items = (root.structuredContent as { items: { path: string }[] }).items;
  expect(items.map((i) => i.path)).toEqual(["Old rules.md", "Guide.md", "Notes.md"]);
  await client.close();
  await expectTokenClient(db, second.id, "e2e-sdk-agent");
});
