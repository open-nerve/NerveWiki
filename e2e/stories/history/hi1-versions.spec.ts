import { expectVersions } from "../../fixtures/assert/page";
import { displayNameOf } from "../../fixtures/auth";
import { errorOf, mcpClient, readPage, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  endSession,
  getVersion,
  getVersions,
  openSession,
  readContent,
  writeContent,
} from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, startEditing, versionPath, versionsListed, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// HI1, a page's versions (M8 design 4.3; M8/P1 design 4.7): each edit
// session is one version, its saves moving the version's revision on; the
// right column lists them, and a version's page reads one, read only, and
// compares it with the version before it and with the page now. An agent
// lists the versions as REST does, and reads one by any revision it held
// (M9/P5 design 3.6).

/** Tasks' contents: a byte order mark, CRLF and a decomposed é, kept byte for byte. */
const created = "﻿# Café\r\n\r\n- [ ] open\r\n";
const first = "﻿# Café\r\n\r\n- [x] open\r\n";
const second = "﻿# Café\r\n\r\n- [x] open\r\nFirst\r\n";
const third = "﻿# Café\r\n\r\n- [x] open\r\nFirst\r\nSecond \r\n";

/** A version of the list, REST's row as the tool's structuredContent has it (MC8). */
type VersionRow = {
  revision: number;
  base_revision: number | null;
  editing: boolean;
  byte_size: number;
  changeset: { kind: string; client: string; created_by: unknown };
} & Record<string, unknown>;

/** versions' list. */
interface Versions {
  current_version: string;
  versions: VersionRow[];
  next_cursor: string | null;
}

test("HI1 (API): each edit session is one version, which any revision it held addresses, answered with its own; the list is newer first, without content, the newest marked while its session is alive; a version's content is what was written, byte for byte", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const page = await createPage(api, pat, notebook.id, "Tasks", null, created);

  const one = await openSession(api, pat, page.id);
  await writeContent(api, pat, page.id, { content: first, base_revision: 1, edit_session_id: one.id });
  await writeContent(api, pat, page.id, { content: second, base_revision: 2, edit_session_id: one.id });
  expect((await endSession(api, pat, one.id)).response.status).toBe(204);
  const two = await openSession(api, pat, page.id);
  await writeContent(api, pat, page.id, { content: third, base_revision: 3, edit_session_id: two.id });

  const editing = await getVersions(api, pat, page.id);
  expect(editing.response.status, JSON.stringify(editing.error)).toBe(200);
  expect(editing.data?.data.map((version) => [version.revision, version.base_revision, version.editing])).toEqual([
    [4, 3, true],
    [3, 1, false],
    [1, null, false],
  ]);
  expect((await endSession(api, pat, two.id)).response.status).toBe(204);
  const listed = await getVersions(api, pat, page.id);
  expect(listed.data?.next_cursor).toBeNull();
  const by = { user_id: adminId, display_name: displayNameOf(adminEmail) };
  expect(
    listed.data?.data.map(({ revision, base_revision, editing: marked, byte_size, changeset }) => ({
      revision,
      base_revision,
      editing: marked,
      byte_size,
      kind: changeset.kind,
      client: changeset.client,
      by: changeset.created_by,
    }))
  ).toEqual([
    {
      revision: 4,
      base_revision: 3,
      editing: false,
      byte_size: Buffer.byteLength(third),
      kind: "edit",
      client: "api",
      by,
    },
    {
      revision: 3,
      base_revision: 1,
      editing: false,
      byte_size: Buffer.byteLength(second),
      kind: "edit",
      client: "api",
      by,
    },
    {
      revision: 1,
      base_revision: null,
      editing: false,
      byte_size: Buffer.byteLength(created),
      kind: "edit",
      client: "api",
      by,
    },
  ]);
  expect(listed.data?.data.some((version) => "content" in version)).toBe(false);

  const oldest = await getVersion(api, pat, page.id, 1);
  expect([oldest.response.status, oldest.data?.revision, oldest.data?.content]).toEqual([200, 1, created]);
  // Revision 2 was the first session's first save: its version holds it, and answers with its own revision.
  const held = await getVersion(api, pat, page.id, 2);
  expect([held.response.status, held.data?.revision, held.data?.base_revision, held.data?.content]).toEqual([
    200,
    3,
    1,
    second,
  ]);
  expect(held.data?.id).toBe(listed.data?.data[1]?.id);
  const none = await getVersion(api, pat, page.id, 5);
  expect([none.response.status, none.error?.code]).toEqual([404, "page.version_not_found"]);
  expect((await readContent(api, pat, page.id)).content).toBe(third);

  await expectVersions(db, page.id, [
    { base: null, revision: 1, content: created, kind: "edit", byId: adminId, client: "api" },
    { base: 1, revision: 3, content: second, kind: "edit", byId: adminId, client: "api" },
    { base: 3, revision: 4, content: third, kind: "edit", byId: adminId, client: "api" },
  ]);
});

