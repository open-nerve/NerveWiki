import type { NotebookRole } from "../services/notebook.service";
import type { NodeMove, PageView, TreeNode } from "../services/page.service";
import { json, notebookJSON, problem, signedInApp, type Answer } from "./fakes";

/** pageNode is the page n of Plans, titled name, under parent (none: at the root). */
export function pageNode(n: number, name: string, parent?: TreeNode): TreeNode {
  return {
    id: `0199a2b4-0000-7000-8000-000000000${(100 + n).toString()}`,
    notebook_id: notebookJSON.id,
    parent_id: parent?.id ?? null,
    kind: "page",
    name,
    created_at: "2026-10-03T08:00:00Z",
    updated_at: "2026-10-03T08:00:00Z",
  };
}

export const guide = pageNode(1, "Guide");
export const install = pageNode(2, "Install", guide);
export const linux = pageNode(3, "Linux", install);
export const notes = pageNode(4, "Notes");

/** pagePath is the address of the page id of Plans. */
export const pagePath = (id: string) => `/lab/notebooks/${notebookJSON.id}/pages/${id}`;

type PageServerOptions = {
  /** Ada's role in Plans. */
  role?: NotebookRole;
  /** Plans' tree, parents before their children. */
  nodes?: TreeNode[];
  /** Answers instead of the server's, by "METHOD /path" as byRoute takes them. */
  answers?: Record<string, Answer>;
};

/**
 * pageServer is the server of Plans' pages as Ada (userJSON) sees them
 * (M4/P5 design 3.4): Lab's one notebook Plans, its tree nodes (by default
 * Guide > Install > Linux, and Notes), each page's reading view its title
 * in a paragraph at revision 1 unless views says otherwise. While
 * nodesDown or viewsDown is set, the tree or the views cannot be read.
 *
 * It writes the tree as the server does where the pages look: a new page
 * goes last under its parent, a title a sibling has (by lower case) is 409
 * page.title_taken, a move takes the page to its place, a deletion takes
 * the page's subtree; it checks no permission, which a test answers
 * through answers. The test changes the state as another tab would; what
 * went out is in sent.
 */
export function pageServer({
  role = "admin",
  nodes = [guide, install, linux, notes],
  answers = {},
}: PageServerOptions = {}) {
  const server = {
    sent: [] as string[],
    nodes,
    views: new Map<string, PageView>(),
    nodesDown: false,
    viewsDown: false,
  };
  const app = signedInApp({
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [{ ...notebookJSON, role }] }),
    [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: () => {
      server.sent.push("GET nodes");
      return server.nodesDown ? Promise.reject(new TypeError("offline")) : json({ data: server.nodes });
    },
    "GET /api/v0/pages/*/view": (request) => {
      const id = new URL(request.url).pathname.split("/")[4] ?? "";
      const page = server.nodes.find((node) => node.id === id);
      server.sent.push(`GET view ${page?.name}`);
      if (server.viewsDown) {
        return Promise.reject(new TypeError("offline"));
      }
      return page === undefined
        ? problem(404, "page.not_found")
        : json(server.views.get(id) ?? { html: `<p>${page.name}</p>`, revision: 1 });
    },
    ...writeRoutes(server),
    ...answers,
  });
  return Object.assign(server, { app });
}

let created = 50;

/** The writes' answers of server, which change it. */
function writeRoutes(server: { sent: string[]; nodes: TreeNode[] }): Record<string, Answer> {
  const taken = (parent: string | null, title: string, except?: string) =>
    server.nodes.some(
      (node) => node.parent_id === parent && node.id !== except && node.name.toLowerCase() === title.toLowerCase()
    );
  /**
   * place puts node right after after among its siblings (null: first;
   * undefined: last): the client orders siblings by the list, which here
   * need not be in preorder.
   */
  const place = (node: TreeNode, after: string | null | undefined) => {
    const others = server.nodes.filter((each) => each.id !== node.id);
    const at =
      after === undefined ? others.length : after === null ? 0 : others.findIndex((each) => each.id === after) + 1;
    server.nodes = [...others.slice(0, at), node, ...others.slice(at)];
  };
  return {
    [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: async (request) => {
      const { parent_id, title } = (await request.clone().json()) as { parent_id: string | null; title: string };
      server.sent.push(`POST ${title} under ${server.nodes.find((n) => n.id === parent_id)?.name ?? "root"}`);
      if (taken(parent_id, title)) {
        return problem(409, "page.title_taken");
      }
      const node: TreeNode = { ...pageNode(created++, title), parent_id };
      server.nodes = [...server.nodes, node];
      place(node, undefined);
      return json(
        {
          ...node,
          ancestors: [],
          revision: 1,
          byte_size: 0,
          content_updated_at: node.created_at,
          content_updated_by: "",
        },
        201
      );
    },
    "PATCH /api/v0/nodes/*": async (request) => {
      const { name } = (await request.clone().json()) as { name: string };
      const node = server.nodes.find((each) => each.id === idOf(request));
      server.sent.push(`PATCH ${node?.name} to ${name}`);
      if (node === undefined) {
        return problem(404, "page.not_found");
      }
      if (name.includes("/")) {
        return problem(422, "validation_failed", { errors: [{ field: "name", code: "invalid_format", message: "/" }] });
      }
      if (taken(node.parent_id, name, node.id)) {
        return problem(409, "page.title_taken");
      }
      server.nodes = server.nodes.map((each) => (each === node ? { ...node, name } : each));
      return json({ ...node, name });
    },
    "POST /api/v0/nodes/*/move": async (request) => {
      const move = (await request.clone().json()) as NodeMove;
      const node = server.nodes.find((each) => each.id === idOf(request));
      const name = (id: string | null | undefined) =>
        id === undefined ? "last" : id === null ? "first" : (server.nodes.find((n) => n.id === id)?.name ?? id);
      server.sent.push(
        `MOVE ${node?.name} under ${move.parent_id === null ? "root" : name(move.parent_id)}, after ${name(move.after_id)}`
      );
      if (node === undefined) {
        return problem(404, "page.not_found");
      }
      const moved = { ...node, parent_id: move.parent_id };
      server.nodes = server.nodes.map((each) => (each === node ? moved : each));
      place(moved, move.after_id);
      return json(moved);
    },
    "DELETE /api/v0/nodes/*": (request) => {
      const node = server.nodes.find((each) => each.id === idOf(request));
      server.sent.push(`DELETE ${node?.name}`);
      if (node === undefined) {
        return problem(404, "page.not_found");
      }
      server.nodes = server.nodes.filter((each) => each !== node && !isUnder(server.nodes, each, node.id));
      return new Response(null, { status: 204 });
    },
  };
}

/** idOf is the node id in a request's path, /api/v0/nodes/{id}… */
function idOf(request: Request): string {
  return new URL(request.url).pathname.split("/")[4] ?? "";
}

/** isUnder tells whether node is below the page id in nodes. */
function isUnder(nodes: readonly TreeNode[], node: TreeNode, id: string): boolean {
  for (let parent = node.parent_id; parent !== null;) {
    if (parent === id) {
      return true;
    }
    parent = nodes.find((each) => each.id === parent)?.parent_id ?? null;
  }
  return false;
}
