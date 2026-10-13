import { accountIdOf } from "../../fixtures/assert/identity";
import { expectIndexedLinks } from "../../fixtures/assert/links";
import { expectVersions, type StoredVersion } from "../../fixtures/assert/page";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, endSession, openSession, postRestore, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  pageHeading,
  restoreDialog,
  restoreWith,
  startEditing,
  versionPath,
  versionsListed,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// HI2, a version restored (M8 design 4.4; M8/P1 design 4.5, 4.7): its
// content becomes the page's, a version of its own in a version_restore
// changeset, the link index following; a content the page holds already
// writes nothing; a page written since it was read, or edited, refuses it.

test("HI2 (API): a version restored on the page's revision writes its content as a new version, a version_restore of its own, and the index follows; restored again it writes nothing; on a revision passed it is 409 page.revision_mismatch, and while another edits, 409 page.locked naming them", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, pat, workspace.slug, bEmail, "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans", "editor");
  const alpha = await createPage(api, pat, notebook.id, "Alpha");
  const beta = await createPage(api, pat, notebook.id, "Beta");
  const first = "See [[Alpha]].\r\n";
  const second = "See [[Beta]].\n";
  const notes = await createPage(api, pat, notebook.id, "Notes", null, first);
  await writeContent(api, pat, notes.id, { content: second, base_revision: 1 });
  await expectIndexedLinks(db, notes.id, [{ kind: "wikilink", property: null, target: "Beta", resolved: beta.id }]);

  const restored = await postRestore(api, pat, notes.id, 1, 2);
  expect(restored.response.status, JSON.stringify(restored.error)).toBe(200);
  expect(restored.data).toMatchObject({ revision: 3, changed: true });
  expect(restored.data?.changeset_id).toMatch(/^[0-9a-f-]{36}$/);
  expect(await readContent(api, pat, notes.id)).toMatchObject({ content: first, revision: 3 });
  await expectIndexedLinks(db, notes.id, [{ kind: "wikilink", property: null, target: "Alpha", resolved: alpha.id }]);
  const versions: StoredVersion[] = [
    { base: null, revision: 1, content: first, kind: "edit", byId: adminId, client: "api" },
    { base: 1, revision: 2, content: second, kind: "edit", byId: adminId, client: "api" },
    { base: 2, revision: 3, content: first, kind: "version_restore", byId: adminId, client: "api" },
  ];
  await expectVersions(db, notes.id, versions);

  const again = await postRestore(api, pat, notes.id, 1, 3);
  expect([again.response.status, again.data]).toEqual([200, { revision: 3, changeset_id: null, changed: false }]);
  const passed = await postRestore(api, pat, notes.id, 2, 2);
  expect([passed.response.status, passed.error?.code]).toEqual([409, "page.revision_mismatch"]);
  const session = await openSession(api, b, notes.id);
  const locked = await postRestore(api, pat, notes.id, 2, 3);
  expect([locked.response.status, locked.error?.code, locked.error?.lock]).toEqual([
    409,
    "page.locked",
    { page_id: notes.id, user_id: await accountIdOf(db, bEmail), display_name: displayNameOf(bEmail) },
  ]);
  expect((await endSession(api, b, session.id)).response.status).toBe(204);
  expect(await readContent(api, pat, notes.id)).toMatchObject({ content: first, revision: 3 });
  await expectVersions(db, notes.id, versions);
});

