import type { NotebookRole } from "../services/notebook.service";
import type { Changeset, ChangesetChange, RevertConflict, TreeNode } from "../services/page.service";
import { json, notebookJSON, problem, userJSON, type Answer } from "./fakes";
import type { FakeTrashItem } from "./trash-server";
import type { Person } from "./page-server";

// The history of the page server (page-server.ts; M8/P3), as the server
// keeps it: the changesets a test logs, each with what it changed by node.
// Listed newer first by when each was made, then by id, limit of them a
// page (50 without one), below a cursor of the last listed. The reads and
// the revert answer the codes in the server's order:
//
// - the log: a cursor it cannot read 400 bad_request; then Plans gone 404
//   notebook.not_found; then a page_id of no node of Plans, alive or in
//   the trash, 404 page.not_found; then a client none of web, api, cli,
//   mcp and mcp:<name> 422 on client, a limit outside 1–100 422 on limit;
//   an id, an account's or a node's, read in either case, as uuid.Parse
//   reads it;
// - a changeset and its changes: one not logged, or Plans gone, 404
//   page.changeset_not_found; its changes by node id, a node the purge
//   took (neither alive nor in the trash) left out, as the server's join
//   leaves it; each node's name and its parents' as they are now;
// - a revert: a message empty, past 4,096 bytes or with a NUL 422 on
//   message; then 404 page.changeset_not_found; then a reader's 403
//   forbidden; then a changeset trimmed 409 page.changeset_trimmed; then
//   one too large (tooLarge) 422 page.revert_too_large; then its conflicts
//   409 page.revert_conflict, by reason in the contract's order, then by
//   node id, the first 100 and their count; then one a version of which
//   the purge took (purged) 409 page.changeset_trimmed, a version it would
//   not write too; then one that changed nothing (unchanged: an edit
//   session that wrote its bytes back) changed false, no changeset,
//   nothing to write, so nothing parsed and no guard run; while busy
//   (revertsBusy) 503 server_busy; then a page someone edits (lockedBy)
//   409 page.locked naming them; else a
//   revert of it, by Ada from the web, the newest of the log and of its
//   reverts, which does to the tree what undo does: a node created or
//   restored deleted again, one deleted restored, neither writing a
//   version.
//
// While historyDown is set, the log and a changeset cannot be read;
// historyWaits holds their reads, revertsHeld the reverts (released with
// the page server's release).

/** ada is the account the tests sign in as (userJSON). */
const ada: Person = { user_id: userJSON.id, display_name: userJSON.display_name };

/** A place of a node a change has: its parent (null: the notebook's top level) and its name there. */
type FakePlace = { parent: string | null; name: string };

/** A FakeChange is what a changeset did to the node node: where it was before and after, and the version it wrote. */
export type FakeChange = {
  node: string;
  kind: ChangesetChange["kind"];
  before: FakePlace | null;
  after: FakePlace | null;
  content: { base: number | null; revision: number } | null;
};

/**
 * A FakeChangeset is a changeset of Plans' history: who made it, from
 * where and when, why, what it reverts (an id the log has not: purged) and
 * what reverted it, newest first; whether the purge trimmed it, and what
 * it changed, its counts by them unless counts says otherwise. What a
 * test sets of its revert: its conflicts, too large, too large for a
 * background job too (jobTooLarge), purged, a page locked, unchanged, and
 * what it does to the tree (undo).
 */
export type FakeChangeset = {
  id: string;
  /** What the server's log of requests (sent) calls it. */
  label: string;
  kind: Changeset["kind"];
  client: string;
  message: string | null;
  by: Person;
  at: string;
  updatedAt: string;
  reverts: string | null;
  revertedBy: string[];
  trimmed: boolean;
  changes: FakeChange[];
  counts?: { nodes: number; contents: number };
  conflicts: RevertConflict[];
  tooLarge: boolean;
  jobTooLarge: boolean;
  purged: boolean;
  lockedBy?: { page: string; holder: Person };
  unchanged: boolean;
  undo?: () => void;
};

let logged = 0;

/** changesetId is the id of the changeset n of the tests: a UUID, as the history's addresses take. */
export function changesetId(n: number): string {
  return `0199a2b4-0000-7000-8000-0000000d${n.toString().padStart(4, "0")}`;
}

/**
 * fakeChangeset is a changeset of Plans' history, the nth the tests made: an edit by Ada from the web, a minute after
 * the one before, unless what says otherwise.
 */
