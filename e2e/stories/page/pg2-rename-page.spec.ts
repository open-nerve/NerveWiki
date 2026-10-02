import { expectRenamed } from "../../fixtures/assert/page";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, postPage, renameNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// PG2, titles and renaming (M4 design 3): siblings' titles differ by their
// keys, the title in NFC with Unicode case folding; the page version comes
// with the tree (M4/P5).

test("PG2 (API): an editor renames a page; a title a sibling holds by its key is refused, in any case or normalization; a change of case alone is written", async ({
  api,
  db,
}, testInfo) => {
  const { pat, adminId, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const notes = await createPage(api, pat, notebook.id, "Notes");
  const street = await createPage(api, pat, notebook.id, "Straße");
  await createPage(api, pat, notebook.id, "Café");

  const renamed = await renameNode(api, pat, notes.id, " Journal ");
  expect(renamed.response.status).toBe(200);
  expect(renamed.data).toMatchObject({ id: notes.id, name: "Journal", parent_id: null });
  await expectRenamed(db, notes.id, "Notes", "Journal", adminId);

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
  await expectRenamed(db, notes.id, "Notes", "Journal", adminId);
});
