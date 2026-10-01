import type { Notebook, NotebookAuditAction, OwnerlessNotebook } from "@nervewiki/api-client";
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
               AND n.deleted_at = w.deleted_at AND n.updated_at = w.deleted_at AND n.updated_by_id = w.updated_by_id) AS with_it,
            (SELECT count(*)::int FROM notebook_members m JOIN notebooks n ON n.id = m.notebook_id WHERE n.workspace_id = w.id
               AND (m.deleted_at IS DISTINCT FROM w.deleted_at OR m.updated_at IS DISTINCT FROM w.deleted_at
                    OR m.updated_by_id <> w.updated_by_id)) AS members_apart
       FROM workspaces w WHERE w.id = $1`,
    [workspaceId]
  );
  expect(rows).toEqual([{ live: 0, with_it: count, members_apart: 0 }]);
}

/** What a notebook membership's row holds, as expectNotebookMember checks it. */
export interface MemberRow {
  /** Its role. */
  role: string;
  /** Whether it has not ended. */
  active: boolean;
  /** The account that wrote it last. */
  writerId: string;
  /** When the account first joined, as the API answered it: a membership given back keeps it. */
  joinedAt?: string;
}

/**
 * notebook_members: userId's one row of the notebook id, not deleted, is
 * want; when it ended, it ended at its last write.
 */
export async function expectNotebookMember(
  db: Database,
  notebookId: string,
  userId: string,
  want: MemberRow
): Promise<void> {
  const rows = await db.query(
    `SELECT role, ended_at IS NULL AS active, updated_by_id = $3 AS by_writer,
            ($4::timestamptz IS NULL OR created_at = $4::timestamptz) AS joined,
            ended_at IS NULL OR ended_at = updated_at AS ended_at_write
       FROM notebook_members WHERE notebook_id = $1 AND user_id = $2 AND deleted_at IS NULL`,
    [notebookId, userId, want.writerId, want.joinedAt ?? null]
  );
  expect(rows).toEqual([{ role: want.role, active: want.active, by_writer: true, joined: true, ended_at_write: true }]);
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

/**
 * notebook_members: userId's memberships of the notebooks of the workspace
 * id, count of them, ended with the account's membership of the workspace,
 * at its time, by its ender, enderId.
 */
export async function expectNotebookMembershipsEndedWith(
  db: Database,
  workspaceId: string,
  userId: string,
  enderId: string,
  count: number
): Promise<void> {
  const rows = await db.query<{ with_it: number; apart: number }>(
    `SELECT count(*) FILTER (WHERE m.ended_at = w.ended_at AND m.updated_at = w.ended_at AND m.updated_by_id = $3)::int AS with_it,
            count(*) FILTER (WHERE m.ended_at IS DISTINCT FROM w.ended_at OR m.updated_at IS DISTINCT FROM w.ended_at
                               OR m.updated_by_id <> $3 OR w.updated_by_id <> $3)::int AS apart
       FROM notebook_members m JOIN notebooks n ON n.id = m.notebook_id
       JOIN workspace_members w ON w.workspace_id = n.workspace_id AND w.user_id = m.user_id
      WHERE n.workspace_id = $1 AND m.user_id = $2 AND m.deleted_at IS NULL`,
    [workspaceId, userId, enderId]
  );
  expect(rows).toEqual([{ with_it: count, apart: 0 }]);
}

/**
 * notebooks: the notebook id has no active admin, and is ownerless of
 * formerOwnerId since that account's membership of its workspace ended,
 * with its own membership of the notebook; its last change stays the one
 * before.
 */
export async function expectOwnerless(db: Database, id: string, formerOwnerId: string): Promise<void> {
  const rows = await db.query(
    `SELECT n.former_owner_id, n.ownerless_since = w.ended_at AS since_the_end, m.ended_at = w.ended_at AS owner_ended,
            n.updated_at < w.ended_at AS changed_before,
            (SELECT count(*)::int FROM notebook_members a WHERE a.notebook_id = n.id AND a.role = 'admin'
               AND a.ended_at IS NULL AND a.deleted_at IS NULL) AS admins
       FROM notebooks n
       JOIN workspace_members w ON w.workspace_id = n.workspace_id AND w.user_id = n.former_owner_id
       JOIN notebook_members m ON m.notebook_id = n.id AND m.user_id = n.former_owner_id
      WHERE n.id = $1 AND n.deleted_at IS NULL`,
    [id]
  );
  expect(rows).toEqual([
    { former_owner_id: formerOwnerId, since_the_end: true, owner_ended: true, changed_before: true, admins: 0 },
  ]);
}

/** notebooks: the notebook id is not deleted, not ownerless, and has an active admin. */
export async function expectOwned(db: Database, id: string): Promise<void> {
  const rows = await db.query(
    `SELECT n.ownerless_since IS NULL AND n.former_owner_id IS NULL AS owned,
            EXISTS (SELECT 1 FROM notebook_members a WHERE a.notebook_id = n.id AND a.role = 'admin'
               AND a.ended_at IS NULL AND a.deleted_at IS NULL) AS has_admin
       FROM notebooks n WHERE n.id = $1 AND n.deleted_at IS NULL`,
    [id]
  );
  expect(rows).toEqual([{ owned: true, has_admin: true }]);
}

/**
 * notebooks and notebook_members: listed is the notebook's row as the
 * ownerless list answered it: ownerless since then, its last change then,
 * as many active members.
 */
export async function expectOwnerlessListed(db: Database, listed: OwnerlessNotebook): Promise<void> {
  const rows = await db.query(
    `SELECT n.name, n.workspace_access, n.former_owner_id, n.ownerless_since = $2::timestamptz AS since,
            n.updated_at = $3::timestamptz AS last_activity,
            (SELECT count(*)::int FROM notebook_members m WHERE m.notebook_id = n.id AND m.ended_at IS NULL
               AND m.deleted_at IS NULL) AS members
       FROM notebooks n WHERE n.id = $1`,
    [listed.id, listed.ownerless_since, listed.last_activity_at]
  );
  expect(rows).toEqual([
    {
      name: listed.name,
      workspace_access: listed.workspace_access,
      former_owner_id: listed.former_owner.user_id,
      since: true,
      last_activity: true,
      members: listed.member_count,
    },
  ]);
}

/** What an audit event's row holds, as expectAuditEvents checks it. */
export interface AuditRow {
  action: NotebookAuditAction;
  notebookId: string;
  notebookName: string;
  formerOwnerId: string;
  actorId: string;
}

/**
 * notebook_audit_events: the events not deleted of the workspace id,
 * oldest first, are want; each written once, by its actor.
 */
export async function expectAuditEvents(db: Database, workspaceId: string, want: AuditRow[]): Promise<void> {
  const rows = await db.query(
    `SELECT action, notebook_id, notebook_name, former_owner_id, created_by_id,
            updated_by_id = created_by_id AND updated_at = created_at AS once
       FROM notebook_audit_events WHERE workspace_id = $1 AND deleted_at IS NULL ORDER BY created_at, id`,
    [workspaceId]
  );
  expect(rows).toEqual(
    want.map((w) => ({
      action: w.action,
      notebook_id: w.notebookId,
      notebook_name: w.notebookName,
      former_owner_id: w.formerOwnerId,
      created_by_id: w.actorId,
      once: true,
    }))
  );
}

/**
 * notebook_audit_events: every event of the workspace id, count of them,
 * is deleted with it, at its time, by its deleter.
 */
export async function expectAuditEventsDeletedWith(db: Database, workspaceId: string, count: number): Promise<void> {
  const rows = await db.query<{ with_it: number; apart: number }>(
    `SELECT count(*) FILTER (WHERE e.deleted_at = w.deleted_at AND e.updated_at = w.deleted_at
                               AND e.updated_by_id = w.updated_by_id)::int AS with_it,
            count(*) FILTER (WHERE e.deleted_at IS DISTINCT FROM w.deleted_at OR e.updated_at IS DISTINCT FROM w.deleted_at
                               OR e.updated_by_id <> w.updated_by_id)::int AS apart
       FROM notebook_audit_events e JOIN workspaces w ON w.id = e.workspace_id WHERE w.id = $1`,
    [workspaceId]
  );
  expect(rows).toEqual([{ with_it: count, apart: 0 }]);
}