export function fakeChangeset(what: Partial<FakeChangeset> = {}): FakeChangeset {
  const n = ++logged;
  const at = new Date(Date.UTC(2026, 9, 5, 8, n % 60, 0) + Math.floor(n / 60) * 3_600_000).toISOString();
  const id = what.id ?? changesetId(n);
  return {
    id,
    label: id,
    kind: "edit",
    client: "web",
    message: null,
    by: ada,
    at,
    updatedAt: what.at ?? at,
    reverts: null,
    revertedBy: [],
    trimmed: false,
    changes: [],
    conflicts: [],
    tooLarge: false,
    jobTooLarge: false,
    purged: false,
    unchanged: false,
    ...what,
  };
}

/** placeOf is the place of node as the tree has it now: its parent and its name. */
function placeOf(node: TreeNode): FakePlace {
  return { parent: node.parent_id, name: node.name };
}

/** created is the change that created node where it is, a page's first version with it. */
export function created(node: TreeNode): FakeChange {
  const content = node.kind === "page" ? { base: null, revision: 1 } : null;
  return { node: node.id, kind: "created", before: null, after: placeOf(node), content };
}

/** deleted is the change that deleted node from where it is. */
export function deleted(node: TreeNode): FakeChange {
  return { node: node.id, kind: "deleted", before: placeOf(node), after: null, content: null };
}

/** restored is the change that restored node from the trash to where it is. */
export function restored(node: TreeNode): FakeChange {
  return { node: node.id, kind: "restored", before: null, after: placeOf(node), content: null };
}

/** renamed is the change that renamed node from from to its name now. */
export function renamed(node: TreeNode, from: string): FakeChange {
  return {
    node: node.id,
    kind: "renamed",
    before: { ...placeOf(node), name: from },
    after: placeOf(node),
    content: null,
  };
}

/** moved is the change that moved node from under from (null: the top level) to where it is. */
export function moved(node: TreeNode, from: string | null): FakeChange {
  return {
    node: node.id,
    kind: "moved",
    before: { ...placeOf(node), parent: from },
    after: placeOf(node),
    content: null,
  };
}

/** reordered is the change that put node elsewhere among its siblings. */
export function reordered(node: TreeNode): FakeChange {
  return { node: node.id, kind: "reordered", before: placeOf(node), after: placeOf(node), content: null };
}

/** wrote is the change that wrote node's content at revision, on base. */
export function wrote(node: TreeNode, base: number, revision: number): FakeChange {
  return { node: node.id, kind: "content", before: null, after: null, content: { base, revision } };
}

/** The kind of what a revert does to a node of each kind of change: the change's reverse. */
const reverse: Record<ChangesetChange["kind"], ChangesetChange["kind"]> = {
  created: "deleted",
  restored: "deleted",
  deleted: "restored",
  moved: "moved",
  renamed: "renamed",
  reordered: "reordered",
  content: "content",
};

/** The reasons of the conflicts, in the order the contract lists them in (common.yaml's RevertConflict). */
const reasons: RevertConflict["reason"][] = [
  "changed",
  "expired",
  "parent_gone",
  "children_added",
  "title_taken",
  "cycle",
  "too_deep",
];

/** orderedConflicts are conflicts as the server lists them: by reason in the contract's order, then by node id. */
export function orderedConflicts(conflicts: readonly RevertConflict[]): RevertConflict[] {
  return conflicts.toSorted(
    (a, b) => reasons.indexOf(a.reason) - reasons.indexOf(b.reason) || compare(a.node_id, b.node_id)
  );
}

/** sameId tells whether two ids are one, in either case. */
function sameId(a: string, b: string): boolean {
  return a.toLowerCase() === b.toLowerCase();
}

/** What of the page server the history reads and changes. */
type HistoryState = {
  sent: string[];
  nodes: TreeNode[];
  trash: Map<string, FakeTrashItem>;
  notebookGone: boolean;
  changesets: FakeChangeset[];
  revertsBusy: boolean;
  revertsHeld: boolean;
  historyDown: boolean;
  historyWaits: HistoryWaits;
  held: (() => void)[];
};

/** What the history's reads wait for while a test holds them: the log's first page's, a next page's, a changeset's. */
export type HistoryWaits = { first?: Promise<void>; next?: Promise<void>; changeset?: Promise<void> };

/** compare orders two strings as the server's keys are: by their code units. */
function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/** A client the log is filtered by: one of the web, the API, the CLI, every MCP client, or one MCP client by name. */
const clientPattern = /^(?:web|api|cli|mcp|mcp:[^\p{Cc}]{1,128})$/u;

/** idOf is the id of the changeset of request's path. */
function idOf(request: Request): string {
  return new URL(request.url).pathname.split("/")[4] ?? "";
}

/** keyOf is where changeset is in the log, newer first, then by id, as its index orders them: a cursor is one. */
function keyOf(changeset: FakeChangeset): string {
  return `${changeset.at} ${changeset.id}`;
}

