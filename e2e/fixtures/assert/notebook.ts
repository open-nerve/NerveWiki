import type { Notebook } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import type { Database } from "../db";

// Database assertions of the notebook stories, by table. The page version
// and the API version of a story call the same function (v0.1 design 10.1).
// The times are compared in SQL, to the microsecond: a JS Date keeps
// milliseconds only.

/**
 * notebooks and notebook_members: created is new, as the API answered it,
 * created by adminId, who is its one member, its admin since then.
 */
export async function expectNewNotebook(db: Database, created: Notebook, adminId: string): Promise<void> {
  const rows = await db.query(
    `SELECT workspace_id, name, workspace_access, created_by_id = $2 AND updated_by_id = $2 AS by_admin,
            created_at = $3::timestamptz AND updated_at = created_at AS at_answer,
            deleted_at IS NULL AND ownerless_since IS NULL AND former_owner_id IS NULL AS live
       FROM notebooks WHERE id = $1`,
    [created.id, adminId, created.created_at]
  );
  expect(rows).toEqual([
    {
      workspace_id: created.workspace_id,
      name: created.name,
      workspace_access: created.workspace_access,
      by_admin: true,
      at_answer: true,
      live: true,
    },
  ]);
  const members = await db.query(
    `SELECT user_id, role, created_by_id, created_at = $2::timestamptz AS at_creation, ended_at IS NULL AND deleted_at IS NULL AS active
       FROM notebook_members WHERE notebook_id = $1`,
    [created.id, created.created_at]
  );
  expect(members).toEqual([
    { user_id: adminId, role: "admin", created_by_id: adminId, at_creation: true, active: true },
  ]);
}

/** notebooks: notebook is the row as the API answered a change, last updated by updaterId. */
export async function expectNotebook(db: Database, notebook: Notebook, updaterId: string): Promise<void> {
  const rows = await db.query(
    `SELECT name, workspace_access, updated_by_id = $2 AS by_updater, updated_at = $3::timestamptz AS at_answer,
            deleted_at IS NOT NULL AS deleted
       FROM notebooks WHERE id = $1`,
    [notebook.id, updaterId, notebook.updated_at]
  );
  expect(rows).toEqual([
    {
      name: notebook.name,
      workspace_access: notebook.workspace_access,
      by_updater: true,
      at_answer: true,
      deleted: false,
    },
  ]);
}

/**
 * notebooks and notebook_members: the notebook id is deleted by deleterId,
 * and every membership of it with it, at the same time.
 */
export async function expectNotebookDeletedWithItsMembers(db: Database, id: string, deleterId: string): Promise<void> {
  const rows = await db.query<{ deleted: boolean; by_deleter: boolean; members: number; members_apart: number }>(
    `SELECT n.deleted_at IS NOT NULL AND n.updated_at = n.deleted_at AS deleted, n.updated_by_id = $2 AS by_deleter,
            (SELECT count(*)::int FROM notebook_members m WHERE m.notebook_id = n.id) AS members,
            (SELECT count(*)::int FROM notebook_members m WHERE m.notebook_id = n.id
               AND (m.deleted_at IS DISTINCT FROM n.deleted_at OR m.updated_at IS DISTINCT FROM n.deleted_at
                    OR m.updated_by_id <> $2)) AS members_apart
       FROM notebooks n WHERE n.id = $1`,
    [id, deleterId]
  );
  expect(rows).toEqual([{ deleted: true, by_deleter: true, members: expect.any(Number), members_apart: 0 }]);
  expect(rows[0]?.members).toBeGreaterThan(0);
}

/**
 * notebooks and notebook_members: every notebook of the workspace id,
 * count of them, is deleted with it, at its time, by its deleter, and
 * every membership of them too.
 */
export async function expectNotebooksDeletedWith(db: Database, workspaceId: string, count: number): Promise<void> {
  const rows = await db.query<{ live: number; with_it: number; members_apart: number }>(
    `SELECT (SELECT count(*)::int FROM notebooks n WHERE n.workspace_id = w.id AND n.deleted_at IS NULL) AS live,
            (SELECT count(*)::int FROM notebooks n WHERE n.workspace_id = w.id
               AND n.deleted_at = w.deleted_at AND n.updated_by_id = w.updated_by_id) AS with_it,
            (SELECT count(*)::int FROM notebook_members m JOIN notebooks n ON n.id = m.notebook_id WHERE n.workspace_id = w.id
               AND (m.deleted_at IS DISTINCT FROM w.deleted_at OR m.updated_by_id <> w.updated_by_id)) AS members_apart
       FROM workspaces w WHERE w.id = $1`,
    [workspaceId]
  );
  expect(rows).toEqual([{ live: 0, with_it: count, members_apart: 0 }]);
}

/** The rows of the notebook tables. */
export interface NotebookCounts {
  notebooks: number;
  members: number;
}

export async function countNotebooks(db: Database): Promise<NotebookCounts> {
  const [counts] = await db.query<NotebookCounts>(
    `SELECT (SELECT count(*)::int FROM notebooks) AS notebooks, (SELECT count(*)::int FROM notebook_members) AS members`
  );
  if (!counts) {
    throw new Error("the counts query returned no row");
  }
  return counts;
}
