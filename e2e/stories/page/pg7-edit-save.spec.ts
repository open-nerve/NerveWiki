import { createHash } from "node:crypto";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectContentWritten, expectOneSessionRevision, expectSessionGone } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, endSession, openSession, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// PG7, editing and saving (M4 design 3, 4; M4/P4 design 3.4): the saves of
// an edit session are one changeset with one version of the page; the page
// version comes with the editor (M4/P6).

test("PG7 (API): an editor opens a session and saves twice: one changeset, one version from the first base; the revision, hash and size are the content's; the end deletes the session", async ({
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

  expect((await endSession(api, editorPat, session.id)).response.status).toBe(204);
  await expectSessionGone(db, session.id);
});
