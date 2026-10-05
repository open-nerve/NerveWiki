import { expectPurged } from "../../fixtures/assert/workspace";
import { emailFor } from "../../fixtures/auth";
import type { Database } from "../../fixtures/db";
import { invite, joinAs, withdraw } from "../../fixtures/invitations";
import { expect, test } from "../../fixtures/test";
import { createWorkspace, deleteWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// W12, the purge of the soft-deleted rows (M2 design 3; M2/P4 design 3.4):
// River's periodic job runs every jobs.purge_interval, 2 s in the test
// configuration, and deletes what was deleted longer than
// jobs.purge_retention (60 days) ago. Only the background job does this:
// there is no page or API version.

/** Moves the deletion of the workspace id, its members' and its invitations' back by days, as if it were that old. */
async function deletedDaysAgo(db: Database, id: string, days: number): Promise<void> {
  const ago = `now() - make_interval(days => ${days})`;
  await db.query(`UPDATE workspaces SET deleted_at = ${ago} WHERE id = $1`, [id]);
  await db.query(`UPDATE workspace_members SET deleted_at = ${ago} WHERE workspace_id = $1`, [id]);
  await db.query(
    `UPDATE workspace_invitations SET deleted_at = ${ago},
       accepted_at = CASE WHEN accepted_at IS NULL THEN NULL ELSE ${ago} END WHERE workspace_id = $1`,
    [id]
  );
}

test("W12: the purge deletes a workspace deleted 61 days ago with its members and invitations, and an invitation withdrawn as long ago; what is younger stays", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace: old } = await newTeam(api, testInfo, "Old");
  await joinAs(api, pat, old.slug, emailFor(testInfo, "member"), "member");
  await invite(api, pat, old.slug, emailFor(testInfo, "invited"));
  const recent = await createWorkspace(api, pat, "Recent", slugFor(testInfo, "recent"));
  await joinAs(api, pat, recent.slug, emailFor(testInfo, "recent-member"), "member");
  const live = await createWorkspace(api, pat, "Live", slugFor(testInfo, "live"));
  const withdrawn = await invite(api, pat, live.slug, emailFor(testInfo, "withdrawn"));
  await withdraw(api, pat, withdrawn.id);
  const pending = await invite(api, pat, live.slug, emailFor(testInfo, "pending"));

  await deleteWorkspace(api, pat, old.slug);
  await deleteWorkspace(api, pat, recent.slug);
  await deletedDaysAgo(db, old.id, 61);
  await deletedDaysAgo(db, recent.id, 59);
  await db.query("UPDATE workspace_invitations SET deleted_at = now() - interval '61 days' WHERE id = $1", [
    withdrawn.id,
  ]);

  // Each kind of row is purged by its own age: a run between the updates above takes the workspace, its members
  // and invitations before the withdrawn invitation is as old, and the next run that one.
  await expect
    .poll(
      async () =>
        (
          await db.query(
            "SELECT id FROM workspaces WHERE id = $1 UNION ALL SELECT id FROM workspace_invitations WHERE id = $2",
            [old.id, withdrawn.id]
          )
        ).length,
      { message: "the workspace and the invitation deleted 61 days ago are purged", timeout: 15_000 }
    )
    .toBe(0);
  await expectPurged(db, { workspaces: [old.id], invitations: [withdrawn.id] });
  // Younger than the retention, or never deleted: all there.
  const kept = await db.query<{ slug: string; members: number; invitations: number }>(
    `SELECT w.slug, (SELECT count(*)::int FROM workspace_members m WHERE m.workspace_id = w.id) AS members,
            (SELECT count(*)::int FROM workspace_invitations i WHERE i.workspace_id = w.id) AS invitations
       FROM workspaces w WHERE w.id = ANY($1) ORDER BY w.slug`,
    [[recent.id, live.id]]
  );
  expect(kept).toEqual([
    { slug: live.slug, members: 1, invitations: 1 },
    { slug: recent.slug, members: 2, invitations: 1 },
  ]);
  expect(await db.query("SELECT id FROM workspace_invitations WHERE id = $1", [pending.id])).toHaveLength(1);
});
