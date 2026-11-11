import { readFileSync } from "node:fs";
import path from "node:path";

import { mcpClient, promptTextOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { startUpgrade } from "../../fixtures/upgrades";
import { newTeam } from "../../fixtures/workspaces";

// UP9, the upgrade's playbook (M11/P3 design 5): the agent of the admin who started a notebook's upgrade lists the
// notebook entry's prompts, the contract's, upgrade among them, and takes upgrade: the playbook, its first line the
// imperative one. A standard notebook without an upgrade refuses it with wiki.upgrade_state. Only MCP tells this:
// the story writes nothing through it and reads no database (M11/P3 design 10.1's exception).

const contract = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, "../../../api/mcp/prompts.json"), "utf8")
) as { notebook: { name: string; description?: string }[] };

/** The upgrade playbook as the server embeds it, its white space around it trimmed. */
const playbook = readFileSync(
  path.resolve(import.meta.dirname, "../../../server/internal/modules/wiki/domain/playbook-upgrade.md"),
  "utf8"
).trim();

test("UP9 (MCP): the agent of the admin who started an upgrade takes the upgrade playbook, a notebook not upgrading refuses it", async ({
  api,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const created = await createNotebook(api, pat, workspace.slug, "Field notes");
  await createPage(api, pat, created.id, "Note", null, "# Note\n");
  const upgrade = await startUpgrade(api, pat, created.id);
  expect([upgrade.notebook_id, upgrade.state]).toEqual([created.id, "upgrading"]);
  const standard = await createNotebook(api, pat, workspace.slug, "Plain");

  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: upgrade.notebook_id });
  const listed = await agent.listPrompts();
  expect(listed).toEqual(contract.notebook);
  const entry = contract.notebook.find((p) => p.name === "upgrade");
  expect(entry, "api/mcp/prompts.json lists upgrade").toBeDefined();
  expect(listed.find((p) => p.name === "upgrade")).toEqual(entry);

  const answer = await agent.getPrompt("upgrade");
  const text = promptTextOf(answer);
  expect(text.split("\n")[0]).toBe("Upgrade this notebook to an LLM Wiki.");
  expect(text).toBe(playbook);
  expect("result" in answer && answer.result.description).toBe(entry?.description);

  // A notebook with no upgrade under way: the same code a client reads in data, as LW11's refusal has it.
  const plain = await mcpClient(nervewiki.baseURL, pat, { notebook: standard.id });
  expect(await plain.getPrompt("upgrade")).toEqual({
    error: {
      code: -32602,
      message: "wiki.upgrade_state: The upgrade is not in a state that allows this.",
      data: { code: "wiki.upgrade_state" },
    },
  });
});
