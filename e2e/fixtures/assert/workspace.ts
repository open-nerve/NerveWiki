import type { Workspace, WorkspaceInvitation, WorkspaceRole } from "@nervewiki/api-client";
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

/**
 * workspaces: renamed is the row as the API answered the rename, last
 * updated by adminId. The times are compared in SQL, to the microsecond:
 * a JS Date keeps milliseconds only.
 */
export async function expectRenamed(db: Database, renamed: Workspace, adminId: string): Promise<void> {
  const rows = await db.query<{ name: string; by_admin: boolean; at_answer: boolean; deleted: boolean }>(
    `SELECT name, updated_by_id = $2 AS by_admin, updated_at = $3::timestamptz AS at_answer, deleted_at IS NOT NULL AS deleted
       FROM workspaces WHERE id = $1`,
    [renamed.id, adminId, renamed.updated_at]
  );
  expect(rows).toEqual([{ name: renamed.name, by_admin: true, at_answer: true, deleted: false }]);
}

/**
 * workspaces and workspace_members: the workspace id is deleted by
 * deleterId, and every membership of it with it, at the same time, to the
 * microsecond (compared in SQL).
 */
export async function expectDeletedWithItsMembers(db: Database, id: string, deleterId: string): Promise<void> {
  const rows = await db.query<{ deleted: boolean; by_deleter: boolean; members: number; members_apart: number }>(
    `SELECT w.deleted_at IS NOT NULL AND w.updated_at = w.deleted_at AS deleted, w.updated_by_id = $2 AS by_deleter,
            (SELECT count(*)::int FROM workspace_members m WHERE m.workspace_id = w.id) AS members,
            (SELECT count(*)::int FROM workspace_members m WHERE m.workspace_id = w.id
               AND (m.deleted_at IS DISTINCT FROM w.deleted_at OR m.updated_at IS DISTINCT FROM w.deleted_at
                    OR m.updated_by_id <> $2)) AS members_apart
       FROM workspaces w WHERE w.id = $1`,
    [id, deleterId]
  );
  expect(rows).toEqual([{ deleted: true, by_deleter: true, members: expect.any(Number), members_apart: 0 }]);
  expect(rows[0]?.members).toBeGreaterThan(0);
}

/**
 * workspace_invitations: invitation is pending, as the API answered it,
 * created by adminId; no column holds its token, which is a MAC of the id,
 * neither as the link spells it nor as its bytes in hex.
 */
export async function expectPendingInvitation(
  db: Database,
  invitation: WorkspaceInvitation,
  adminId: string
): Promise<void> {
  const tag = invitation.token.replace(/^nwk_inv_/, "");
  const rows = await db.query<{
    email: string;
    role: string;
    by_admin: boolean;
    at_answer: boolean;
    pending: boolean;
    token_stored: boolean;
  }>(
    `SELECT email, role, created_by_id = $2 AND updated_by_id = $2 AS by_admin, created_at = $3::timestamptz AS at_answer,
            accepted_at IS NULL AND deleted_at IS NULL AS pending,
            strpos(row_to_json(i)::text, $4) > 0 OR strpos(row_to_json(i)::text, $5) > 0 AS token_stored
       FROM workspace_invitations i WHERE id = $1`,
    [invitation.id, adminId, invitation.created_at, tag, Buffer.from(tag, "base64url").toString("hex")]
  );
  expect(rows).toEqual([
    {
      email: invitation.email,
      role: invitation.role,
      by_admin: true,
      at_answer: true,
      pending: true,
      token_stored: false,
    },
  ]);
}

/** What became of an invitation, and who last wrote it. */
export type InvitationState = "pending" | "accepted" | "deleted";

/** workspace_invitations: the invitation id is state, last written by byId; an acceptance is a deletion at its time. */
export async function expectInvitation(db: Database, id: string, state: InvitationState, byId: string): Promise<void> {
  const rows = await db.query<{ state: InvitationState; by: boolean; consistent: boolean }>(
    `SELECT CASE WHEN deleted_at IS NULL THEN 'pending' WHEN accepted_at IS NOT NULL THEN 'accepted' ELSE 'deleted' END AS state,
            updated_by_id = $2 AS by, (deleted_at IS NULL OR updated_at = deleted_at) AS consistent
       FROM workspace_invitations WHERE id = $1`,
    [id, byId]
  );
  expect(rows).toEqual([{ state, by: true, consistent: true }]);
}

/**
 * workspace_invitations: every invitation of the workspace id that was
 * pending when it was deleted, count of them, is deleted with it, at its
 * time, by its deleter (compared in SQL).
 */
export async function expectInvitationsDeletedWith(db: Database, id: string, count: number): Promise<void> {
  const rows = await db.query<{ pending: number; with_it: number }>(
    `SELECT (SELECT count(*)::int FROM workspace_invitations i WHERE i.workspace_id = w.id AND i.deleted_at IS NULL) AS pending,
            (SELECT count(*)::int FROM workspace_invitations i WHERE i.workspace_id = w.id AND i.accepted_at IS NULL
               AND i.deleted_at = w.deleted_at AND i.updated_by_id = w.updated_by_id) AS with_it
       FROM workspaces w WHERE w.id = $1`,
    [id]
  );
  expect(rows).toEqual([{ pending: 0, with_it: count }]);
}

/** What an account's membership of a workspace is: active with its role, ended, or none. */
export type MembershipState = WorkspaceRole | "ended" | "none";

/**
 * workspace_members: userId's membership of the workspace id is state;
 * joinedAt, when given, is when it was first created, to the microsecond.
 */
export async function expectMembership(
  db: Database,
  id: string,
  userId: string,
  state: MembershipState,
  joinedAt?: string
): Promise<void> {
  const rows = await db.query<{ state: MembershipState; joined: boolean | null }>(
    `SELECT CASE WHEN ended_at IS NOT NULL THEN 'ended' ELSE role END AS state, created_at = $3::timestamptz AS joined
       FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 AND deleted_at IS NULL`,
    [id, userId, joinedAt ?? null]
  );
  if (state === "none") {
    expect(rows).toEqual([]);
    return;
  }
  expect(rows).toEqual([{ state, joined: joinedAt === undefined ? null : true }]);
}

/** When userId's membership of the workspace id ended, as reactivate-member prints it: to the second, in UTC. */
export async function membershipEndedAt(db: Database, id: string, userId: string): Promise<string> {
  const [row] = await db.query<{ ended_at: Date }>(
    "SELECT ended_at FROM workspace_members WHERE workspace_id = $1 AND user_id = $2",
    [id, userId]
  );
  return (row?.ended_at ?? new Date(0)).toISOString().replace(/\.\d{3}Z$/, "Z");
}

/**
 * workspaces, workspace_members and workspace_invitations: nothing is left
 * of the workspaces, their members and invitations, nor of the
 * invitations: the purge deleted them all.
 */
export async function expectPurged(
  db: Database,
  { workspaces, invitations }: { workspaces: string[]; invitations: string[] }
): Promise<void> {
  const rows = await db.query<{ workspaces: number; members: number; invitations: number }>(
    `SELECT (SELECT count(*)::int FROM workspaces WHERE id = ANY($1)) AS workspaces,
            (SELECT count(*)::int FROM workspace_members WHERE workspace_id = ANY($1)) AS members,
            (SELECT count(*)::int FROM workspace_invitations WHERE workspace_id = ANY($1) OR id = ANY($2)) AS invitations`,
    [workspaces, invitations]
  );
  expect(rows).toEqual([{ workspaces: 0, members: 0, invitations: 0 }]);
}
