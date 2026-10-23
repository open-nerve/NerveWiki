import { accountIdOf } from "../../fixtures/assert/identity";
import { expectAliveSessions } from "../../fixtures/assert/collab";
import { expectIndexedLinks } from "../../fixtures/assert/links";
import { expectChangesetOf } from "../../fixtures/assert/page";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { joinAs } from "../../fixtures/invitations";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
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
// its editor, and nothing changes until the editor is done. An agent's
// move is refused alike, its own editor's too; one that leaves the links
// as they are passes a page being edited (M9/P5 design 3.4).

/** A move's answer, as the tool's structuredContent has it. */
interface Moved {
  path: string;
  node_id: string;
  changed: boolean;
  changeset_id: string | null;
}

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

test("L3 (MCP): an agent's move whose rewrite reaches a page another edits is linking.pages_locked naming the page and its editor, the editor's own agent's too, and moves nothing; once the editor is done, the editor's agent renames it and the links follow, as the mover; a move without its links passes a page being edited and leaves it as it is", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { adminId, pat: admin, workspace } = await newTeam(api, testInfo);
  const aEmail = emailFor(testInfo, "a");
  const a = await joinAs(api, admin, workspace.slug, aEmail, "member");
  const aId = await accountIdOf(db, aEmail);
  const notebook = await createNotebook(api, admin, workspace.slug, "Plans", "editor");
  const target = await createPage(api, admin, notebook.id, "Target");
  const source = await createPage(api, admin, notebook.id, "Source", null, "[[Target]] and [md](Target.md)\n");
  const session = await openSession(api, a, source.id);
  const adminAgent = await mcpClient(nervewiki.baseURL, admin, { notebook: notebook.id, client: "admin-agent" });
  const aAgent = await mcpClient(nervewiki.baseURL, a, { notebook: notebook.id, client: "a-agent" });

  // The lock names the page with its path and its editor; to the editor's own agent, as its caller's.
  const lock = { page_id: source.id, path: "Source.md", user_id: aId, display_name: displayNameOf(aEmail), you: false };
  const refused = errorOf(await adminAgent.call("move", { from: "Target.md", to: "Renamed.md" }), "the admin's move");
  expect([refused.code, refused.locks, refused.locks_count]).toEqual(["linking.pages_locked", [lock], 1]);
  expect(refused.message).toContain(`by ${displayNameOf(aEmail)}`);
  expect(refused.message).toContain("update_links false");
  const own = errorOf(await aAgent.call("move", { from: "Target.md", to: "Renamed.md" }), "A's agent's move");
  expect([own.code, own.locks, own.locks_count]).toEqual(["linking.pages_locked", [{ ...lock, you: true }], 1]);
  expect(own.message).toContain("by you, in a browser");
  expect((await listNodes(api, admin, notebook.id)).map((node) => node.name).toSorted()).toEqual(["Source", "Target"]);
  expect((await readContent(api, admin, source.id)).revision).toBe(1);
  await expectAliveSessions(db, source.id, [session.id]);

  // The editor done, its agent's move renames Target, and the links follow it, written as the mover.
  expect((await endSession(api, a, session.id)).response.status).toBe(204);
  const renamed = resultOf<Moved>(
    await aAgent.call("move", { from: "Target.md", to: "Renamed.md" }),
    "A's agent's move"
  );
  expect([renamed.path, renamed.node_id, renamed.changed]).toEqual(["Renamed.md", target.id, true]);
  const written = await readContent(api, admin, source.id);
  expect([written.content, written.revision]).toEqual(["[[Renamed]] and [md](Renamed.md)\n", 2]);
  expect((await getPage(api, admin, source.id)).data?.content_updated_by).toBe(aId);
  await expectIndexedLinks(db, source.id, [
    { kind: "wikilink", property: null, target: "Renamed", resolved: target.id },
    { kind: "link", property: null, target: "Renamed.md", resolved: target.id },
  ]);
  await expectChangesetOf(db, String(renamed.changeset_id), {
    client: "mcp:a-agent",
    byId: aId,
    items: [target.id],
    versions: [source.id],
  });

  // A move that leaves the links as they are reaches no lock: it passes Source being edited again, writing no
  // version of it; renamed, its links lead nowhere.
  const again = await openSession(api, a, source.id);
  const kept = resultOf<Moved>(
    await adminAgent.call("move", { from: "Renamed.md", to: "Renamed later.md", update_links: false }),
    "the admin's move without the links"
  );
  expect([kept.path, kept.node_id, kept.changed]).toEqual(["Renamed later.md", target.id, true]);
  await expectChangesetOf(db, String(kept.changeset_id), {
    client: "mcp:admin-agent",
    byId: adminId,
    items: [target.id],
    versions: [],
  });
  expect(await readContent(api, admin, source.id)).toEqual(written);
  await expectIndexedLinks(db, source.id, [
    { kind: "wikilink", property: null, target: "Renamed", resolved: null },
    { kind: "link", property: null, target: "Renamed.md", resolved: null },
  ]);
  await expectAliveSessions(db, source.id, [again.id]);
});

test("L3 (page): a rename in the dialog, and a drag, whose links' page another edits say who edits which page and change nothing; once the editor is done, the rename goes through and the link leads to the page", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { adminId, pat: admin, tokens, workspace } = await newOnboardedTeam(api, testInfo);
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
  // Written again as the renamer.
  expect((await getPage(api, admin, source.id)).data?.content_updated_by).toBe(adminId);
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
