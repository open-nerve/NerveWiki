import { aliveSessionsOf, expectAliveSessions } from "../../fixtures/assert/collab";
import { expectSessionGone, sessionsOf } from "../../fixtures/assert/page";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { holdStream } from "../../fixtures/events";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { notebookHeading } from "../../fixtures/notebook-pages";
import { createNotebook, deleteNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  deleteNode,
  endSession,
  moveNode,
  openSession,
  putContent,
  readContent,
  renameNode,
} from "../../fixtures/pages";
import { expect, test, watchOf } from "../../fixtures/test";
import {
  choosePageAction,
  editorContent,
  holdContentWrites,
  lostBanner,
  movePageWith,
  pageHeading,
  renamePageWith,
  startEditing,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// C4, what the lock guards (M5 design 4.4): a content write in another
// session than the lock's, and a deletion of what another account holds;
// a rename, a move and a write of the content the page holds pass, and so
// does the holder's own deletion, and a notebook's. Since M6 a rename or a
// move whose links are written again in a page held, the page renamed's own
// content too, is 409 linking.pages_locked (M6/P4 design 6; L3): no page
// here links to the page renamed.

test("C4 (API): while A edits Notes, a write without A's session, B's or A's own token's, is 409 page.locked, as is B's deletion of Notes or of its parent; a rename, a move and a write of the same content pass; A's deletion passes and ends A's session; the notebook's deletion passes", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat: a, workspace } = await newTeam(api, testInfo);
  const b = await joinAs(api, a, workspace.slug, emailFor(testInfo, "b"), "member");
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  const parent = await createPage(api, a, notebook.id, "Parent");
  const notes = await createPage(api, a, notebook.id, "Notes", parent.id, "Held\n");
  const session = await openSession(api, a, notes.id);

  const writes = [
    await putContent(api, b, notes.id, { content: "Theirs\n", base_revision: 1 }),
    await putContent(api, a, notes.id, { content: "Mine\n", base_revision: 1 }),
  ];
  const deletions = [await deleteNode(api, b, notes.id), await deleteNode(api, b, parent.id)];
  const lock = { page_id: notes.id, user_id: adminId, display_name: displayNameOf(adminEmail) };
  expect([...writes, ...deletions].map((r) => [r.response.status, r.error?.code, r.error?.lock])).toEqual([
    [409, "page.locked", lock],
    [409, "page.locked", lock],
    [409, "page.locked", lock],
    [409, "page.locked", lock],
  ]);
  expect(await readContent(api, a, notes.id)).toMatchObject({ content: "Held\n", revision: 1 });

  expect((await renameNode(api, b, notes.id, "Renamed")).response.status).toBe(200);
  await moveNode(api, b, notes.id, { parent_id: null });
  const same = await putContent(api, b, notes.id, { content: "Held\n", base_revision: 1 });
  expect([same.response.status, same.data?.revision]).toEqual([200, 1]);
  await expectAliveSessions(db, notes.id, [session.id]);

  expect((await deleteNode(api, a, notes.id)).response.status).toBe(204);
  await expectSessionGone(db, session.id);

  const other = await createPage(api, a, notebook.id, "Other");
  await openSession(api, b, other.id);
  expect((await deleteNotebook(api, a, notebook.id)).response.status).toBe(204);
  expect(await sessionsOf(db, other.id)).toEqual([]);
});

