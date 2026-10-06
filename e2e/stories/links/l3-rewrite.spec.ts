import { accountIdOf } from "../../fixtures/assert/identity";
import { expectIndexedLinks } from "../../fixtures/assert/links";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, endSession, getPage, listNodes, openSession, readContent, renameNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  dragPage,
  pageHeading,
  pageTree,
  renameDialog,
  renamePageWith,
  treeTitles,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// L3, a rename that writes links again (M6 design 9; M6/P4 design 4, 6): a
// rename writes the links to the page on the other pages again, and each
// leads to it still; one whose rewrite reaches a page someone edits is
// refused whole, the dialog, or the tree for a drag, naming the page and
// its editor, and nothing changes until the editor is done.

/** What linking.pages_locked says above the list of the pages it names. */
const writesAgain =
  "This change would write the links on these pages again, and they are being edited. Try again once their editors are done; a notebook admin can also release a lock.";

test("L3 (API): a rename writes the links to the page again, each as the renamer; one whose rewrite reaches a page another edits is 409 linking.pages_locked naming it and its editor, and renames nothing", async ({
  api,
  db,
}, testInfo) => {
  const { pat: admin, workspace } = await newTeam(api, testInfo);
  const aEmail = emailFor(testInfo, "a");
  const a = await joinAs(api, admin, workspace.slug, aEmail, "member");
  const notebook = await createNotebook(api, admin, workspace.slug, "Plans", "editor");
  const target = await createPage(api, admin, notebook.id, "Target");
  const source = await createPage(api, admin, notebook.id, "Source", null, "[[Target]] and [md](Target.md)\n");
  const session = await openSession(api, a, source.id);

  const refused = await renameNode(api, admin, target.id, "Renamed");
  expect([refused.response.status, refused.error?.code, refused.error?.locks]).toEqual([
    409,
    "linking.pages_locked",
    [{ page_id: source.id, user_id: expect.any(String), display_name: displayNameOf(aEmail) }],
  ]);
  expect((await listNodes(api, admin, notebook.id)).map((node) => node.name).toSorted()).toEqual(["Source", "Target"]);
  expect((await readContent(api, admin, source.id)).revision).toBe(1);

  expect((await endSession(api, a, session.id)).response.status).toBe(204);
  expect((await renameNode(api, a, target.id, "Renamed")).response.status).toBe(200);
  const written = await readContent(api, admin, source.id);
  expect([written.content, written.revision]).toEqual(["[[Renamed]] and [md](Renamed.md)\n", 2]);
  expect((await getPage(api, admin, source.id)).data?.content_updated_by).toBe(await accountIdOf(db, aEmail));
  await expectIndexedLinks(db, source.id, [
    { kind: "wikilink", property: null, target: "Renamed", resolved: target.id },
    { kind: "link", property: null, target: "Renamed.md", resolved: target.id },
  ]);
});

test("L3 (page): a rename in the dialog, and a drag, whose links' page another edits say who edits which page and change nothing; once the editor is done, the rename goes through and the link leads to the page", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat: admin, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const aEmail = emailFor(testInfo, "a");
  const a = await joinAs(api, admin, workspace.slug, aEmail, "member");
  const notebook = await createNotebook(api, admin, workspace.slug, "Plans", "editor");
  const target = await createPage(api, admin, notebook.id, "Target");
  await createPage(api, admin, notebook.id, "Box");
  // A link from the root: once Target moves under Box, it would lead nowhere.
  const source = await createPage(api, admin, notebook.id, "Source", null, "[[Target]] and [rooted](/Target.md)\n");
  const session = await openSession(api, a, source.id);
  const editing = `${displayNameOf(aEmail)} is editing “Source”.`;
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  await expect(pageHeading(page, "Source")).toBeVisible();

  const refused = await renamePageWith(page, "Plans", target.id, "Target", "Renamed");
  expect(refused.status()).toBe(409);
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  const alert = renameDialog(page, "Target").getByRole("alert");
  await expect(alert.getByRole("paragraph")).toHaveText(writesAgain);
  await expect(alert.getByRole("listitem")).toHaveText([editing]);
  await page.keyboard.press("Escape");
  await expect(renameDialog(page, "Target")).toBeHidden();

  await dragPage(page, "Plans", "Target", "Box", "into");
  const treeAlert = pageTree(page, "Plans").getByRole("alert");
  await expect(treeAlert.getByRole("listitem")).toHaveText([editing]);
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  expect(await treeTitles(page, "Plans")).toEqual(["Target", "Box", "Source"]);
  expect((await readContent(api, admin, source.id)).revision).toBe(1);

  expect((await endSession(api, a, session.id)).response.status).toBe(204);
  const renamed = await renamePageWith(page, "Plans", target.id, "Target", "Renamed");
  expect(renamed.status()).toBe(200);
  await expect(renameDialog(page, "Target")).toBeHidden();
  expect((await readContent(api, admin, source.id)).content).toBe("[[Renamed]] and [rooted](/Renamed.md)\n");
  await expectIndexedLinks(db, source.id, [
    { kind: "wikilink", property: null, target: "Renamed", resolved: target.id },
    { kind: "link", property: null, target: "/Renamed.md", resolved: target.id },
  ]);
  const article = page.getByRole("article", { name: "Source" });
  const link = article.getByRole("link", { name: "Renamed", exact: true });
  await expect(link).toHaveAttribute("href", wikiPagePath(workspace.slug, notebook.id, target.id));
  await link.click();
  await expect(pageHeading(page, "Renamed")).toBeFocused();
});
