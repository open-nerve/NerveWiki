import type { NotebookRole } from "../services/notebook.service";
import type { NodeRestored, TrashItem, TreeNode } from "../services/page.service";
import { titleKey } from "../lib/title-key";
import { extensionOf } from "../lib/upload-name";
import { json, notebookJSON, problem, type Answer } from "./fakes";
import type { Person } from "./page-server";

// The trash of the page server (page-server.ts; M8/P2), as the server keeps
// it: a deletion puts the node and the nodes under it in the trash, an item
// of its root. Listed newer first, limit of them a page (50 without one),
// below a cursor of the last listed; an item expired is listed no more.
// An item, a restore read the codes in the server's order: a node that is
// neither alive nor in the trash, or Plans gone, 404 page.not_found; then a
// reader's restore 403 forbidden; then a node alive 409 page.not_in_trash,
// one deleted with a page above it 409 page.not_trash_root naming its
// item's root in root_id, an item expired 409 page.restore_expired; then a
// restore past the unit's bounds (its nodes, its history, more than one
// page past its pages or their bytes) 422 page.restore_too_large; an item
// the purge took part of (purged) 409 page.trash_changed; while busy, 503
// server_busy; past its links (more than one page), 422 again. A restore
// puts the item back under its parent at its place, or last at the root
// when its parent is in the trash or its pages would be too deep, numbered
// where a sibling there has its name. While trashDown is set, the list and
// the items cannot be read; trashWaits holds their reads, restoresHeld
// the restores (released with the page server's release).

/** idOf is the node's id in a request's path, /api/v0/nodes/{id}… */
function idOf(request: Request): string {
  return new URL(request.url).pathname.split("/")[4] ?? "";
}

/** compare orders two strings as the server's keys are: by their code units. */
function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/**
 * A FakeTrashItem is a deletion's root in the trash, with the nodes deleted
 * with it, its members, root first, each at its place among the nodes as
 * it was; when and by whom it was deleted, and what a test sets: expired,
 * purged (its content or file taken), its history's rows and its links,
 * which the restore's bounds count. Where it was is its parent's line as
 * it is now, through the pages alive and those in the trash.
 */
export type FakeTrashItem = {
  members: TreeNode[];
  places: number[];
  deletedAt: string;
  deletedBy: Person;
  expired: boolean;
  purged: boolean;
  history: number;
  links: number;
};

/** The unit's bounds a restore is held to, the server's (page/domain/trash.go). */
const unit = { nodes: 10_000, history: 400_000, pages: 100, bytes: 8 << 20, links: 20_000 };

/**
 * trashed is the item a deletion of the nodes gone makes of nodes as they were, the server's nth: deleted by by, a
 * minute after the one before (a test sets another time, within the same minute say).
 */
export function trashed(nodes: readonly TreeNode[], gone: TreeNode[], nth: number, by: Person): FakeTrashItem {
  const at = new Date(Date.UTC(2026, 9, 4, 8, nth)).toISOString();
  return {
    members: gone,
    places: gone.map((each) => nodes.indexOf(each)),
    deletedAt: at,
    deletedBy: by,
    expired: false,
    purged: false,
    history: gone.length * 2,
    links: 0,
  };
}

/** levelOf is the level of node in item, the root's 1. */
function levelOf(item: FakeTrashItem, node: TreeNode): number {
  const parent = item.members.find((each) => each.id === node.parent_id);
  return node === item.members[0] || parent === undefined ? 1 : 1 + levelOf(item, parent);
}

/**
 * What the trash's reads wait for while a test holds them: the list's first page's, a next page's (after a
 * cursor), an item's.
 */
export type TrashWaits = { first?: Promise<void>; next?: Promise<void>; item?: Promise<void> };

/** What of the page server the trash reads and changes. */
type TrashState = {
  sent: string[];
  nodes: TreeNode[];
  contents: Map<string, { content: string; revision: number }>;
  trash: Map<string, FakeTrashItem>;
  notebookGone: boolean;
  restoresBusy: boolean;
  restoresHeld: boolean;
  trashDown: boolean;
  trashWaits: TrashWaits;
  held: (() => void)[];
};

/** An attachment's type and size, as the server answers them. */
type AssetOf = (node: TreeNode) => { mime: string; byte_size: number };

