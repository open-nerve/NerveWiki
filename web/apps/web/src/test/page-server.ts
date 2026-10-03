import type { NotebookRole } from "../services/notebook.service";
import type { EditLock, NodeMove, PageContent, PageView, TaskToggle, TreeNode } from "../services/page.service";
import { json, notebookJSON, problem, signedInApp, userJSON, type Answer } from "./fakes";

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

/** Someone who holds a lock, or ended a session: their id and name. */
export type Person = { user_id: string; display_name: string };

/** ada is the account the tests sign in as (userJSON); bob is another member. */
export const ada: Person = { user_id: userJSON.id, display_name: userJSON.display_name };
export const bob: Person = { user_id: "0199a2b4-0000-7000-8000-000000000002", display_name: "Bob" };

/**
 * A FakeSession is an edit session of the page page, whose holder holds
 * its lock while it is alive; a tombstone says why it ended (M5 design
 * 4.3): taken over, or unlocked by an admin.
 */
type FakeSession = {
  page: string;
  holder: Person;
  expiresIn: number;
  ended?: { code: "page.edit_session_taken_over" } | { code: "page.edit_session_unlocked"; by: Person };
};

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
 * nodesDown or viewsDown is set, the tree or the views cannot be read;
 * while notebookGone is, Plans' tree is 404 notebook.not_found; while
 * writesDown is, no content can be written.
 *
 * It writes the tree as the server does where the pages look: a new page
 * goes last under its parent, a title a sibling has (by lower case) is 409
 * page.title_taken, a move takes the page to its place, a deletion takes
 * the page's subtree; it checks no permission, which a test answers
 * through answers. The test changes the state as another tab would; what
 * went out is in sent.
 *
 * Each page's content is its title on a line at revision 1 unless
 * contents says otherwise; a write in a session that is not open is 409
 * page.edit_session_ended, one on another revision 409
 * page.revision_mismatch (M4/P6 design 3.6).
 *
 * A page's edit lock is its alive session's (M5 design 4.1–4.3): Ada's
 * opening is 409 page.locked while someone holds it, Ada herself elsewhere
 * too unless she takes it over; a session taken over or unlocked answers
 * its beats and writes with why. The test holds, takes over, unlocks and
 * lapses sessions as other tabs and members would.
 *
 * withTasks gives a page a content of task items and its view (tasksView);
 * a toggle of one (M5/P6 design 3.3) answers as the server does: 409
 * page.revision_mismatch on another revision, 422 at an offset of no item,
 * the page as it is for an item in that state, 409 page.locked while a
 * session holds the page; otherwise it writes the content and its view.
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
    contents: new Map<string, { content: string; revision: number }>(),
    sessions: new Map<string, FakeSession>(),
    nodesDown: false,
    viewsDown: false,
    notebookGone: false,
    writesDown: false,
    /** hold opens holder's session of the page pageId, its lease expiresIn seconds; it answers its id. */
    hold(pageId: string, holder: Person = bob, expiresIn = 120): string {
      const id = `held-${(++held).toString()}`;
      server.sessions.set(id, { page: pageId, holder, expiresIn });
      return id;
    },
    /** takeOver has Ada take the page over in another tab: her sessions of it end so; it answers the new one's id. */
    takeOver(pageId: string): string {
      endAlive(server.sessions, pageId, { code: "page.edit_session_taken_over" });
      return server.hold(pageId, ada);
    },
    /** unlock has an admin, by, unlock the page: its alive session ends so. */
    unlock(pageId: string, by: Person = bob): void {
      endAlive(server.sessions, pageId, { code: "page.edit_session_unlocked", by });
    },
    /** withTasks gives the page pageId the content of task items and its view, at revision. */
    withTasks(pageId: string, content: string, revision = 1): void {
      server.contents.set(pageId, { content, revision });
      server.views.set(pageId, { html: tasksView(content), revision });
    },
    /** lockOf is the page's edit lock, as GET edit-lock answers it. */
    lockOf(pageId: string): EditLock {
      const alive = aliveOf(server.sessions, pageId);
      return alive === undefined
        ? { holder: null, expires_in: null }
        : { holder: alive.holder, expires_in: alive.expiresIn };
    },
  };
  const app = signedInApp({
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [{ ...notebookJSON, role }] }),
    [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: () => {
      server.sent.push("GET nodes");
      if (server.notebookGone) {
        return problem(404, "notebook.not_found");
      }
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
    "GET /api/v0/pages/*/edit-lock": (request) => json(server.lockOf(idOf(request))),
    "DELETE /api/v0/pages/*/edit-lock": (request) => {
      server.sent.push(`RELEASE ${server.nodes.find((node) => node.id === idOf(request))?.name}`);
      server.unlock(idOf(request), ada);
      return new Response(null, { status: 204 });
    },
    ...writeRoutes(server),
    ...contentRoutes(server),
    ...taskRoutes(server),
    ...answers,
  });
  return Object.assign(server, { app });
}

