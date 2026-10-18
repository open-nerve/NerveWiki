import { expectChangesetOf, expectNewPage, expectVersions } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { memberOf, removeMember } from "../../fixtures/members";
import { createNotebook } from "../../fixtures/notebooks";
import { ownerlessNotebooks } from "../../fixtures/ownerless";
import { createPage, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { historyListed, historyPath, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// MC3, an agent writes and edits pages over MCP (M9 design 9; M9/P2 design
// 4.2): a page created with its missing parents in one changeset of the
// agent's client, a write refused or writing nothing, a write on a version
// that the reading view shows at once, edits once and every time, CRLF
// kept, the name the token recorded without a handshake, and the history
// of the agent's changesets; an ownerless notebook's activity of an
// agent's writes.

/** A write's answer, as the tool's structuredContent has it. */
interface Written {
  path: string;
  node_id: string;
  version: string;
  changed: boolean;
  created: { path: string; node_id: string }[];
  created_count: number;
  changeset_id: string | null;
  at: string | null;
}

/** An edit's answer. */
interface Edited {
  version: string;
  replacements: number;
  changed: boolean;
  changeset_id: string | null;
}

/** Plan's first content: one "ship" and two task boxes, which steps 7 and 8 edit. */
const plan = "# Plan\n\n- [ ] ship\n- [ ] tell\n";

/** Plan's second content, written on its first version. */
const updated = `${plan}\nUpdated by the agent.\n`;

/** The client of the agent's changesets: the scripted client's name (fixtures/mcp.ts). */
const client = "mcp:e2e-agent";

test("MC3 (MCP): an agent writes a page with its missing parents in one changeset, refused another content without a version and writing nothing for the same; writes on the version read, which the page shows at once, refused on an old one; edits a string once and every time, a CRLF page with LF; its token's name stays its client without a handshake; the history lists its changesets", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Handbook");
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  const path = "Projects/Q3/Plan.md";

  const first = resultOf<Written>(await agent.call("write", { path, content: plan }), "write Plan");
  expect([first.changed, first.created.map((c) => c.path), first.created_count]).toEqual([
    true,
    ["Projects.md", "Projects/Q3.md", "Projects/Q3/Plan.md"],
    3,
  ]);
  const planId = first.node_id;
  expect(first.version).toBe(`${planId}:1`);
  // Each created as the answer tells it: its name the path's last segment, its parent the node before it.
  await Promise.all(
    first.created.map((node, i) =>
      expectNewPage(
        db,
        {
          id: node.node_id,
          notebook_id: notebook.id,
          parent_id: i === 0 ? null : (first.created[i - 1]?.node_id ?? null),
          name: (node.path.split("/").at(-1) ?? "").replace(/\.md$/, ""),
          created_at: String(first.at),
        },
        adminId,
        client,
        node.node_id === planId ? plan : ""
      )
    )
  );
  const ids = first.created.map((c) => c.node_id);
  await expectChangesetOf(db, String(first.changeset_id), { client, byId: adminId, items: ids, versions: ids });

  const taken = errorOf(await agent.call("write", { path, content: "# Other\n" }), "write another content");
  expect(taken.code).toBe("page.title_taken");
  expect(taken.message).toContain("is a page already");
  const same = resultOf<Written>(await agent.call("write", { path, content: plan }), "write the same content");
  expect([same.changed, same.changeset_id, same.version]).toEqual([false, null, first.version]);
  // Plan's first version is in the changeset that created its parents too: three versions.
  const firstVersion = { base: null, revision: 1, content: plan, kind: "edit", byId: adminId, client, versions: 3 };
  await expectVersions(db, planId, [firstVersion]);

  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, planId));
  await expect(page.getByRole("article")).toContainText("ship");
  const second = resultOf<Written>(
    await agent.call("write", { path, content: updated, base_version: first.version }),
    "write on the first version"
  );
  expect([second.version, second.changed]).toEqual([`${planId}:2`, true]);
  await expect(page.getByRole("article")).toContainText("Updated by the agent.");
  const stale = errorOf(
    await agent.call("write", { path, content: "# Stale\n", base_version: first.version }),
    "write on the first version again"
  );
  expect([stale.code, stale.current_version]).toEqual(["page.revision_mismatch", `${planId}:2`]);
  expect(await readContent(api, pat, planId)).toMatchObject({ content: updated, revision: 2 });

  const shipped = resultOf<Edited>(
    await agent.call("edit", { path, old_string: "ship", new_string: "ship it" }),
    "edit ship"
  );
  expect([shipped.replacements, shipped.version]).toEqual([1, `${planId}:3`]);
  const boxes = errorOf(
    await agent.call("edit", { path, old_string: "- [ ]", new_string: "- [x]" }),
    "edit a task box"
  );
  expect(boxes.code).toBe("validation_failed");
  expect(boxes.message).toContain("found 2 times");
  const all = resultOf<Edited>(
    await agent.call("edit", { path, old_string: "- [ ]", new_string: "- [x]", replace_all: true }),
    "edit every task box"
  );
  expect([all.replacements, all.version]).toEqual([2, `${planId}:4`]);
  const notes = await createPage(api, pat, notebook.id, "Notes", null, "a\r\nb\r\n");
  resultOf<Edited>(
    await agent.call("edit", { path: "Notes.md", old_string: "a\nb", new_string: "a\nc" }),
    "edit Notes with LF"
  );
  expect((await readContent(api, pat, notes.id)).content).toBe("a\r\nc\r\n");

  const later = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id, handshake: false });
  const late = resultOf<Written>(
    await later.call("write", { path: "Later.md", content: "Later.\n" }),
    "write without a handshake"
  );
  await expectChangesetOf(db, String(late.changeset_id), {
    client,
    byId: adminId,
    items: [late.node_id],
    versions: [late.node_id],
  });

  await page.goto(historyPath(workspace.slug, notebook.id, { client: "mcp" }));
  // The agent's six: Plan created, written, edited twice; Notes edited; Later created. Notes' creation is REST's.
  const listed = historyListed(page);
  await expect(listed).toHaveCount(6);
  await expect(listed).toContainText(Array.from({ length: 6 }, () => "MCP (e2e-agent) · "));
  await expectVersions(db, planId, [
    firstVersion,
    { base: 1, revision: 2, content: updated, kind: "edit", byId: adminId, client },
    { base: 2, revision: 3, content: updated.replace("ship", "ship it"), kind: "edit", byId: adminId, client },
    {
      base: 3,
      revision: 4,
      content: updated.replace("ship", "ship it").replaceAll("- [ ]", "- [x]"),
      kind: "edit",
      byId: adminId,
      client,
    },
  ]);
});