/** trashedNodes are the nodes of server's trash, of every item. */
function trashedNodes(server: TrashState): TreeNode[] {
  return [...server.trash.values()].flatMap((item) => item.members);
}

/** ancestorsOf is where item was: its root's parent's line as it is now, through the pages alive and in the trash. */
function ancestorsOf(server: TrashState, item: FakeTrashItem): { id: string; name: string }[] {
  const ancestors: { id: string; name: string }[] = [];
  const known = [...server.nodes, ...trashedNodes(server)];
  for (let at = (item.members[0] as TreeNode).parent_id; at !== null;) {
    const page = known.find((node) => node.id === at);
    if (page === undefined) {
      break;
    }
    ancestors.unshift({ id: page.id, name: page.name });
    at = page.parent_id;
  }
  return ancestors;
}

/**
 * placementOf is where a restore puts item, by where it was (ancestors): under its parent, or at the root when it
 * was there, when its parent is in the trash, or when its pages would nest too deep under its parent.
 */
function placementOf(
  server: TrashState,
  item: FakeTrashItem,
  ancestors = ancestorsOf(server, item)
): TrashItem["restore_to"] {
  const pages = item.members.filter((node) => node.kind === "page");
  const height = Math.max(0, ...pages.map((page) => levelOf(item, page)));
  const parent = ancestors.at(-1);
  const alive = parent !== undefined && server.nodes.some((node) => node.id === parent.id);
  return parent === undefined
    ? { parent_id: null, reason: "original" }
    : !alive
      ? { parent_id: null, reason: "parent_in_trash" }
      : ancestors.length + height > 10
        ? { parent_id: null, reason: "too_deep" }
        : { parent_id: parent.id, reason: "original" };
}

/**
 * trashItemOf is the trash item of the root id, item, as the server answers it: where it was, its parent's line
 * now, through the pages alive and those in the trash, and where a restore puts it by that line.
 */
function trashItemOf(server: TrashState, id: string, item: FakeTrashItem, assetOf: AssetOf): TrashItem {
  const root = item.members[0] as TreeNode;
  const pages = item.members.filter((node) => node.kind === "page");
  const ancestors = ancestorsOf(server, item);
  const restoreTo = placementOf(server, item, ancestors);
  const asset = root.kind === "asset" ? assetOf(root) : undefined;
  return {
    id,
    notebook_id: root.notebook_id,
    kind: root.kind,
    name: root.name,
    ancestors,
    deleted_at: item.deletedAt,
    deleted_by: item.deletedBy,
    restorable_until: new Date(Date.parse(item.deletedAt) + 5_180_400_000).toISOString(),
    pages: pages.length,
    assets: item.members.length - pages.length,
    restore_to: restoreTo,
    revision: root.kind === "page" && !item.purged ? (server.contents.get(id)?.revision ?? 1) : null,
    asset: asset === undefined || item.purged ? null : { mime: asset.mime, byte_size: asset.byte_size },
  };
}

/**
 * putBack restores the trash item of the root id, item, of server: under its parent at its place, or last at the
 * root, numbered where a sibling there has its name, an attachment's number before its extension.
 */
export function putBack(server: TrashState, id: string, item: FakeTrashItem): NodeRestored {
  const placed = placementOf(server, item);
  const root = item.members[0] as TreeNode;
  const siblings = server.nodes.filter((node) => node.parent_id === placed.parent_id);
  const extension = root.kind === "asset" ? extensionOf(root.name) : "";
  let name = root.name;
  for (let n = 2; siblings.some((node) => titleKey(node.name) === titleKey(name)); n++) {
    name = `${root.name.slice(0, root.name.length - extension.length)} ${n.toString()}${extension}`;
  }
  let nodes = server.nodes;
  item.members.forEach((node, i) => {
    const at =
      placed.parent_id === root.parent_id ? Math.min(item.places[i] ?? nodes.length, nodes.length) : nodes.length;
    nodes = [
      ...nodes.slice(0, at),
      i === 0 ? { ...root, parent_id: placed.parent_id, name } : node,
      ...nodes.slice(at),
    ];
  });
  server.nodes = nodes;
  server.trash.delete(id);
  return { id, kind: root.kind, name, renamed: name !== root.name, placed, changeset_id: `restore-${id}` };
}

