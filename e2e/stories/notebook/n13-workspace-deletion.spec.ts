import { expectNotebooksDeletedWith } from "../../fixtures/assert/notebook";
import type { Database } from "../../fixtures/db";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { createWorkspace, deleteWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// N13, a workspace's deletion takes its notebooks, and the purge clears
// them (M3 design 3): the notebook part. The audit records come with
// M3/P3, the page version with M3/P5; the purge is the background job's
// alone (W12's way: the deletion moved back by SQL).

/** Moves the deletion of the workspace id and of everything deleted with it back by days, its accepted invitations' too. */
async function deletedDaysAgo(db: Database, id: string, days: number): Promise<void> {
  const ago = `now() - make_interval(days => ${days})`;
  await db.query(`UPDATE workspaces SET deleted_at = ${ago} WHERE id = $1`, [id]);
  await db.query(`UPDATE workspace_members SET deleted_at = ${ago} WHERE workspace_id = $1`, [id]);
  await db.query(
    `UPDATE workspace_invitations SET deleted_at = ${ago},
       accepted_at = CASE WHEN accepted_at IS NULL THEN NULL ELSE ${ago} END WHERE workspace_id = $1`,
    [id]
  );
  await db.query(
    `UPDATE notebook_members SET deleted_at = ${ago} WHERE notebook_id IN (SELECT id FROM notebooks WHERE workspace_id = $1)`,
    [id]
  );
  await db.query(`UPDATE notebooks SET deleted_at = ${ago} WHERE workspace_id = $1`, [id]);
}

test("N13 (API): deleting a workspace deletes its notebooks and their members with it; the purge clears them 60 days on", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace: old } = await newTeam(api, testInfo, "Old");
  const memberPat = await joinAs(api, pat, old.slug, emailFor(testInfo, "member"), "member");
  await createNotebook(api, pat, old.slug, "Plans");
  await createNotebook(api, memberPat, old.slug, "Notes", "viewer");
  const recent = await createWorkspace(api, pat, "Recent", slugFor(testInfo, "recent"));
  await createNotebook(api, pat, recent.slug, "Kept");

  await deleteWorkspace(api, pat, old.slug);
  await deleteWorkspace(api, pat, recent.slug);
  await expectNotebooksDeletedWith(db, old.id, 2);
  await expectNotebooksDeletedWith(db, recent.id, 1);

  await deletedDaysAgo(db, old.id, 61);
  await deletedDaysAgo(db, recent.id, 59);
  await expect
    .poll(async () => (await db.query("SELECT id FROM workspaces WHERE id = $1", [old.id])).length, {
      message: "the workspace deleted 61 days ago is purged, after its notebooks",
      timeout: 15_000,
    })
    .toBe(0);
  const left = await db.query<{ workspace_id: string; notebooks: number; members: number }>(
    `SELECT n.workspace_id, count(DISTINCT n.id)::int AS notebooks, count(m.id)::int AS members
       FROM notebooks n LEFT JOIN notebook_members m ON m.notebook_id = n.id
      WHERE n.workspace_id = ANY($1) GROUP BY n.workspace_id`,
    [[old.id, recent.id]]
  );
  expect(left).toEqual([{ workspace_id: recent.id, notebooks: 1, members: 1 }]);
});
