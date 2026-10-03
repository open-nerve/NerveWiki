import { createHash } from "node:crypto";

import type { Page as WikiPage } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectContentWritten, expectOneSessionRevision, expectSessionGone } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { answerTo } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, endSession, getView, openSession, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  editorContent,
  holdContentWrites,
  pageHeading,
  pageTree,
  saveEdit,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newTeam } from "../../fixtures/workspaces";

// PG7, editing and saving (M4 design 3, 4; M4/P4 design 3.4): the saves of
// an edit session are one changeset with one version of the page; in the
// browser, the editor's (M4/P6 design 3.6, 3.7).

test("PG7 (API): an editor opens a session and saves twice: one changeset, one version from the first base; the revision, hash and size are the content's, and the reading view shows it; the end deletes the session", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const editorPat = await joinAs(api, adminPat, workspace.slug, editorEmail, "member");
  const editorId = await accountIdOf(db, editorEmail);
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Plans", "editor");
  const page = await createPage(api, adminPat, notebook.id, "Notes");

  const session = await openSession(api, editorPat, page.id);
  expect(session).toMatchObject({ page_id: page.id });
  const first = await writeContent(api, editorPat, page.id, {
    content: "# Notes\n",
    base_revision: 1,
    edit_session_id: session.id,
  });
  expect(first.revision).toBe(2);
  const content = "# Notes\n\nSecond save.\n";
  const second = await writeContent(api, editorPat, page.id, {
    content,
    base_revision: 2,
    edit_session_id: session.id,
  });
  expect(second).toMatchObject({ revision: 3, byte_size: Buffer.byteLength(content), content_updated_by: editorId });
  await expectContentWritten(db, second, content, editorId);
  await expectOneSessionRevision(db, session.id, page.id, 1, 3);
  expect(await readContent(api, editorPat, page.id)).toEqual({
    content,
    revision: 3,
    content_hash: createHash("sha256").update(content, "utf8").digest("hex"),
  });
  const view = await getView(api, editorPat, page.id);
  expect(view.data?.revision).toBe(3);
  expect(view.data?.html).toContain("<p>Second save.</p>");

  expect((await endSession(api, editorPat, session.id)).response.status).toBe(204);
  await expectSessionGone(db, session.id);
});

test("PG7 (page): an editor opens the editor with Ctrl+E and saves twice with Ctrl+S, one changeset and one version; unsaved, another page asks first; Ctrl+E saves the rest, ends the session and shows the new content", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const page = await signedInPage(await joinOnboarded(api, adminPat, workspace.slug, editorEmail, "member"));
  const editorId = await accountIdOf(db, editorEmail);
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Plans", "editor");
  const notes = await createPage(api, adminPat, notebook.id, "Notes");
  await createPage(api, adminPat, notebook.id, "Other");

  await page.goto(wikiPagePath(workspace.slug, notebook.id, notes.id));
  await expect(pageHeading(page, "Notes")).toBeVisible();
  await page.keyboard.press("ControlOrMeta+e");
  const content = editorContent(page);
  await expect(content).toBeFocused();
  await page.keyboard.type("# Notes");
  await saveEdit(page);
  const [session] = await db.query<{ id: string }>("SELECT id FROM edit_sessions WHERE node_id = $1", [notes.id]);
  await page.keyboard.press("Enter");
  await page.keyboard.press("Enter");
  await page.keyboard.type("Second save.");
  const answer = answerTo(page, "PUT", `/api/v0/pages/${notes.id}/content`);
  await saveEdit(page);
  const second = (await (await answer).json()) as WikiPage;
  await expectContentWritten(db, second, "# Notes\n\nSecond save.", editorId);
  await expectOneSessionRevision(db, session?.id ?? "", notes.id, 1, 3);

  // The writes are held from here: what is typed stays unsaved while the dialog asks, autosave or not (M5/P5).
  const held = await holdContentWrites(page, notes.id);
  await page.keyboard.type(" More.");
  await pageTree(page, "Plans").getByRole("link", { name: "Other", exact: true }).click();
  await page
    .getByRole("alertdialog", { name: "Leave without saving?" })
    .getByRole("button", { name: "Stay", exact: true })
    .click();
  await expect(pageHeading(page, "Notes")).toBeVisible();
  await expect(content).toContainText("Second save. More.");
  // The keys do nothing while a dialog is open (M4/P6): Ctrl+E goes once it is gone.
  await expect(page.getByRole("alertdialog")).toHaveCount(0);

  await page.keyboard.press("ControlOrMeta+e");
  await held.sent();
  // One write of the rest: autosave's, if it went first, and Ctrl+E's are one.
  expect((await held.release()).map((write) => write.status())).toEqual([200]);
  await expect(page.getByRole("article")).toContainText("Second save. More.");
  await expect(page.getByRole("main").getByRole("button", { name: "Edit", exact: true })).toBeFocused();
  expect(await readContent(api, adminPat, notes.id)).toMatchObject({
    content: "# Notes\n\nSecond save. More.",
    revision: 4,
  });
  await expect
    .poll(async () => (await db.query("SELECT 1 FROM edit_sessions WHERE node_id = $1", [notes.id])).length)
    .toBe(0);
  await expectSessionGone(db, session?.id ?? "");
});