/** The answers to the trash of server: its list, an item and a restore, which changes it. */
export function trashRoutes(server: TrashState, role: NotebookRole, assetOf: AssetOf): Record<string, Answer> {
  // The trash item of the node id, as getTrashItem and restoreNode find it, or the refusal.
  const found = (id: string): { item: FakeTrashItem } | Response => {
    if (server.notebookGone) {
      return problem(404, "page.not_found");
    }
    if (server.nodes.some((node) => node.id === id)) {
      return problem(409, "page.not_in_trash");
    }
    const item = server.trash.get(id);
    if (item === undefined) {
      const root = [...server.trash].find(([, each]) => each.members.some((node) => node.id === id))?.[0];
      return root === undefined
        ? problem(404, "page.not_found")
        : problem(409, "page.not_trash_root", { root_id: root });
    }
    return item.expired ? problem(409, "page.restore_expired") : { item };
  };
  const nameOf = (id: string) => [...server.nodes, ...trashedNodes(server)].find((node) => node.id === id)?.name ?? id;
  return {
    [`GET /api/v0/notebooks/${notebookJSON.id}/trash`]: async (request) => {
      const query = new URL(request.url).searchParams;
      const cursor = query.get("cursor");
      const limit = Number(query.get("limit") ?? "50");
      server.sent.push(`GET trash${cursor === null ? "" : ` below ${nameOf(cursor.split(" ")[1] ?? "")}`}`);
      await (cursor === null ? server.trashWaits.first : server.trashWaits.next);
      if (server.trashDown) {
        return problem(500, "internal");
      }
      if (server.notebookGone) {
        return problem(404, "notebook.not_found");
      }
      // The cursor is the last item listed, by when it was deleted and its id, as the server's.
      const listed = [...server.trash]
        .filter(([id, item]) => !item.expired && (cursor === null || `${item.deletedAt} ${id}` < cursor))
        .toSorted(([a, x], [b, y]) => compare(`${y.deletedAt} ${b}`, `${x.deletedAt} ${a}`));
      const data = listed.slice(0, limit);
      const last = data.at(-1);
      return json({
        data: data.map(([id, item]) => trashItemOf(server, id, item, assetOf)),
        next_cursor: listed.length > limit && last !== undefined ? `${last[1].deletedAt} ${last[0]}` : null,
      });
    },
    "GET /api/v0/nodes/*/trash-item": async (request) => {
      const id = idOf(request);
      server.sent.push(`GET trash item ${nameOf(id)}`);
      await server.trashWaits.item;
      if (server.trashDown) {
        return problem(500, "internal");
      }
      const answer = found(id);
      return answer instanceof Response ? answer : json(trashItemOf(server, id, answer.item, assetOf));
    },
    "POST /api/v0/nodes/*/restore": async (request) => {
      const id = idOf(request);
      server.sent.push(`RESTORE ${nameOf(id)}`);
      if (server.restoresHeld) {
        await new Promise<void>((resolve) => server.held.push(resolve));
      }
      const answer = found(id);
      if (answer instanceof Response && answer.status === 404) {
        return answer;
      }
      if (role === "reader") {
        return problem(403, "forbidden");
      }
      if (answer instanceof Response) {
        return answer;
      }
      const { item } = answer;
      const pages = item.members.filter((node) => node.kind === "page");
      const bytes = pages.reduce(
        (sum, page) => sum + new TextEncoder().encode(server.contents.get(page.id)?.content ?? `${page.name}\n`).length,
        0
      );
      if (
        item.members.length > unit.nodes ||
        (pages.length > 1 && (pages.length > unit.pages || bytes > unit.bytes)) ||
        item.history > unit.history
      ) {
        return problem(422, "page.restore_too_large");
      }
      if (item.purged) {
        return problem(409, "page.trash_changed");
      }
      if (server.restoresBusy) {
        return problem(503, "server_busy", {}, { "Retry-After": "2" });
      }
      if (pages.length > 1 && item.links > unit.links) {
        return problem(422, "page.restore_too_large");
      }
      return json(putBack(server, id, item));
    },
  };
}