test("MC3 (activity): an ownerless notebook's size is the bytes of the pages an agent wrote, and its last activity the agent's last write", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Plans");
  const agent = await mcpClient(nervewiki.baseURL, ownerPat, { notebook: notebook.id });
  const intro = "Written by the agent.\n";
  const content = "# Notes\n\nWritten again.\n";
  resultOf<Written>(await agent.call("write", { path: "Intro.md", content: intro }), "write Intro");
  const notes = resultOf<Written>(await agent.call("write", { path: "Notes.md", content: "# Notes\n" }), "write Notes");
  const last = resultOf<Written>(
    await agent.call("write", { path: "Notes.md", content, base_version: notes.version }),
    "write Notes again"
  );
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, workspace.slug, ownerEmail)).id)).response.status
  ).toBe(204);

  const listed = await ownerlessNotebooks(api, adminPat, workspace.slug);
  expect(listed.map((l) => [l.id, l.size_bytes])).toEqual([
    [notebook.id, Buffer.byteLength(intro) + Buffer.byteLength(content)],
  ]);
  const [at] = await db.query<{ last: boolean }>("SELECT $1::timestamptz = $2::timestamptz AS last", [
    listed[0]?.last_activity_at,
    last.at,
  ]);
  expect(at?.last).toBe(true);
});