/** The answers to the history of server: its log, a changeset and its changes, and a revert, which changes it. */
export function historyRoutes(server: HistoryState, role: NotebookRole): Record<string, Answer> {
  const nodeOf = (id: string | null) =>
    id === null
      ? undefined
      : (server.nodes.find((node) => sameId(node.id, id)) ??
        [...server.trash.values()].flatMap((item) => item.members).find((node) => sameId(node.id, id)));
  const changesetOf = (id: string) =>
    server.notebookGone ? undefined : server.changesets.find((changeset) => changeset.id === id);
  const labelOf = (id: string) => changesetOf(id)?.label ?? id;
  // Its changes as the server answers them now, by node id: those of nodes the purge took left out.
  const changesOf = (changeset: FakeChangeset): ChangesetChange[] =>
    changeset.changes
      .toSorted((a, b) => compare(a.node, b.node))
      .flatMap((change) => {
        const node = nodeOf(change.node);
        if (node === undefined) {
          return [];
        }
        const state = (place: FakePlace | null) =>
          place === null
            ? null
            : { parent_id: place.parent, parent_name: nodeOf(place.parent)?.name ?? null, name: place.name };
        return [
          {
            node: { id: node.id, kind: node.kind, name: node.name, alive: server.nodes.includes(node) },
            kind: change.kind,
            before: state(change.before),
            after: state(change.after),
            content:
              change.content === null
                ? null
                : { base_revision: change.content.base, revision: change.content.revision },
          },
        ];
      });
  const answered = (changeset: FakeChangeset): Changeset => {
    const reverts = changeset.reverts === null ? undefined : changesetOf(changeset.reverts);
    const items = changeset.changes.filter((change) => change.before !== null || change.after !== null);
    return {
      id: changeset.id,
      notebook_id: notebookJSON.id,
      kind: changeset.kind,
      client: changeset.client,
      message: changeset.message,
      created_by: changeset.by,
      created_at: changeset.at,
      updated_at: changeset.updatedAt,
      reverts:
        reverts === undefined
          ? null
          : { id: reverts.id, kind: reverts.kind, created_by: reverts.by, created_at: reverts.at },
      reverted_by: changeset.revertedBy.slice(0, 100),
      trimmed: changeset.trimmed,
      node_count: Math.min(1000, changeset.counts?.nodes ?? items.length),
      content_count: Math.min(
        1000,
        changeset.counts?.contents ?? changeset.changes.filter((change) => change.content !== null).length
      ),
      changes: changesOf(changeset).slice(0, 3),
    };
  };
  return {
    [`GET /api/v0/notebooks/${notebookJSON.id}/changesets`]: async (request) => {
      const query = new URL(request.url).searchParams;
      const cursor = query.get("cursor");
      const [actor, client, page] = [query.get("actor"), query.get("client"), query.get("page_id")];
      const limit = Number(query.get("limit") ?? "50");
      const filters = [actor === null ? "" : ` by ${actor}`, client === null ? "" : ` from ${client}`];
      filters.push(page === null ? "" : ` of ${nodeOf(page)?.name ?? page}`);
      server.sent.push(
        `GET changesets${filters.join("")}${cursor === null ? "" : ` below ${labelOf(cursor.split(" ")[1] ?? "")}`}`
      );
      await (cursor === null ? server.historyWaits.first : server.historyWaits.next);
      if (server.historyDown) {
        return problem(500, "internal_error");
      }
      if (cursor !== null && !/^\S+ \S+$/.test(cursor)) {
        return problem(400, "bad_request");
      }
      if (server.notebookGone) {
        return problem(404, "notebook.not_found");
      }
      if (page !== null && nodeOf(page) === undefined) {
        return problem(404, "page.not_found");
      }
      if (client !== null && !clientPattern.test(client)) {
        return problem(422, "validation_failed", {
          errors: [{ field: "client", code: "invalid_format", message: "" }],
        });
      }
      if (!Number.isInteger(limit) || limit < 1 || limit > 100) {
        return problem(422, "validation_failed", { errors: [{ field: "limit", code: "out_of_range", message: "" }] });
      }
      const listed = server.changesets
        .filter(
          (changeset) =>
            (cursor === null || keyOf(changeset) < cursor) &&
            (actor === null || sameId(changeset.by.user_id, actor)) &&
            (client === null ||
              changeset.client === client ||
              (client === "mcp" && changeset.client.startsWith("mcp:"))) &&
            (page === null || changeset.changes.some((change) => sameId(change.node, page)))
        )
        .toSorted((a, b) => compare(keyOf(b), keyOf(a)));
      const data = listed.slice(0, limit);
      const last = data.at(-1);
      return json({
        data: data.map(answered),
        next_cursor: listed.length > limit && last !== undefined ? keyOf(last) : null,
      });
    },
    "GET /api/v0/changesets/*": async (request) => {
      const id = idOf(request);
      server.sent.push(`GET changeset ${labelOf(id)}`);
      await server.historyWaits.changeset;
      if (server.historyDown) {
        return problem(500, "internal_error");
      }
      const changeset = changesetOf(id);
      if (changeset === undefined) {
        return problem(404, "page.changeset_not_found");
      }
      const changes = changeset.changes.filter((change) => change.before !== null && change.after !== null);
      return json({
        ...answered(changeset),
        renames: changes.some((change) => change.before?.name !== change.after?.name),
        moves: changes.some((change) => change.before?.parent !== change.after?.parent),
      });
    },
    "GET /api/v0/changesets/*/changes": (request) => {
      const id = idOf(request);
      const query = new URL(request.url).searchParams;
      const cursor = query.get("cursor");
      const limit = Number(query.get("limit") ?? "50");
      server.sent.push(
        `GET changes ${labelOf(id)}${cursor === null ? "" : ` after ${nodeOf(cursor)?.name ?? cursor}`}`
      );
      if (server.historyDown) {
        return problem(500, "internal_error");
      }
      const changeset = changesetOf(id);
      if (changeset === undefined) {
        return problem(404, "page.changeset_not_found");
      }
      if (!Number.isInteger(limit) || limit < 1 || limit > 100) {
        return problem(422, "validation_failed", { errors: [{ field: "limit", code: "out_of_range", message: "" }] });
      }
      const listed = changesOf(changeset).filter((change) => cursor === null || change.node.id > cursor);
      const data = listed.slice(0, limit);
      return json({ data, next_cursor: listed.length > limit ? (data.at(-1)?.node.id ?? null) : null });
    },
    "POST /api/v0/changesets/*/revert": async (request) => {
      const id = idOf(request);
      const text = await request.clone().text();
      const message = text === "" ? undefined : ((JSON.parse(text) as { message?: string }).message ?? undefined);
      server.sent.push(`REVERT ${labelOf(id)}${message === undefined ? "" : ` ${JSON.stringify(message)}`}`);
      if (server.revertsHeld) {
        await new Promise<void>((resolve) => server.held.push(resolve));
      }
      const bytes = message === undefined ? 0 : new TextEncoder().encode(message).length;
      if (message !== undefined && (bytes === 0 || bytes > 4096 || message.includes("\0"))) {
        const code = bytes === 0 ? "too_short" : bytes > 4096 ? "too_long" : "invalid_format";
        return problem(422, "validation_failed", { errors: [{ field: "message", code, message: "" }] });
      }
      const changeset = changesetOf(id);
      if (changeset === undefined) {
        return problem(404, "page.changeset_not_found");
      }
      if (role === "reader") {
        return problem(403, "forbidden");
      }
      if (changeset.trimmed) {
        return problem(409, "page.changeset_trimmed");
      }
      if (changeset.tooLarge) {
        return problem(422, "page.revert_too_large");
      }
      if (changeset.conflicts.length > 0) {
        return problem(409, "page.revert_conflict", {
          conflicts: orderedConflicts(changeset.conflicts).slice(0, 100),
          conflict_count: changeset.conflicts.length,
        });
      }
      if (changeset.purged) {
        return problem(409, "page.changeset_trimmed");
      }
      if (changeset.unchanged) {
        return json({ changeset_id: null, changed: false });
      }
      if (server.revertsBusy) {
        return problem(503, "server_busy", {}, { "Retry-After": "2" });
      }
      if (changeset.lockedBy !== undefined) {
        const { page, holder } = changeset.lockedBy;
        return problem(409, "page.locked", { lock: { page_id: page, ...holder } });
      }
      changeset.undo?.();
      const revert = fakeChangeset({
        kind: "revert",
        message: message ?? null,
        reverts: changeset.id,
        changes: changeset.changes.map((change) => ({
          ...change,
          kind: reverse[change.kind],
          before: change.after,
          after: change.before,
          content:
            change.content === null || reverse[change.kind] === "deleted" || reverse[change.kind] === "restored"
              ? null
              : { base: change.content.revision, revision: change.content.revision + 1 },
        })),
      });
      revert.label = `revert of ${changeset.label}`;
      changeset.revertedBy = [revert.id, ...changeset.revertedBy];
      server.changesets = [...server.changesets, revert];
      return json({ changeset_id: revert.id, changed: true });
    },
  };
}
