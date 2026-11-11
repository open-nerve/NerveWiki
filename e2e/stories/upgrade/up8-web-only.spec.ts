import { expectUpgrade } from "../../fixtures/assert/wiki";
import { createNotebook, getNotebook } from "../../fixtures/notebooks";
import { listNodes } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  approveUpgrade,
  checkUpgrade,
  getUpgrade,
  postApprove,
  startUpgrade,
  submitUpgrade,
} from "../../fixtures/upgrades";
import { newTeam } from "../../fixtures/workspaces";

// UP8, an upgrade carried through without an agent (M11/P3 design 5): an
// empty standard notebook's admin starts its upgrade, which writes raw and
// AGENTS, checks it, submits it and approves it, each answering as it
// should, the approval refused before the submission; approved, the
// notebook is an LLM Wiki whose root holds what the start wrote. Through
// the API; the page version is S3's. No MCP version: the story is the one
// without an agent, and UP7 covers MCP after an approval.

test("UP8 (API): an empty notebook's admin starts, checks, submits and approves its upgrade with no agent; the notebook is an LLM Wiki whose root holds the raw and AGENTS the start wrote", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");

  const started = await startUpgrade(api, pat, notebook.id);
  expect([started.state, started.seeded_page_ids.length, started.baseline.nodes]).toEqual(["upgrading", 2, 0]);
  const early = await postApprove(api, pat, notebook.id);
  expect([early.response.status, early.error?.code]).toEqual([409, "wiki.upgrade_state"]);
  const check = await checkUpgrade(api, pat, notebook.id);
  expect(check.passed).toBe(true);
  const submitted = await submitUpgrade(api, pat, notebook.id);
  expect([submitted.state, submitted.report?.passed]).toEqual(["reviewing", true]);
  const approved = await approveUpgrade(api, pat, notebook.id);
  expect(approved).toEqual({ ...submitted, state: "approved" });
  await expectUpgrade(db, approved);

  const read = await getUpgrade(api, pat, notebook.id);
  expect([read.response.status, read.error?.code]).toEqual([404, "wiki.upgrade_not_found"]);
  const after = await getNotebook(api, pat, notebook.id);
  expect([after.data?.mode, after.data?.upgrade]).toEqual(["llm_wiki", null]);
  const nodes = await listNodes(api, pat, notebook.id);
  expect(nodes.map((node) => [node.name, node.parent_id, node.id]).toSorted()).toEqual(
    [
      ["AGENTS", null, started.seeded_page_ids[1]],
      ["raw", null, started.source_root_id],
    ].toSorted()
  );
});
