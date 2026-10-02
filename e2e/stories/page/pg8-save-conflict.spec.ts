import { accountIdOf } from "../../fixtures/assert/identity";
import { expectContentWritten, expectOneSessionRevision } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, openSession, putContent, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// PG8, a save that conflicts (M4 design 3, 4): a write that came in first
// refuses a save on the version before it; the editor reads the current
// content and keeps theirs on it. The page version comes with the editor
// (M4/P6).

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
