import { accountIdOf } from "../../fixtures/assert/identity";
import { expectContentWritten, expectOneSessionRevision } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, openSession, putContent, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { conflictRegion, editorContent, editStatus, startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// PG8, a save that conflicts (M4 design 3, 4): a write that came in first
// refuses a save on the version before it; the editor reads the current
// content, shows the differences, and keeps the user's text on it or
// takes theirs (M4/P6 design 3.8).

test("PG8 (API): another credential writes first, so a save on the version before is 409 page.revision_mismatch; the editor reads the current content and keeps theirs on its revision, in a changeset of its own", async ({
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
  expect((await writeContent(api, adminPat, page.id, { content: "Theirs\n", base_revision: 1 })).revision).toBe(2);
  const stale = await putContent(api, editorPat, page.id, {
    content: "Mine\n",
    base_revision: 1,
    edit_session_id: session.id,
  });
  expect([stale.response.status, stale.error?.code]).toEqual([409, "page.revision_mismatch"]);

  const current = await readContent(api, editorPat, page.id);
  expect(current).toMatchObject({ content: "Theirs\n", revision: 2 });
  const kept = await writeContent(api, editorPat, page.id, {
    content: "Mine\n",
    base_revision: current.revision,
    edit_session_id: session.id,
  });
  expect(kept.revision).toBe(3);
  await expectContentWritten(db, kept, "Mine\n", editorId);
  await expectOneSessionRevision(db, session.id, page.id, 2, 3);
});

test("PG8 (page): a token writes while the page is edited; the save shows the differences, and Keep mine saves over them; on another page Discard mine edits theirs, and the next save goes through", async ({
  api,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const kept = await createPage(api, pat, notebook.id, "Kept", null, "Base\n");
  const discarded = await createPage(api, pat, notebook.id, "Discarded", null, "Base\n");
  const page = await signedInPage(tokens);

  await page.goto(wikiPagePath(workspace.slug, notebook.id, kept.id));
  await startEditing(page);
  await writeContent(api, pat, kept.id, { content: "Base\nTheirs\n", base_revision: 1 });
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("Mine");
  await page.keyboard.press("ControlOrMeta+s");
  const conflict = conflictRegion(page);
  await expect(conflict.getByRole("heading")).toBeFocused();
  await expect(
    conflict.getByRole("textbox", { name: "Your text against the page as it is now", exact: true })
  ).toContainText("Mine");
  await expect(conflict).toContainText("Theirs");
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  await conflict.getByRole("button", { name: "Keep mine", exact: true }).click();
  await expect(editStatus(page)).toHaveText("Saved.");
  await expect(conflict).toHaveCount(0);
  expect(await readContent(api, pat, kept.id)).toMatchObject({ content: "Base\nMine", revision: 3 });

  await page.goto(wikiPagePath(workspace.slug, notebook.id, discarded.id));
  await startEditing(page);
  await writeContent(api, pat, discarded.id, { content: "Base\r\nTheirs\r\n", base_revision: 1 });
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("Mine");
  await page.keyboard.press("ControlOrMeta+s");
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  await conflict.getByRole("button", { name: "Discard mine", exact: true }).click();
  const content = editorContent(page);
  await expect(content).toBeFocused();
  await expect(content).toContainText("Theirs");
  await expect(content).not.toContainText("Mine");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("More");
  await page.keyboard.press("ControlOrMeta+s");
  await expect(editStatus(page)).toHaveText("Saved.");
  expect(await readContent(api, pat, discarded.id)).toMatchObject({ content: "Base\r\nTheirs\r\nMore", revision: 3 });
});