test("HI1 (MCP): an agent lists a page's versions as REST does, each edit session one version, the newest marked while its session is alive; reads a version by any revision it held, answered with its own, byte for byte; its own write is a version of its own", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { adminEmail, adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const page = await createPage(api, pat, notebook.id, "Tasks", null, created);
  const one = await openSession(api, pat, page.id);
  await writeContent(api, pat, page.id, { content: first, base_revision: 1, edit_session_id: one.id });
  await writeContent(api, pat, page.id, { content: second, base_revision: 2, edit_session_id: one.id });
  expect((await endSession(api, pat, one.id)).response.status).toBe(204);
  const two = await openSession(api, pat, page.id);
  await writeContent(api, pat, page.id, { content: third, base_revision: 3, edit_session_id: two.id });
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  const versions = async () => resultOf<Versions>(await agent.call("versions", { path: "Tasks.md" }), "versions");

  const editing = await versions();
  expect(editing.versions.map((v) => [v.revision, v.base_revision, v.editing])).toEqual([
    [4, 3, true],
    [3, 1, false],
    [1, null, false],
  ]);

  // Each row is REST's, which the API version's literal projection reads; the version now is read's.
  expect((await endSession(api, pat, two.id)).response.status).toBe(204);
  const listed = await versions();
  expect(listed.versions).toEqual((await getVersions(api, pat, page.id)).data?.data);
  const by = { user_id: adminId, display_name: displayNameOf(adminEmail) };
  const row = (revision: number, base: number | null, content: string) => ({
    revision,
    base_revision: base,
    editing: false,
    byte_size: Buffer.byteLength(content),
    kind: "edit",
    client: "api",
    by,
  });
  expect(
    listed.versions.map(({ revision, base_revision, editing: marked, byte_size, changeset }) => ({
      revision,
      base_revision,
      editing: marked,
      byte_size,
      kind: changeset.kind,
      client: changeset.client,
      by: changeset.created_by,
    }))
  ).toEqual([row(4, 3, third), row(3, 1, second), row(1, null, created)]);
  expect([listed.next_cursor, listed.versions.some((version) => "content" in version)]).toEqual([null, false]);
  const read = await readPage(agent, { path: "Tasks.md" });
  expect([listed.current_version, read.meta.version]).toEqual([`${page.id}:4`, `${page.id}:4`]);
  expect([read.text === third, (await readContent(api, pat, page.id)).content === third]).toEqual([true, true]);

  // A version by any revision it held, answered with its own revision; none past the last.
  for (const [asked, held, content] of [
    [1, 1, created],
    [2, 3, second],
  ] as const) {
    // oxlint-disable-next-line no-await-in-loop -- one version at a time
    const answer = await agent.call("versions", { path: "Tasks.md", revision: asked });
    expect(answer.isError, answer.text.join(" ")).toBe(false);
    const meta = JSON.parse(answer.text[0] ?? "{}") as { revision: number };
    expect([meta.revision, answer.text[1] === content], `revision ${asked}`).toEqual([held, true]);
  }
  const none = errorOf(await agent.call("versions", { path: "Tasks.md", revision: 5 }), "versions at 5");
  expect(none.code).toBe("page.version_not_found");

  // The agent's write is a version of its own, of its client, no session marking it.
  const fourth = `${third}By the agent.\r\n`;
  const written = resultOf<{ version: string }>(
    await agent.call("write", { path: "Tasks.md", content: fourth, base_version: read.meta.version }),
    "write Tasks"
  );
  expect(written.version).toBe(`${page.id}:5`);
  const after = await versions();
  expect(after.versions.map((v) => [v.revision, v.base_revision, v.editing, v.changeset.client])).toEqual([
    [5, 4, false, "mcp:e2e-agent"],
    [4, 3, false, "api"],
    [3, 1, false, "api"],
    [1, null, false, "api"],
  ]);
  await expectVersions(db, page.id, [
    { base: null, revision: 1, content: created, kind: "edit", byId: adminId, client: "api" },
    { base: 1, revision: 3, content: second, kind: "edit", byId: adminId, client: "api" },
    { base: 3, revision: 4, content: third, kind: "edit", byId: adminId, client: "api" },
    { base: 4, revision: 5, content: fourth, kind: "edit", byId: adminId, client: "mcp:e2e-agent" },
  ]);
});

