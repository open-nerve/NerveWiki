import { accountIdOf } from "../../fixtures/assert/identity";
import { expectRenamed } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, postPage, renameNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  breadcrumbs,
  pageHeading,
  pageTree,
  renameDialog,
  renamePageWith,
  treeTitles,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newTeam } from "../../fixtures/workspaces";

// PG2, titles and renaming (M4 design 3): siblings' titles differ by their
// keys, the title in NFC with Unicode case folding (M4/P5 design 3.7 for the
// page).

test("PG2 (API): an editor renames a page; a title a sibling holds by its key is refused, in any case or normalization; a change of case alone is written", async ({
  api,
  db,
}, testInfo) => {
  const { pat, adminId, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const editorPat = await joinAs(api, pat, workspace.slug, editorEmail, "member");
  const editorId = await accountIdOf(db, editorEmail);
  // The admin creates the pages; a member edits by the workspace access.
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans", "editor");
  const notes = await createPage(api, pat, notebook.id, "Notes");
  const street = await createPage(api, pat, notebook.id, "Straße");
  await createPage(api, pat, notebook.id, "Café");

  const renamed = await renameNode(api, editorPat, notes.id, " Journal ");
  expect(renamed.response.status).toBe(200);
  expect(renamed.data).toMatchObject({ id: notes.id, name: "Journal", parent_id: null });
  await expectRenamed(db, notes.id, "Notes", "Journal", editorId);

  const invalid = await renameNode(api, pat, notes.id, "a:b");
  expect([invalid.response.status, invalid.error?.errors?.map((e) => [e.field, e.code])]).toEqual([
    422,
    [["name", "invalid_format"]],
  ]);
  // Straße and STRASSE fold alike; Café in NFD is Café in NFC.
  const taken = ["STRASSE", "strasse", "Cafe\u0301", "CAFÉ"];
  const refused = await Promise.all(taken.map((name) => renameNode(api, pat, notes.id, name)));
  expect(refused.map((a, i) => [taken[i], a.response.status, a.error?.code])).toEqual(
    taken.map((name) => [name, 409, "page.title_taken"])
  );
  const twin = await postPage(api, pat, notebook.id, { parent_id: null, title: "STRASSE" });
  expect([twin.response.status, twin.error?.code]).toEqual([409, "page.title_taken"]);

  // The page itself holds its key: a change of case alone is written.
  const recased = await renameNode(api, pat, street.id, "STRASSE");
  expect(recased.response.status).toBe(200);
  expect(recased.data?.name).toBe("STRASSE");
  await expectRenamed(db, street.id, "Straße", "STRASSE", adminId);
  await expectRenamed(db, notes.id, "Notes", "Journal", editorId);
});

test("PG2 (page): an editor renames a page in its dialog; a title the rules refuse, or a sibling's in another case, stays in the form; the tree and the shell take the new title", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const page = await signedInPage(await joinOnboarded(api, adminPat, workspace.slug, editorEmail, "member"));
  const editorId = await accountIdOf(db, editorEmail);
  // Open to the workspace to write: the editor writes by the workspace's access.
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Plans", "editor");
  const roadmap = await createPage(api, adminPat, notebook.id, "Roadmap");
  await createPage(api, adminPat, notebook.id, "Notes");
  await page.goto(wikiPagePath(workspace.slug, notebook.id, roadmap.id));
  await expect(pageHeading(page, "Roadmap")).toBeVisible();

  const dialog = renameDialog(page, "Roadmap");
  expect((await renamePageWith(page, "Plans", roadmap.id, "Roadmap", "Q4/2026")).status()).toBe(422);
  await expect(
    dialog.getByText('Cannot contain / \\ : * ? " < > | # ^ [ ] or control characters, nor start or end with a dot.', {
      exact: true,
    })
  ).toBeVisible();
  expect((await renamePageWith(page, "Plans", roadmap.id, "Roadmap", "NOTES")).status()).toBe(409);
  await expect(dialog.getByRole("alert")).toHaveText(
    "A page under the same parent already has this title (titles differ in more than case)."
  );
  pageWatch.expectConsole({ errors: [failedToLoad(422), failedToLoad(409)] });

  expect((await renamePageWith(page, "Plans", roadmap.id, "Roadmap", "  Roadmap 2027 ")).status()).toBe(200);
  await expect(dialog).toBeHidden();
  await expect(pageHeading(page, "Roadmap 2027")).toBeVisible();
  await expect(
    pageTree(page, "Plans").getByRole("button", { name: "Actions for Roadmap 2027", exact: true })
  ).toBeFocused();
  expect(await treeTitles(page, "Plans")).toEqual(["Roadmap 2027", "Notes"]);
  expect(await breadcrumbs(page)).toEqual(["Plans", "Roadmap 2027"]);
  await expectRenamed(db, roadmap.id, "Roadmap", "Roadmap 2027", editorId);
});
