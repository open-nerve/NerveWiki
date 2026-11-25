import { expectChangesetOf, expectNewPage, expectVersions } from "../../fixtures/assert/page";
import { bearer } from "../../fixtures/auth";
import { revertChangeset } from "../../fixtures/history";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { expectFits, phoneWidth } from "../../fixtures/narrow";
import { goto } from "../../fixtures/navigation";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { changesetPath, historyListed, historyPath, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// MC3, an agent writes and edits pages over MCP (M9 design 9; M9/P2 design
// 4.2): a page created with its missing parents in one changeset of the
// agent's client, a write refused or writing nothing, a write on a version
// that the reading view shows at once, edits once and every time, CRLF
// kept, the name the token recorded without a handshake, and the history
// of the agent's changesets.

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
  await goto(page, wikiPagePath(workspace.slug, notebook.id, planId));
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

  await goto(page, historyPath(workspace.slug, notebook.id, { client: "mcp" }));
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

// At a phone's width (M12/P2/S3; M5-polish 26.17, 32.1–32.3, 35.8, 38.12): an MCP client's name of 128 characters
// and an author's of 100, neither with a space, widen no row, filter or line of the history and a changeset's page,
// in Chromium and in WebKit (@narrow). Only the second write is undone: undoing a creation puts its page in the trash,
// and the page filter would name "a deleted page" (M12/P2 design review B-M1).
test(
  "MC3 (page): at 375 wide, a client's name of 128 characters and an author's of 100, neither with a space, keep the history, its filters and a changeset's page to the screen",
  { tag: "@narrow" },
  async ({ api, nervewiki, signedInPage }, testInfo) => {
    const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
    const author = `ada_${"a".repeat(96)}`;
    const agentName = `agent.${"c".repeat(122)}`;
    const title = `report_${"r".repeat(113)}`;
    const other = `other_${"o".repeat(114)}`;
    const renamed = `renamed_${"n".repeat(112)}`;
    expect(
      (await api.PATCH("/api/v0/me", { body: { display_name: author }, headers: bearer(pat) })).response.status
    ).toBe(200);
    const notebook = await createNotebook(api, pat, workspace.slug, "Handbook");
    const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id, client: agentName });
    const wrote = resultOf<Written>(await agent.call("write", { path: `${title}.md`, content: "Long.\n" }), "write");
    const second = resultOf<Written>(await agent.call("write", { path: `${other}.md`, content: "Other.\n" }), "write");
    expect((await revertChangeset(api, pat, String(second.changeset_id))).response.status).toBe(200);
    const page = await signedInPage(tokens);
    const main = page.getByRole("main");
    await page.setViewportSize({ width: phoneWidth, height: 800 });
    const breadcrumb = main.getByRole("navigation", { name: "Breadcrumb" });
    const changed = main.getByRole("list", { name: "Nodes of this change" });

    // The history: its filters (Who lists the author, Client the agent) and its rows.
    await goto(page, historyPath(workspace.slug, notebook.id));
    const listed = historyListed(page);
    await expect(listed).toHaveCount(3);
    // On a narrow screen a row's Only this person shows at rest (M13/P5/S2, decision 927: max-md).
    await expect(listed.first().getByRole("button", { name: /^Only this person/ })).toHaveCSS("opacity", "1");
    const wrapper = (label: string) => main.getByLabel(label, { exact: true }).locator("xpath=..");
    await expectFits(
      page,
      wrapper("Who"),
      wrapper("Client"),
      wrapper("Ingestion"),
      ...[0, 1, 2].map((n) => listed.nth(n))
    );

    // Filtered by the long title's page.
    await goto(page, historyPath(workspace.slug, notebook.id, { page: wrote.node_id }));
    const filtered = main.getByText(`Page: ${title}`, { exact: true });
    await expect(filtered).toBeVisible();
    await expectFits(page, filtered.locator("xpath=.."));

    // The write's page: its client, its breadcrumbs (who made it, when) and its node's link.
    await goto(page, changesetPath(workspace.slug, notebook.id, String(wrote.changeset_id)));
    const clientLine = main.getByText(`Client: MCP (${agentName})`, { exact: true });
    await expect(clientLine).toBeVisible();
    await expectFits(page, clientLine, breadcrumb, changed.getByRole("link", { name: title, exact: true }));

    // The undone write's: its node, in the trash, by name.
    await goto(page, changesetPath(workspace.slug, notebook.id, String(second.changeset_id)));
    const gone = changed.getByText(other, { exact: true });
    await expect(gone).toBeVisible();
    await expectFits(page, gone, breadcrumb);

    // A rename's: "Renamed: “…” → “…”".
    const moved = resultOf<Written>(await agent.call("move", { from: `${title}.md`, to: `${renamed}.md` }), "rename");
    await goto(page, changesetPath(workspace.slug, notebook.id, String(moved.changeset_id)));
    const said = main.getByText(`Renamed: “${title}” → “${renamed}”`, { exact: true });
    await expect(said).toBeVisible();
    await expectFits(page, said);
  }
);