test("HI2 (page): A restores an older version: the page shows it, its title focused, saying so, its versions one more, a Restore, and the index follows; restored again, the version's page says nothing changed; while B edits, the restore is refused naming B, and nothing is written", async ({
  anotherPage,
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { adminEmail, adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await anotherPage(await joinOnboarded(api, pat, workspace.slug, bEmail, "member"));
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans", "editor");
  const alpha = await createPage(api, pat, notebook.id, "Alpha");
  const beta = await createPage(api, pat, notebook.id, "Beta");
  const first = "See [[Alpha]].\n";
  const second = "See [[Beta]].\n";
  const notes = await createPage(api, pat, notebook.id, "Notes", null, first);
  await writeContent(api, pat, notes.id, { content: second, base_revision: 1 });
  await expectIndexedLinks(db, notes.id, [{ kind: "wikilink", property: null, target: "Beta", resolved: beta.id }]);
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");

  await page.goto(versionPath(workspace.slug, notebook.id, notes.id, 1));
  await expect(main.getByRole("article", { name: "Version 1", exact: true })).toContainText("See Alpha.");
  await main.getByRole("button", { name: "Restore this version", exact: true }).click();
  const dialog = restoreDialog(page, 1);
  await expect(dialog).toContainText("The page now → Version 1");
  // What the restore changes: the page now's line, struck out, for the version's.
  const changes = dialog.getByRole("textbox", { name: "Changes from the page now to version 1", exact: true });
  await expect(changes.locator(".cm-deletedLine")).toHaveText(["See [[Beta]]."]);
  await expect(changes.locator(".cm-changedLine")).toHaveText(["See [[Alpha]]."]);
  const restored = await restoreWith(page, notes.id, 1);
  expect([restored.status(), await restored.json()]).toEqual([
    200,
    expect.objectContaining({ revision: 3, changed: true }),
  ]);
  await expect(pageHeading(page, "Notes")).toBeFocused();
  expect(new URL(page.url()).pathname).toBe(wikiPagePath(workspace.slug, notebook.id, notes.id));
  await expect(main.getByRole("status").filter({ hasText: "Restored version 1." })).toBeVisible();
  await expect(main.getByRole("article", { name: "Notes", exact: true }).getByRole("link")).toHaveText("Alpha");
  const listed = versionsListed(page);
  await expect(listed).toHaveCount(3);
  await expect(listed.first()).toContainText(`Current${displayNameOf(adminEmail)} · Restore`);
  expect(await readContent(api, pat, notes.id)).toMatchObject({ content: first, revision: 3 });
  await expectIndexedLinks(db, notes.id, [{ kind: "wikilink", property: null, target: "Alpha", resolved: alpha.id }]);
  const versions: StoredVersion[] = [
    { base: null, revision: 1, content: first, kind: "edit", byId: adminId, client: "api" },
    { base: 1, revision: 2, content: second, kind: "edit", byId: adminId, client: "api" },
    { base: 2, revision: 3, content: first, kind: "version_restore", byId: adminId, client: "web" },
  ];
  await expectVersions(db, notes.id, versions);

  await page.goto(versionPath(workspace.slug, notebook.id, notes.id, 1));
  const unchanged = await restoreWith(page, notes.id, 1);
  expect([unchanged.status(), await unchanged.json()]).toEqual([
    200,
    { revision: 3, changeset_id: null, changed: false },
  ]);
  await expect(dialog).toBeHidden();
  await expect(main.getByText("The page is the same as this version: nothing changed.", { exact: true })).toBeVisible();
  await expect(main.getByRole("button", { name: "Restore this version", exact: true })).toBeFocused();

  await b.goto(wikiPagePath(workspace.slug, notebook.id, notes.id));
  await startEditing(b);
  await page.goto(versionPath(workspace.slug, notebook.id, notes.id, 2));
  const bEditing = displayNameOf(bEmail);
  await expect(main.getByText(`${bEditing} is editing this page.`, { exact: true })).toBeVisible();
  const refused = await restoreWith(page, notes.id, 2);
  expect([refused.status(), ((await refused.json()) as { code: string }).code]).toEqual([409, "page.locked"]);
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  await expect(restoreDialog(page, 2).getByRole("alert")).toHaveText(`${bEditing} is editing “Notes”.`);
  await restoreDialog(page, 2).getByRole("button", { name: "Cancel", exact: true }).click();
  await b.getByRole("main").getByRole("button", { name: "Done", exact: true }).click();
  await expect(b.getByRole("main").getByRole("button", { name: "Edit", exact: true })).toBeFocused();
  expect(await readContent(api, pat, notes.id)).toMatchObject({ content: first, revision: 3 });
  await expectVersions(db, notes.id, versions);
  await expectIndexedLinks(db, notes.id, [{ kind: "wikilink", property: null, target: "Alpha", resolved: alpha.id }]);
});
