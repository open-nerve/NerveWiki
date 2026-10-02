import type { NotebookRole } from "../services/notebook.service";
import type { PageView, TreeNode } from "../services/page.service";
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
 * nodesDown or viewsDown is set, the tree or the views cannot be read. The
 * test changes the state as another tab would; what went out is in sent.
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
    ...answers,
  });
  return Object.assign(server, { app });
}