let created = 50;
let held = 0;

/** aliveOf is the page's alive session: the one holding its lock. */
function aliveOf(sessions: Map<string, FakeSession>, pageId: string): FakeSession | undefined {
  return [...sessions.values()].find((session) => session.page === pageId && session.ended === undefined);
}

/** endAlive ends the page's alive sessions as ended says, keeping their tombstones. */
function endAlive(sessions: Map<string, FakeSession>, pageId: string, ended: FakeSession["ended"]): void {
  for (const session of sessions.values()) {
    if (session.page === pageId && session.ended === undefined) {
      session.ended = ended;
    }
  }
}

/** endedAnswer is what a tombstone answers a beat or a write: why it ended, and who unlocked it. */
function endedAnswer(session: FakeSession): Response {
  const ended = session.ended;
  if (ended === undefined) {
    throw new Error("the session is alive");
  }
  return ended.code === "page.edit_session_unlocked"
    ? problem(409, ended.code, { ended_by: ended.by })
    : problem(409, ended.code);
}

/** The writes' answers of server, which change it. */
function writeRoutes(server: {
  sent: string[];
  nodes: TreeNode[];
  sessions: Map<string, FakeSession>;
}): Record<string, Answer> {
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
      const gone = server.nodes.filter((each) => each === node || isUnder(server.nodes, each, node.id));
      // Someone else's edit of a page that would go refuses it; Ada's own do not (M5 design 4.4).
      const editing = gone
        .map((each) => aliveOf(server.sessions, each.id))
        .find((alive) => alive !== undefined && alive.holder.user_id !== ada.user_id);
      if (editing !== undefined) {
        return problem(409, "page.locked", { lock: { page_id: editing.page, ...editing.holder } });
      }
      server.nodes = server.nodes.filter((each) => !gone.includes(each));
      for (const [id, session] of server.sessions) {
        if (gone.some((each) => each.id === session.page)) {
          server.sessions.delete(id);
        }
      }
      return new Response(null, { status: 204 });
    },
  };
}

type ContentState = {
  sent: string[];
  nodes: TreeNode[];
  contents: Map<string, { content: string; revision: number }>;
  sessions: Map<string, FakeSession>;
  writesDown: boolean;
};

/** The answers to the contents and the edit sessions of server, which change it. */
function contentRoutes(server: ContentState): Record<string, Answer> {
  const nameOf = (request: Request) => server.nodes.find((node) => node.id === idOf(request))?.name;
  const contentOf = (id: string, name: string) => server.contents.get(id) ?? { content: `${name}\n`, revision: 1 };
  let opened = 0;
  return {
    "GET /api/v0/pages/*/content": (request) => {
      const name = nameOf(request);
      server.sent.push(`GET content ${name}`);
      if (name === undefined) {
        return problem(404, "page.not_found");
      }
      const content: PageContent = { ...contentOf(idOf(request), name), content_hash: "0".repeat(64) };
      return json(content);
    },
    "PUT /api/v0/pages/*/content": async (request) => {
      const write = (await request.clone().json()) as {
        content: string;
        base_revision: number;
        edit_session_id?: string;
      };
      const name = nameOf(request);
      server.sent.push(
        `PUT ${name} ${JSON.stringify(write.content)} on ${write.base_revision} in ${write.edit_session_id}`
      );
      if (server.writesDown) {
        return Promise.reject(new TypeError("offline"));
      }
      const node = server.nodes.find((each) => each.id === idOf(request));
      if (node === undefined) {
        return problem(404, "page.not_found");
      }
      const session = write.edit_session_id === undefined ? undefined : server.sessions.get(write.edit_session_id);
      if (write.edit_session_id !== undefined && session === undefined) {
        return problem(409, "page.edit_session_ended");
      }
      if (session?.ended !== undefined) {
        return endedAnswer(session);
      }
      const current = contentOf(node.id, node.name);
      if (write.base_revision !== current.revision) {
        return problem(409, "page.revision_mismatch");
      }
      const revision = current.content === write.content ? current.revision : current.revision + 1;
      server.contents.set(node.id, { content: write.content, revision });
      return json({
        ...node,
        ancestors: [],
        revision,
        byte_size: write.content.length,
        content_updated_at: node.updated_at,
        content_updated_by: "",
      });
    },
    "POST /api/v0/pages/*/edit-sessions": async (request) => {
      const name = nameOf(request);
      const { take_over: takeOver = false } = (await request
        .clone()
        .json()
        .catch(() => ({}))) as { take_over?: boolean };
      server.sent.push(`OPEN ${name}${takeOver ? " TAKE" : ""}`);
      const page = idOf(request);
      if (name === undefined) {
        return problem(404, "page.not_found");
      }
      const alive = aliveOf(server.sessions, page);
      if (alive !== undefined && takeOver && alive.holder.user_id === ada.user_id) {
        endAlive(server.sessions, page, { code: "page.edit_session_taken_over" });
      } else if (alive !== undefined) {
        return problem(409, "page.locked", { lock: { page_id: page, ...alive.holder } });
      }
      const id = `session-${(++opened).toString()}`;
      server.sessions.set(id, { page, holder: ada, expiresIn: 120 });
      return json({ id, page_id: page, expires_at: "2026-10-03T08:02:00Z" }, 201);
    },
    "POST /api/v0/edit-sessions/*/heartbeat": (request) => {
      server.sent.push(`BEAT ${idOf(request)}`);
      const session = server.sessions.get(idOf(request));
      if (session === undefined) {
        return problem(404, "page.edit_session_not_found");
      }
      return session.ended === undefined
        ? json({ id: idOf(request), page_id: session.page, expires_at: "2026-10-03T08:02:00Z" })
        : endedAnswer(session);
    },
    "DELETE /api/v0/edit-sessions/*": (request) => {
      server.sent.push(`END ${idOf(request)}`);
      return server.sessions.delete(idOf(request))
        ? new Response(null, { status: 204 })
        : problem(404, "page.edit_session_not_found");
    },
  };
}

