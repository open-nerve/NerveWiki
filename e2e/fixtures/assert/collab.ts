import { expect } from "@playwright/test";

import type { Database } from "../db";

// Database assertions of the collaboration stories (M5 design 4.1, 4.3).

/** edit_sessions: the ids of the page pageId's sessions alive now, its lock's holders, oldest first. */
export async function aliveSessionsOf(db: Database, pageId: string): Promise<string[]> {
  const rows = await db.query<{ id: string }>(
    `SELECT id FROM edit_sessions WHERE node_id = $1 AND ended_reason IS NULL AND expires_at > now()
      ORDER BY created_at, id`,
    [pageId]
  );
  return rows.map((r) => r.id);
}

/** edit_sessions: the page pageId's sessions alive now, its lock's holders: sessionIds, oldest first. */
export async function expectAliveSessions(db: Database, pageId: string, sessionIds: string[]): Promise<void> {
  expect(await aliveSessionsOf(db, pageId)).toEqual(sessionIds);
}

/**
 * edit_sessions: the session id is a tombstone, ended for reason by the account byId, kept a lease after its end at
 * least, so that its tab learns why.
 */
export async function expectTombstone(
  db: Database,
  id: string,
  reason: "taken_over" | "unlocked",
  byId: string
): Promise<void> {
  const rows = await db.query(
    `SELECT ended_reason, ended_by_id, ended_at IS NOT NULL AND ended_at >= created_at AS ended,
            expires_at >= ended_at + interval '120 seconds' AS kept
       FROM edit_sessions WHERE id = $1`,
    [id]
  );
  expect(rows).toEqual([{ ended_reason: reason, ended_by_id: byId, ended: true, kept: true }]);
}