test("C4 (MCP): while A edits Notes, B's agent's write and edit are page.locked by A, the lock with Notes' path, and A's own agent's write by A, as A's; B's deletions of Notes and of its parent, and A's own, are page.locked, A's session alive; B's agent renames and moves Notes and writes the content it holds; once A's session ends, A's agent deletes it", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { adminEmail, adminId, pat: a, workspace } = await newTeam(api, testInfo);
  const b = await joinAs(api, a, workspace.slug, emailFor(testInfo, "b"), "member");
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  const team = await createPage(api, a, notebook.id, "Team");
  const notes = await createPage(api, a, notebook.id, "Notes", team.id, "Held\n");
  const session = await openSession(api, a, notes.id);
  const aAgent = await mcpClient(nervewiki.baseURL, a, { notebook: notebook.id, client: "a-agent" });
  const bAgent = await mcpClient(nervewiki.baseURL, b, { notebook: notebook.id, client: "b-agent" });
  const name = displayNameOf(adminEmail);
  const read = await bAgent.call("read", { path: "Team/Notes.md" });
  const { version } = JSON.parse(read.text[0] ?? "{}") as { version: string };
  expect(version).toBe(`${notes.id}:1`);

  const lock = { page_id: notes.id, path: "Team/Notes.md", user_id: adminId, display_name: name, you: false };
  const theirs = await Promise.all([
    bAgent.call("write", { path: "Team/Notes.md", content: "Theirs\n", base_version: version }),
    bAgent.call("edit", { path: "Team/Notes.md", old_string: "Held", new_string: "Theirs" }),
  ]);
  for (const [i, answer] of theirs.entries()) {
    const refused = errorOf(answer, `B's ${["write", "edit"][i]}`);
    expect([refused.code, refused.lock]).toEqual(["page.locked", lock]);
    expect(refused.message).toContain(`by ${name} in the web editor`);
    expect(refused.message).toContain("cannot take the lock over");
  }
  const own = errorOf(
    await aAgent.call("write", { path: "Team/Notes.md", content: "Mine\n", base_version: version }),
    "A's write"
  );
  expect([own.code, own.lock]).toEqual(["page.locked", { ...lock, you: true }]);
  expect(own.message).toContain("by you, in a browser");

  const { path: _, ...holder } = lock;
  const deletions = ["Team/Notes.md", "Team.md"];
  const deleted = await Promise.all(deletions.map((path) => bAgent.call("delete", { path, recursive: true })));
  for (const [i, answer] of deleted.entries()) {
    const refused = errorOf(answer, `B's delete of ${deletions[i]}`);
    expect([refused.code, refused.locks, refused.locks_count, refused.lock]).toEqual([
      "page.locked",
      [lock],
      1,
      holder,
    ]);
  }
  const mine = errorOf(await aAgent.call("delete", { path: "Team/Notes.md" }), "A's delete");
  expect([mine.code, mine.locks]).toEqual(["page.locked", [{ ...lock, you: true }]]);
  await expectAliveSessions(db, notes.id, [session.id]);
  expect(await readContent(api, a, notes.id)).toMatchObject({ content: "Held\n", revision: 1 });

  resultOf(await bAgent.call("move", { from: "Team/Notes.md", to: "Team/Notes 2.md" }), "B's rename");
  resultOf(await bAgent.call("move", { from: "Team/Notes 2.md", to: "Notes 2.md" }), "B's move to the root");
  const same = resultOf<{ changed: boolean; version: string }>(
    await bAgent.call("write", { path: "Notes 2.md", content: "Held\n", base_version: version }),
    "B's write of the content Notes holds"
  );
  expect([same.changed, same.version]).toEqual([false, version]);
  await expectAliveSessions(db, notes.id, [session.id]);

  expect((await endSession(api, a, session.id)).response.status).toBe(204);
  resultOf(await aAgent.call("delete", { path: "Notes 2.md" }), "A's delete once the session ended");
  expect(await sessionsOf(db, notes.id)).toEqual([]);
});