/** The tasks of a content of task item lines, "- [ ] text" or "- [x] text", by their characters' offsets. */
function tasksOf(content: string): { offset: number; checked: boolean; text: string }[] {
  const tasks = [];
  let at = 0;
  for (const line of content.split(/(?<=\n)/)) {
    const match = /^- \[([ xX])\] (.*)$/.exec(line.replace(/\r?\n$/, ""));
    if (match !== null) {
      tasks.push({ offset: at + 3, checked: match[1] !== " ", text: match[2] ?? "" });
    }
    at += new TextEncoder().encode(line).length;
  }
  return tasks;
}

/** tasksView is the reading view of a content of task items, as the server renders it. */
function tasksView(content: string): string {
  const items = tasksOf(content).map(
    ({ offset, checked, text }) =>
      `<li><input ${checked ? 'checked="" ' : ""}disabled="" type="checkbox" data-task="${offset.toString()}"> ${text}</li>\n`
  );
  return `<ul>\n${items.join("")}</ul>\n`;
}

/** The answer to a toggle of a task item of server's pages, which changes it. */
function taskRoutes(server: ContentState & { views: Map<string, PageView> }): Record<string, Answer> {
  return {
    "POST /api/v0/pages/*/toggle-task": async (request) => {
      const toggle = (await request.clone().json()) as TaskToggle;
      const node = server.nodes.find((each) => each.id === idOf(request));
      server.sent.push(
        `TOGGLE ${node?.name} ${toggle.offset.toString()} ${String(toggle.checked)} on ${toggle.base_revision.toString()}`
      );
      if (node === undefined) {
        return problem(404, "page.not_found");
      }
      const current = server.contents.get(node.id) ?? { content: `${node.name}\n`, revision: 1 };
      if (toggle.base_revision !== current.revision) {
        return problem(409, "page.revision_mismatch");
      }
      const task = tasksOf(current.content).find((each) => each.offset === toggle.offset);
      if (task === undefined) {
        return problem(422, "validation_failed", {
          errors: [{ field: "offset", code: "out_of_range", message: "no task" }],
        });
      }
      let revision = current.revision;
      if (task.checked !== toggle.checked) {
        const alive = aliveOf(server.sessions, node.id);
        if (alive !== undefined) {
          return problem(409, "page.locked", { lock: { page_id: node.id, ...alive.holder } });
        }
        const bytes = new TextEncoder().encode(current.content);
        bytes[toggle.offset] = (toggle.checked ? "x" : " ").charCodeAt(0);
        const content = new TextDecoder().decode(bytes);
        revision += 1;
        server.contents.set(node.id, { content, revision });
        server.views.set(node.id, { html: tasksView(content), revision });
      }
      return json({
        ...node,
        ancestors: [],
        revision,
        byte_size: 0,
        content_updated_at: node.updated_at,
        content_updated_by: "",
      });
    },
  };
}

/** idOf is the id in a request's path, /api/v0/nodes/{id}…, /api/v0/pages/{id}… */
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
