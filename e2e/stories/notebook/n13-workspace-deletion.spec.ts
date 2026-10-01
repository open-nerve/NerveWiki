import type { ApiClient } from "@nervewiki/api-client";

import { expectAuditEventsDeletedWith, expectNotebooksDeletedWith } from "../../fixtures/assert/notebook";
import type { Database } from "../../fixtures/db";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { memberOf, removeMember } from "../../fixtures/members";
import { createNotebook } from "../../fixtures/notebooks";
import { takeOver } from "../../fixtures/ownerless";
import { expect, test } from "../../fixtures/test";
import { deleteWorkspaceWith, expectCreatePage } from "../../fixtures/workspace-pages";
import { createWorkspace, deleteWorkspace, newOnboardedTeam, newTeam, slugFor } from "../../fixtures/workspaces";

// N13, a workspace's deletion takes its notebooks and its audit records,
// and the purge clears them (M3 design 3; M3/P5 design 3.7 for the page);
// the purge is the background job's alone (W12's way: the deletion moved
// back by SQL).

/**
 * Has email join the workspace of slug, create the notebook name, and be removed by credential, its admin, who takes
 * the notebook over: a notebook with an audit event, and a member's ended membership.
 */
async function takenOver(api: ApiClient, credential: string, slug: string, email: string, name: string): Promise<void> {
  const notebook = await createNotebook(api, await joinAs(api, credential, slug, email, "member"), slug, name);
  expect((await removeMember(api, credential, (await memberOf(api, credential, slug, email)).id)).response.status).toBe(
    204
  );
  expect((await takeOver(api, credential, notebook.id)).response.status).toBe(200);
}

/**
 * Moves the deletion of the workspace id and of everything deleted with it back by days, its accepted invitations' too:
 * leaf to root, so the purge, which may run between two statements, never meets a row moved back whose children are not.
 */
async function deletedDaysAgo(db: Database, id: string, days: number): Promise<void> {
  const ago = `now() - make_interval(days => ${days})`;
  await db.query(`UPDATE notebook_audit_events SET deleted_at = ${ago} WHERE workspace_id = $1`, [id]);
  await db.query(
    `UPDATE notebook_members SET deleted_at = ${ago} WHERE notebook_id IN (SELECT id FROM notebooks WHERE workspace_id = $1)`,
    [id]
  );
  await db.query(`UPDATE notebooks SET deleted_at = ${ago} WHERE workspace_id = $1`, [id]);
  await db.query(
    `UPDATE workspace_invitations SET deleted_at = ${ago},
       accepted_at = CASE WHEN accepted_at IS NULL THEN NULL ELSE ${ago} END WHERE workspace_id = $1`,
    [id]
  );
  await db.query(`UPDATE workspace_members SET deleted_at = ${ago} WHERE workspace_id = $1`, [id]);
  await db.query(`UPDATE workspaces SET deleted_at = ${ago} WHERE id = $1`, [id]);
}

test("N13 (API): deleting a workspace deletes its notebooks, their members and its audit records with it; the purge clears them 60 days on", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace: old } = await newTeam(api, testInfo, "Old");
  const memberPat = await joinAs(api, pat, old.slug, emailFor(testInfo, "member"), "member");
  await createNotebook(api, pat, old.slug, "Plans");
  await createNotebook(api, memberPat, old.slug, "Notes", "viewer");
  await takenOver(api, pat, old.slug, emailFor(testInfo, "leaver"), "Drafts");
  const recent = await createWorkspace(api, pat, "Recent", slugFor(testInfo, "recent"));
  await createNotebook(api, pat, recent.slug, "Kept");
  await takenOver(api, pat, recent.slug, emailFor(testInfo, "drafter"), "Sketch");

  await deleteWorkspace(api, pat, old.slug);
  await deleteWorkspace(api, pat, recent.slug);
  await expectNotebooksDeletedWith(db, old.id, 3);
  await expectNotebooksDeletedWith(db, recent.id, 2);
  await expectAuditEventsDeletedWith(db, old.id, 1);
  await expectAuditEventsDeletedWith(db, recent.id, 1);

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
  expect(left).toEqual([{ workspace_id: recent.id, notebooks: 2, members: 3 }]);
  const events = await db.query<{ workspace_id: string; events: number }>(
    `SELECT workspace_id, count(*)::int AS events FROM notebook_audit_events WHERE workspace_id = ANY($1) GROUP BY workspace_id`,
    [[old.id, recent.id]]
  );
  expect(events).toEqual([{ workspace_id: recent.id, events: 1 }]);
});

test("N13 (page): its admin deletes a workspace with notebooks on its general page; its notebooks, their members and its audit records go with it", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const memberPat = await joinAs(api, pat, workspace.slug, emailFor(testInfo, "member"), "member");
  await createNotebook(api, pat, workspace.slug, "Plans");
  await createNotebook(api, memberPat, workspace.slug, "Notes", "viewer");
  await takenOver(api, pat, workspace.slug, emailFor(testInfo, "leaver"), "Drafts");
  const page = await signedInPage(tokens);
  await page.goto(`/${workspace.slug}/settings/general`);

  expect(await deleteWorkspaceWith(page, workspace.slug)).toBe(204);

  await expectCreatePage(page);
  await expectNotebooksDeletedWith(db, workspace.id, 3);
  await expectAuditEventsDeletedWith(db, workspace.id, 1);
});
