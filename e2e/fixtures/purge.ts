import type { Database } from "./db";

// The purge of the stories: the background job's alone (W12's way). A
// story deletes through the API, then moves the deletion back by SQL.

/**
 * Moves the deletion of the workspace id and of everything deleted with it back by days, its accepted invitations' too:
 * leaf to root, so a purge that runs between two statements never meets a row moved back whose children are not. Its
 * notebooks' pages and what follows them come first, their attachments' rows before them all (M7/P2): a node or a
 * notebook moved back without what references it would fail every purge on that foreign key. A purge whose own
 * statements straddle the move may still meet such a row and fail, to be retried; a story that checks the purge's runs
 * looks at those queued after the move.
 */
export async function deletedDaysAgo(db: Database, id: string, days: number): Promise<void> {
  const ago = `now() - make_interval(days => ${days})`;
  const notebooks = "SELECT id FROM notebooks WHERE workspace_id = $1";
  const nodes = `SELECT id FROM nodes WHERE notebook_id IN (${notebooks})`;
  await db.query(`UPDATE asset_blobs SET deleted_at = ${ago} WHERE node_id IN (${nodes})`, [id]);
  await db.query(`UPDATE changeset_items SET deleted_at = ${ago} WHERE node_id IN (${nodes})`, [id]);
  await db.query(`UPDATE page_revisions SET deleted_at = ${ago} WHERE node_id IN (${nodes})`, [id]);
  await db.query(`UPDATE page_contents SET deleted_at = ${ago} WHERE node_id IN (${nodes})`, [id]);
  await db.query(`UPDATE nodes SET deleted_at = ${ago} WHERE notebook_id IN (${notebooks})`, [id]);
  await db.query(`UPDATE changesets SET deleted_at = ${ago} WHERE notebook_id IN (${notebooks})`, [id]);
  await db.query(`UPDATE notebook_audit_events SET deleted_at = ${ago} WHERE workspace_id = $1`, [id]);
  await db.query(`UPDATE notebook_members SET deleted_at = ${ago} WHERE notebook_id IN (${notebooks})`, [id]);
  await db.query(`UPDATE notebooks SET deleted_at = ${ago} WHERE workspace_id = $1`, [id]);
  await db.query(
    `UPDATE workspace_invitations SET deleted_at = ${ago},
       accepted_at = CASE WHEN accepted_at IS NULL THEN NULL ELSE ${ago} END WHERE workspace_id = $1`,
    [id]
  );
  await db.query(`UPDATE workspace_members SET deleted_at = ${ago} WHERE workspace_id = $1`, [id]);
  await db.query(`UPDATE workspaces SET deleted_at = ${ago} WHERE id = $1`, [id]);
}