test("HI1 (page): two edits, the page left between them, are two more versions in the right column; the first, opened, reads its task items without ticking them, and the second compares with the first and with the page now", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { adminEmail, adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const plainCreated = "- [ ] open\n- [x] done\n\nNotes\n";
  const tasks = await createPage(api, pat, notebook.id, "Tasks", null, plainCreated);
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");
  const admin = displayNameOf(adminEmail);

  await page.goto(wikiPagePath(workspace.slug, notebook.id, tasks.id));
  await expect(versionsListed(page)).toHaveCount(1);
  // A writer ticks the page's own task items.
  await expect(main.getByRole("article").getByRole("checkbox", { name: "open", exact: true })).toBeEnabled();
  await startEditing(page);
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("First");
  await main.getByRole("button", { name: "Done", exact: true }).click();
  await expect(main.getByRole("button", { name: "Edit", exact: true })).toBeFocused();
  await startEditing(page);
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type(" Second");
  await main.getByRole("button", { name: "Done", exact: true }).click();
  await expect(main.getByRole("button", { name: "Edit", exact: true })).toBeFocused();

  const listed = versionsListed(page);
  await expect(listed).toHaveCount(3);
  await expect(listed.nth(0)).toContainText(`Current${admin} · Edit`);
  await expect(listed.nth(1)).toContainText(`${admin} · Edit`);
  await expect(listed.nth(2)).toContainText(`${admin} · Edit`);
  const plainSecond = `${plainCreated}First`;
  const plainThird = `${plainCreated}First Second`;
  expect(await readContent(api, pat, tasks.id)).toMatchObject({ content: plainThird, revision: 3 });

  await listed.nth(2).getByRole("link").click();
  await expect(pageHeading(page, "Version 1")).toBeFocused();
  await expect(page).toHaveTitle("Version 1 · Tasks · Plans · Nerve Wiki");
  const read = main.getByRole("article", { name: "Version 1", exact: true });
  await expect(read.getByRole("checkbox")).toHaveCount(2);
  await expect(read.getByRole("checkbox").first()).toBeDisabled();
  await expect(read.getByRole("checkbox").nth(1)).toBeDisabled();
  await expect(main.getByRole("tab")).toHaveText(["Read", "Compare with the current"]);

  await page.goto(versionPath(workspace.slug, notebook.id, tasks.id, 2));
  await expect(main.getByRole("article", { name: "Version 2", exact: true })).toContainText("First");
  await main.getByRole("tab", { name: "Compare with the previous", exact: true }).click();
  await expect(main.getByRole("tabpanel")).toContainText("Version 1 → Version 2");
  // What the version added to the one before it: its last line, empty there (the text ended in a line break).
  const previous = main.getByRole("textbox", { name: "Changes from version 1 to version 2", exact: true });
  await expect(previous.locator(".cm-changedLine")).toHaveText(["First"]);
  await expect(previous.locator(".cm-deletedLine")).toHaveText([""]);
  await main.getByRole("tab", { name: "Compare with the current", exact: true }).click();
  await expect(main.getByRole("tabpanel")).toContainText("The page now → Version 2");
  // What a restore would take away: the page now's line, struck out, for the version's.
  const current = main.getByRole("textbox", { name: "Changes from the page now to version 2", exact: true });
  await expect(current.locator(".cm-deletedLine")).toHaveText(["First Second"]);
  await expect(current.locator(".cm-changedLine")).toHaveText(["First"]);
  // Not in the address, the tab chosen: a reload reads the version again.
  expect(new URL(page.url()).pathname).toBe(versionPath(workspace.slug, notebook.id, tasks.id, 2));

  await expectVersions(db, tasks.id, [
    { base: null, revision: 1, content: plainCreated, kind: "edit", byId: adminId, client: "api" },
    { base: 1, revision: 2, content: plainSecond, kind: "edit", byId: adminId, client: "web" },
    { base: 2, revision: 3, content: plainThird, kind: "edit", byId: adminId, client: "web" },
  ]);
});

// A version's links lead where the notebook resolves them now (M8 design 4.13; M8 Codex review R3): its target's alias
// dropped, a link by it leads nowhere in a version's view shown, without a reload, whether the page's text now holds the
// link too, the page the links event names, or the version alone does, which the event reaches by telling that the
// aliases changed.
for (const now of [false, true]) {
  test(`HI1 (page): a version's link by an alias leads nowhere once the alias is dropped, without a reload; ${now ? "the page now links by it too" : "the version alone links by it"}`, async ({
    api,
    signedInPage,
    anotherTab,
  }, testInfo) => {
    const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
    const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
    const target = await createPage(api, pat, notebook.id, "Target", null, "---\naliases: [OldAlias]\n---\nInitial\n");
    const source = await createPage(api, pat, notebook.id, "Source", null, "[[OldAlias]]\n");
    await writeContent(api, pat, source.id, {
      content: now ? "[[OldAlias]]\nNow\n" : "No links now\n",
      base_revision: 1,
    });
    const page = await signedInPage(tokens);

    await page.goto(versionPath(workspace.slug, notebook.id, source.id, 1));
    const version = page.getByRole("main").getByRole("article", { name: "Version 1", exact: true });
    await expect(version.getByRole("link", { name: "OldAlias", exact: true })).toHaveAttribute(
      "href",
      wikiPagePath(workspace.slug, notebook.id, target.id)
    );
    // Another tab shows Target: once it shows the write, the write's events have come.
    const other = await anotherTab(page);
    await other.goto(wikiPagePath(workspace.slug, notebook.id, target.id));
    const shown = other.getByRole("main").getByRole("article", { name: "Target", exact: true });
    await expect(shown).toContainText("Initial");
    await writeContent(api, pat, target.id, { content: "Alias removed\n", base_revision: 1 });
    await expect(shown).toContainText("Alias removed");
    await expect(version.getByRole("button", { name: "OldAlias", exact: true })).toBeVisible();
    await expect(version.getByRole("link", { name: "OldAlias", exact: true })).toHaveCount(0);
  });
}
