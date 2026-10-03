import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { memberOf, removeMember } from "../../fixtures/members";
import { createNotebook } from "../../fixtures/notebooks";
import { ownerlessNotebooks } from "../../fixtures/ownerless";
import { listedOwnerless, ownerlessListed, ownerlessPath } from "../../fixtures/ownerless-pages";
import { createPage, openSession, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// PG14, a notebook's activity (M3 design 4; M4/P4 design 3.10): an
// ownerless notebook's size is its pages' bytes, and its last activity the
// latest page write. The ownerless list (M3/P5) shows that size (M4/P5
// design 3.13).

test("PG14 (API): an ownerless notebook's size is the bytes of its pages' contents, and its last activity the latest page write, a session's second save", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Plans");
  const created = "Created with its content.\n";
  await createPage(api, ownerPat, notebook.id, "Intro", null, created);
  const notes = await createPage(api, ownerPat, notebook.id, "Notes");
  const session = await openSession(api, ownerPat, notes.id);
  await writeContent(api, ownerPat, notes.id, { content: "# Notes\n", base_revision: 1, edit_session_id: session.id });
  const content = "# Notes\n\nThe second save.\n";
  const last = await writeContent(api, ownerPat, notes.id, { content, base_revision: 2, edit_session_id: session.id });
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, workspace.slug, ownerEmail)).id)).response.status
  ).toBe(204);

  const listed = await ownerlessNotebooks(api, adminPat, workspace.slug);
  expect(listed.map((l) => [l.id, l.size_bytes])).toEqual([
    [notebook.id, Buffer.byteLength(created) + Buffer.byteLength(content)],
  ]);
  const [at] = await db.query<{ last: boolean }>("SELECT $1::timestamptz = $2::timestamptz AS last", [
    listed[0]?.last_activity_at,
    last.content_updated_at,
  ]);
  expect(at?.last).toBe(true);
});

test("PG14 (page): the ownerless list gives the notebook's size as its pages' bytes, and its last activity as the day of the latest page write", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Plans");
  const contents = ["Created with its content.\n", "# Notes\n"];
  const intro = await createPage(api, ownerPat, notebook.id, "Intro", null, contents[0]);
  const notes = await createPage(api, ownerPat, notebook.id, "Notes", null, contents[1]);
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, workspace.slug, ownerEmail)).id)).response.status
  ).toBe(204);
  // Each its own day: the notebook's own change, then Intro's write, then Notes', the latest.
  const days = { notebook: "2025-03-01T12:00:00Z", intro: "2025-04-01T12:00:00Z", notes: "2025-05-10T12:00:00Z" };
  await db.query("UPDATE notebooks SET created_at = $2, updated_at = $2 WHERE id = $1", [notebook.id, days.notebook]);
  await Promise.all(
    [
      [intro.id, days.intro],
      [notes.id, days.notes],
    ].map(([id, day]) =>
      db.query(
        `UPDATE changesets SET created_at = $2, updated_at = $2
          WHERE id IN (SELECT changeset_id FROM changeset_items WHERE node_id = $1)`,
        [id, day]
      )
    )
  );
  const page = await signedInPage(tokens);

  await page.goto(ownerlessPath(workspace.slug));

  await expect.poll(() => ownerlessListed(page)).toEqual([listedOwnerless("Plans", ownerEmail, "Private", 0)]);
  const size = contents.reduce((sum, content) => sum + Buffer.byteLength(content), 0);
  const latest = await page.evaluate(
    (iso) => new Intl.DateTimeFormat("en", { dateStyle: "medium" }).format(new Date(iso)),
    days.notes
  );
  expect((await ownerlessListed(page))[0]?.[2]).toMatch(new RegExp(` · Last activity ${latest} · Size ${size} B$`));
});
