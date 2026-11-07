import { readFileSync } from "node:fs";
import path from "node:path";

import { mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// LW11, the playbooks (M10 design 4.15; M10/P5 design 5): an agent of an LLM Wiki takes the three prompts through the
// notebook entry, and nothing else does. The story only reads: a prompt writes nothing and runs no model.

const contract = JSON.parse(
  readFileSync(path.resolve(import.meta.dirname, "../../../api/mcp/prompts.json"), "utf8")
) as { notebook: unknown[]; account: unknown[] };

/** The one message's text of a prompt's answer, which must be no error. */
function textOf(answer: Awaited<ReturnType<Awaited<ReturnType<typeof mcpClient>>["getPrompt"]>>): string {
  if ("error" in answer) {
    throw new Error(`the prompt was refused: ${JSON.stringify(answer.error)}`);
  }
  expect(answer.result.messages).toHaveLength(1);
  expect(answer.result.messages[0]?.role).toBe("user");
  return answer.result.messages[0]?.content.text ?? "";
}

test("LW11 (MCP): an agent of an LLM Wiki takes its three playbooks, a standard notebook refuses them, the account entry has none", async ({
  api,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const wiki = await createNotebook(api, pat, workspace.slug, "Field notes", undefined, {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  if (!wiki.source_root_id) {
    throw new Error("the LLM Wiki has no source root");
  }
  await createPage(api, pat, wiki.id, "Paper", wiki.source_root_id, "A source.\n");
  const standard = await createNotebook(api, pat, workspace.slug, "Plain");

  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: wiki.id });
  expect(await agent.listPrompts()).toEqual(contract.notebook);

  const pending = resultOf<{ data: { path: string }[] }>(await agent.call("pending_sources"), "pending_sources");
  const source = pending.data[0]?.path ?? "";
  expect(source).toBe("raw/Paper.md");

  const ingest = textOf(await agent.getPrompt("ingest", { source }));
  expect(ingest.split("\n")[0]).toBe("Ingest a source into this LLM Wiki.");
  expect(ingest.endsWith(`Source path supplied by the caller:\n${source}`)).toBe(true);

  const question = "What does the paper claim?";
  const query = textOf(await agent.getPrompt("query", { question }));
  expect(query.split("\n")[0]).toBe("Answer a question from this LLM Wiki.");
  expect(query.endsWith(`Question supplied by the caller:\n${question}`)).toBe(true);

  expect(textOf(await agent.getPrompt("lint")).split("\n")[0]).toBe("Check and maintain this LLM Wiki.");

  // A standard notebook lists the same three, and refuses them with the code the unknown prompt has not.
  const plain = await mcpClient(nervewiki.baseURL, pat, { notebook: standard.id });
  expect(await plain.listPrompts()).toEqual(contract.notebook);
  expect(await plain.getPrompt("lint")).toEqual({
    error: {
      code: -32602,
      message: "notebook.not_llm_wiki: The notebook is not an LLM Wiki.",
      data: { code: "notebook.not_llm_wiki" },
    },
  });

  // The account entry has no notebook to read them for.
  const account = await mcpClient(nervewiki.baseURL, pat);
  expect(await account.listPrompts()).toEqual(contract.account);
  const unknown = await account.getPrompt("lint");
  if (!("error" in unknown)) {
    throw new Error("the account entry answered a prompt");
  }
  expect(unknown.error.code).toBe(-32602);
  expect(unknown.error.message).toContain("unknown prompt");
  expect(unknown.error.data).toBeUndefined();
});
