import type { Workspace } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import type { Database } from "../db";

// Database assertions of the workspace stories, by table. The page version
// and the API version of a story call the same function (v0.1 design 10.1).

/** A workspace's row, as the assertions read it. */
interface WorkspaceRow {
  id: string;
  slug: string;
  name: string;
  created_by_id: string;
  updated_by_id: string;
  created_at: Date;
  updated_at: Date;
  deleted_at: Date | null;
}

/** A membership's row, as the assertions read it. */
interface MemberRow {
  user_id: string;
  role: string;
  ended_at: Date | null;
  created_by_id: string;
  created_at: Date;
  deleted_at: Date | null;
}

/**
 * workspaces and workspace_members: created is new, as the API answered
 * it, created by adminId, who is its one member, an admin since then.
 */
export async function expectNewWorkspace(db: Database, created: Workspace, adminId: string): Promise<void> {
  const rows = await db.query<WorkspaceRow>(
    `SELECT id, slug, name, created_by_id, updated_by_id, created_at, updated_at, deleted_at
       FROM workspaces WHERE id = $1`,
    [created.id]
  );
  expect(rows).toEqual([
    {
      id: created.id,
      slug: created.slug,
      name: created.name,
      created_by_id: adminId,
      updated_by_id: adminId,
      created_at: new Date(created.created_at),
      updated_at: new Date(created.created_at),
      deleted_at: null,
    },
  ]);
  const members = await db.query<MemberRow>(
    `SELECT user_id, role, ended_at, created_by_id, created_at, deleted_at
       FROM workspace_members WHERE workspace_id = $1`,
    [created.id]
  );
  expect(members).toEqual([
    {
      user_id: adminId,
      role: "admin",
      ended_at: null,
      created_by_id: adminId,
      created_at: new Date(created.created_at),
      deleted_at: null,
    },
  ]);
}

/** The rows of the workspace tables. */
export interface WorkspaceCounts {
  workspaces: number;
  members: number;
}

export async function countWorkspaces(db: Database): Promise<WorkspaceCounts> {
  const [counts] = await db.query<WorkspaceCounts>(
    `SELECT (SELECT count(*)::int FROM workspaces) AS workspaces, (SELECT count(*)::int FROM workspace_members) AS members`
  );
  if (!counts) {
    throw new Error("the counts query returned no row");
  }
  return counts;
}

/** A refused creation added no workspace and no membership. */
export async function expectNoWorkspaceAdded(db: Database, before: WorkspaceCounts): Promise<void> {
  expect(await countWorkspaces(db)).toEqual(before);
}

/** workspaces: renamed is the row as the API answered the rename, last updated by adminId. */
export async function expectRenamed(db: Database, renamed: Workspace, adminId: string): Promise<void> {
  const rows = await db.query<Pick<WorkspaceRow, "name" | "updated_by_id" | "updated_at" | "deleted_at">>(
    `SELECT name, updated_by_id, updated_at, deleted_at FROM workspaces WHERE id = $1`,
    [renamed.id]
  );
  expect(rows).toEqual([
    { name: renamed.name, updated_by_id: adminId, updated_at: new Date(renamed.updated_at), deleted_at: null },
  ]);
}

/**
 * workspaces and workspace_members: the workspace id is deleted by
 * deleterId, and every membership of it with it, at the same time.
 */
export async function expectDeletedWithItsMembers(db: Database, id: string, deleterId: string): Promise<void> {
  const [workspace] = await db.query<Pick<WorkspaceRow, "updated_by_id" | "updated_at" | "deleted_at">>(
    `SELECT updated_by_id, updated_at, deleted_at FROM workspaces WHERE id = $1`,
    [id]
  );
  expect(workspace?.deleted_at).toBeInstanceOf(Date);
  expect(workspace).toEqual({
    updated_by_id: deleterId,
    updated_at: workspace?.deleted_at,
    deleted_at: workspace?.deleted_at,
  });
  const members = await db.query<{ deleted_at: Date | null; updated_by_id: string }>(
    `SELECT deleted_at, updated_by_id FROM workspace_members WHERE workspace_id = $1`,
    [id]
  );
  expect(members.length).toBeGreaterThan(0);
  for (const m of members) {
    expect(m).toEqual({ deleted_at: workspace?.deleted_at, updated_by_id: deleterId });
  }
}