test("C4 (page): while A edits Linux, B's deletion of its parent is refused, the dialog naming A and Linux, A's session alive; B renames and moves it, A's editor staying; A deleting it in another tab keeps the editor and its unsaved text, saying the page is gone, until A leaves it, its session gone", async ({
  anotherPage,
  anotherTab,
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { adminEmail, pat: a, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const b = await anotherPage(await joinOnboarded(api, a, workspace.slug, emailFor(testInfo, "b"), "member"));
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  const guide = await createPage(api, a, notebook.id, "Guide");
  const linux = await createPage(api, a, notebook.id, "Linux", guide.id, "Drafted.\n");
  const linuxPath = wikiPagePath(workspace.slug, notebook.id, linux.id);
  const page = await signedInPage(tokens);

  await page.goto(linuxPath);
  await startEditing(page);
  // A's writes are held: autosave's goes out and is not answered, so that what A types stays unsaved (M5/P5).
  const held = await holdContentWrites(page, linux.id);
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("Unsaved");
  await held.sent();
  const [mine] = await aliveSessionsOf(db, linux.id);

  await b.goto(wikiPagePath(workspace.slug, notebook.id, guide.id));
  await expect(pageHeading(b, "Guide")).toBeVisible();
  await choosePageAction(b, "Plans", "Guide", "Delete");
  const dialog = b.getByRole("alertdialog", { name: "Delete Guide?" });
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(dialog.getByRole("alert")).toHaveText(`${displayNameOf(adminEmail)} is editing “Linux”.`);
  watchOf(b).expectConsole({ errors: [failedToLoad(409)] });
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await expectAliveSessions(db, linux.id, [mine ?? ""]);

  // A rename and a move of what A edits pass (M5 design 4.4; no link to it to write again, M6): A's editor stays,
  // its heading following.
  const attachmentsRead = (readers = [page, b]) =>
    Promise.all(
      readers.map(async (each) => {
        const request = await each.waitForRequest((sent) => {
          const url = new URL(sent.url());
          return (
            sent.method() === "GET" &&
            url.pathname === `/api/v0/notebooks/${notebook.id}/assets` &&
            url.searchParams.get("parent_id") === linux.id
          );
        });
        const answer = await request.response();
        expect(answer?.status()).toBe(200);
        expect(await answer?.finished()).toBeNull();
      })
    );
  // Finish B's initial read and then the connection's refresh separately, so neither can stand in
  // for the later rename or move event's attachment read.
  const letStreamIn = await holdStream(b);
  const initialAttachments = attachmentsRead([b]);
  await b.goto(linuxPath);
  await expect(pageHeading(b, "Linux")).toBeVisible();
  await initialAttachments;
  const connectedAttachments = attachmentsRead([b]);
  letStreamIn();
  await connectedAttachments;
  // Finish each event's attachment refresh while the page exists. A move's tree refresh can wait for
  // the 500 ms interval; deleting meanwhile would make its later attachment read legitimately return 404.
  const afterRename = attachmentsRead();
  expect((await renamePageWith(b, "Plans", linux.id, "Linux", "Kernel")).status()).toBe(200);
  await afterRename;
  const afterMove = attachmentsRead();
  expect((await movePageWith(b, "Plans", linux.id, "Kernel", "The notebook's top level", "Last")).status()).toBe(200);
  await afterMove;
  await expect(pageHeading(page, "Kernel")).toBeVisible();
  await expect(editorContent(page)).toContainText("Drafted.Unsaved");
  await expectAliveSessions(db, linux.id, [mine ?? ""]);

  // A's own edit does not refuse A's deletion (M5 design 4.4).
  const tab = await anotherTab(page);
  await tab.goto(linuxPath);
  await expect(pageHeading(tab, "Kernel")).toBeVisible();
  await choosePageAction(tab, "Plans", "Kernel", "Delete");
  await tab
    .getByRole("alertdialog", { name: "Delete Kernel?" })
    .getByRole("button", { name: "Delete", exact: true })
    .click();
  await expect(notebookHeading(tab, "Plans")).toBeVisible();

  await expect(lostBanner(page)).toContainText("This page no longer exists: this editor saves no more.");
  await expect(lostBanner(page)).toContainText("Your changes here are not saved: copy them before you leave.");
  await expect(pageHeading(page, "Kernel")).toBeVisible();
  await expect(editorContent(page)).toContainText("Drafted.Unsaved");
  await expectSessionGone(db, mine ?? "");
  await lostBanner(page).getByRole("button", { name: "Back to reading", exact: true }).click();
  await page
    .getByRole("alertdialog", { name: "Leave without saving?" })
    .getByRole("button", { name: "Leave", exact: true })
    .click();
  await expect(pageHeading(page, "Page not found")).toBeVisible();
  // The write held all along reaches a page gone.
  expect((await held.release()).map((answer) => answer.status())).toEqual([404]);
  // The session's beat on the event, the opening it tried again, and the write held: all not found.
  pageWatch.expectConsole({ errors: [failedToLoad(404), failedToLoad(404), failedToLoad(404)] });
});

test("C4 (page, notebook): while A edits Notes, unsaved, the notebook's deletion passes: A's editor keeps its text, saying the page is gone, until A leaves it; the session is gone", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat: a, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, a, workspace.slug, "Plans");
  const notes = await createPage(api, a, notebook.id, "Notes", null, "Drafted.\n");
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, notes.id));
  await startEditing(page);
  const held = await holdContentWrites(page, notes.id);
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("Unsaved");
  await held.sent();
  const [mine] = await aliveSessionsOf(db, notes.id);

  expect((await deleteNotebook(api, a, notebook.id)).response.status).toBe(204);

  await expect(lostBanner(page)).toContainText("This page no longer exists: this editor saves no more.");
  await expect(lostBanner(page)).toContainText("Your changes here are not saved: copy them before you leave.");
  await expect(pageHeading(page, "Notes")).toBeVisible();
  await expect(editorContent(page)).toContainText("Drafted.Unsaved");
  expect(await sessionsOf(db, notes.id)).toEqual([]);
  await expectSessionGone(db, mine ?? "");
  await lostBanner(page).getByRole("button", { name: "Back to reading", exact: true }).click();
  await page
    .getByRole("alertdialog", { name: "Leave without saving?" })
    .getByRole("button", { name: "Leave", exact: true })
    .click();
  await expect(pageHeading(page, "Page not found")).toBeVisible();
  expect((await held.release()).map((answer) => answer.status())).toEqual([404]);
  // The tree and the right column's versions read again on the lock's event and on the connection after the
  // stream's reset, the right column's backlinks and properties and the page's attachments on the connection, the
  // session's beat, the opening tried again, and the write held: all not found.
  pageWatch.expectConsole({ errors: Array.from({ length: 10 }, () => failedToLoad(404)) });
});
