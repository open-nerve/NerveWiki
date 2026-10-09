import type { Asset } from "../services/asset.service";
import type { BacklinkPage, LinkLanding, PageProperties } from "../services/linking.service";
import type { NotebookRole } from "../services/notebook.service";
import type { EditLock, NodeMove, PageContent, PageView, TaskToggle, TreeNode } from "../services/page.service";
import { titleKey } from "../lib/title-key";
import { extensionOf } from "../lib/upload-name";
import { instanceJSON, json, notebookJSON, problem, signedInApp, userJSON, type Answer } from "./fakes";
import { abortedFor, formOf } from "./transfer";

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

/** assetNode is an attachment named name, under the page parent, at the root without one (M7/P2). */
export function assetNode(n: number, name: string, parent?: TreeNode): TreeNode {
  return { ...pageNode(n, name, parent), kind: "asset" };
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
 *
 * A tag's pages are the ids tags has for it, by its name as the path
 * carries it decoded (M6/P5), none for a tag it does not have. A link's
 * landing (M6/P6) is what landings has for its target, by default a page
 * titled the target at the root. The notebook's link targets are its pages,
 * each linked by its title, with the aliases aliases has for it; its tags,
 * tags' names with their pages counted. A page's backlinks are what
 * backlinks has for it, a page of them each; its properties, what
 * properties has, by default none (M6/P7).
 *
 * The notebook's attachments are its nodes of kind asset (M7/P4): listed
 * by parent as the server lists them, by title key then id, limit of them
 * a page (50 without one; 422 outside 1–100), a parent that is no page 404
 * page.not_found; each linked by its name, or its path where another
 * attachment of the notebook has its title key; uploaded as the server
 * takes them, a reader refused (403 forbidden), a parent that is no page
 * refused (422 on parent_id), a name ending with .md refused (422 on
 * name), a file over the instance's asset_max_bytes refused (413
 * payload_too_large), a name a sibling has 409 page.title_taken. While
 * uploadsHeld is set, an upload is answered once released; while
 * viewsHeld is, a reading view.
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
    /** The pages of each tag, by its name. */
    tags: new Map<string, string[]>(),
    /** The landing of each link's target, or its answer. */
    landings: new Map<string, LinkLanding | (() => Response | Promise<Response>)>(),
    /** The aliases of each page, by its id. */
    aliases: new Map<string, string[]>(),
    /** Each page's backlinks, a page of them each, by its id. */
    backlinks: new Map<string, BacklinkPage[]>(),
    /** Each page's properties, by its id. */
    properties: new Map<string, PageProperties>(),
    /** When the addresses of the attachments listed expire. */
    assetsExpireAt: "2100-01-01T00:00:00Z",
    /** Whether uploads wait to be released. */
    uploadsHeld: false,
    /** Whether reading views wait to be released. */
    viewsHeld: false,
    held: [] as (() => void)[],
    /** release answers the uploads and the views held. */
    release(): void {
      const held = server.held;
      server.held = [];
      for (const answer of held) {
        answer();
      }
    },
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
      server.views.set(pageId, { html: tasksView(content), revision, assets_expire_at: null });
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
    "GET /api/v0/pages/*/view": async (request) => {
      const id = new URL(request.url).pathname.split("/")[4] ?? "";
      const page = server.nodes.find((node) => node.id === id);
      server.sent.push(`GET view ${page?.name}`);
      if (server.viewsHeld) {
        await new Promise<void>((resolve) => server.held.push(resolve));
      }
      if (server.viewsDown) {
        return Promise.reject(new TypeError("offline"));
      }
      return page === undefined
        ? problem(404, "page.not_found")
        : json(server.views.get(id) ?? { html: `<p>${page.name}</p>`, revision: 1, assets_expire_at: null });
    },
    "GET /api/v0/pages/*/edit-lock": (request) => json(server.lockOf(idOf(request))),
    "GET /api/v0/pages/*/link-landing": (request) => {
      const target = new URL(request.url).searchParams.get("target") ?? "";
      server.sent.push(`GET landing ${target}`);
      const landing = server.landings.get(target) ?? {
        node_id: null,
        landing: { parent_id: null, title: target },
        reason: null,
      };
      return typeof landing === "function" ? landing() : json(landing);
    },
    [`GET /api/v0/notebooks/${notebookJSON.id}/link-targets`]: () => {
      server.sent.push("GET link targets");
      return json({
        data: server.nodes
          .filter((node) => node.kind === "page")
          .map((node) => ({
            id: node.id,
            kind: "page",
            name: node.name,
            link: node.name,
            aliases: server.aliases.get(node.id) ?? [],
          })),
      });
    },
    [`GET /api/v0/notebooks/${notebookJSON.id}/tags`]: () => {
      server.sent.push("GET tags");
      return json({ data: [...server.tags].map(([tag, pages]) => ({ tag, count: pages.length })) });
    },
    "GET /api/v0/pages/*/backlinks": (request) => {
      const cursor = new URL(request.url).searchParams.get("cursor");
      server.sent.push(`GET backlinks ${idOf(request)} ${cursor ?? ""}`.trim());
      const pages = server.backlinks.get(idOf(request)) ?? [];
      return json(pages[cursor === null ? 0 : Number(cursor)] ?? { data: [], next_cursor: null });
    },
    "GET /api/v0/pages/*/properties": (request) => {
      server.sent.push(`GET properties ${idOf(request)}`);
      return json(
        server.properties.get(idOf(request)) ?? { valid: true, properties: [], links: [], assets_expire_at: null }
      );
    },
    [`GET /api/v0/notebooks/${notebookJSON.id}/tags/*`]: (request) => {
      const tag = decodeURIComponent(new URL(request.url).pathname.split("/")[6] ?? "");
      server.sent.push(`GET tag ${tag}`);
      return json({ data: (server.tags.get(tag) ?? []).map((id) => ({ id })) });
    },
    "DELETE /api/v0/pages/*/edit-lock": (request) => {
      server.sent.push(`RELEASE ${server.nodes.find((node) => node.id === idOf(request))?.name}`);
      server.unlock(idOf(request), ada);
      return new Response(null, { status: 204 });
    },
    ...writeRoutes(server),
    ...assetRoutes(server, role),
    ...contentRoutes(server),
    ...taskRoutes(server),
    ...answers,
  });
  return Object.assign(server, { app });
}

// The nodes made: numbered past those the tests make (under 600), as a file's tests go on; an id's number stays three
// digits after 100 for 299 of them.
let created = 600;
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
      (node) => node.parent_id === parent && node.id !== except && titleKey(node.name) === titleKey(title)
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
      if (move.parent_id !== null && !server.nodes.some((each) => each.id === move.parent_id && each.kind === "page")) {
        return problem(422, "validation_failed", {
          errors: [{ field: "parent_id", code: "not_allowed", message: "The parent is no page of this notebook." }],
        });
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

/**
 * assetJSON is the attachment node as the server answers it, of size bytes, its addresses expiring at expires,
 * linked by link (its name by default; none without an extension, as one whose only dot starts it).
 */
export function assetJSON(
  node: TreeNode,
  size = 1024,
  expires = "2100-01-01T00:00:00Z",
  link: string = node.name
): Asset {
  const extension = extensionOf(node.name).slice(1).toLowerCase();
  const mimes: Record<string, string> = {
    png: "image/png",
    mp3: "audio/mpeg",
    mp4: "video/mp4",
    pdf: "application/pdf",
  };
  return {
    id: node.id,
    notebook_id: node.notebook_id,
    parent_id: node.parent_id,
    name: node.name,
    link: extension === "" ? null : link,
    mime: mimes[extension] ?? "application/octet-stream",
    byte_size: size,
    sha256: "0".repeat(64),
    width: null,
    height: null,
    created_by: userJSON.id,
    created_at: node.created_at,
    content_url: `/api/v0/assets/${node.id}/content?sig=1`,
    download_url: `/api/v0/assets/${node.id}/content?sig=1&download=1`,
    expires_at: expires,
  };
}

type AssetState = {
  sent: string[];
  nodes: TreeNode[];
  assetsExpireAt: string;
  uploadsHeld: boolean;
  held: (() => void)[];
};

/** The answers to the attachments of server: their lists and uploads, which change it. */
function assetRoutes(server: AssetState, role: NotebookRole): Record<string, Answer> {
  const sizes = new Map<string, number>();
  const nameOf = (id: string | null) =>
    id === null ? "root" : (server.nodes.find((node) => node.id === id)?.name ?? id);
  const isPage = (id: string | null) =>
    id === null || server.nodes.some((node) => node.id === id && node.kind === "page");
  // Its name where no other attachment of the notebook has its title key, its path from the root otherwise.
  const linkOf = (node: TreeNode) => {
    const alike = server.nodes.filter((each) => each.kind === "asset" && titleKey(each.name) === titleKey(node.name));
    if (alike.length === 1) {
      return node.name;
    }
    const path = [node.name];
    for (let parent = node.parent_id; parent !== null;) {
      const page = server.nodes.find((each) => each.id === parent);
      path.unshift(page?.name ?? "");
      parent = page?.parent_id ?? null;
    }
    return path.join("/");
  };
  const answered = (node: TreeNode) => assetJSON(node, sizes.get(node.id), server.assetsExpireAt, linkOf(node));
  return {
    [`GET /api/v0/notebooks/${notebookJSON.id}/assets`]: (request) => {
      const query = new URL(request.url).searchParams;
      const parent = query.get("parent_id");
      const at = Number(query.get("cursor") ?? "0");
      const limit = Number(query.get("limit") ?? "50");
      server.sent.push(`GET assets ${nameOf(parent)}${at === 0 ? "" : ` from ${at.toString()}`}`);
      if (!Number.isInteger(limit) || limit < 1 || limit > 100) {
        return problem(422, "validation_failed", { errors: [{ field: "limit", code: "out_of_range", message: "" }] });
      }
      if (!isPage(parent)) {
        return problem(404, "page.not_found");
      }
      const all = server.nodes
        .filter((node) => node.kind === "asset" && node.parent_id === parent)
        .toSorted((a, b) => compare(titleKey(a.name), titleKey(b.name)) || compare(a.id, b.id));
      const next = at + limit;
      return json({ data: all.slice(at, next).map(answered), next_cursor: next < all.length ? next.toString() : null });
    },
    [`POST /api/v0/notebooks/${notebookJSON.id}/assets`]: async (request) => {
      const form = formOf(request);
      const parent = (form?.get("parent_id") as string | null) ?? null;
      const name = String(form?.get("name") ?? "");
      const file = form?.get("file");
      server.sent.push(`UPLOAD ${name} under ${nameOf(parent)}`);
      if (server.uploadsHeld) {
        await new Promise<void>((resolve) => server.held.push(resolve));
      }
      // One stopped before its body all went makes nothing; its answer is no one's.
      if (abortedFor(request)) {
        return problem(400, "aborted");
      }
      if (role === "reader") {
        return problem(403, "forbidden");
      }
      if (!isPage(parent)) {
        return problem(422, "validation_failed", {
          errors: [{ field: "parent_id", code: "not_allowed", message: "no page" }],
        });
      }
      if (name.toLowerCase().endsWith(".md")) {
        return problem(422, "validation_failed", { errors: [{ field: "name", code: "not_allowed", message: ".md" }] });
      }
      if (file instanceof Blob && file.size > instanceJSON.asset_max_bytes) {
        return problem(413, "payload_too_large");
      }
      if (server.nodes.some((node) => node.parent_id === parent && titleKey(node.name) === titleKey(name))) {
        return problem(409, "page.title_taken");
      }
      const node: TreeNode = { ...assetNode(created++, name), parent_id: parent };
      server.nodes = [...server.nodes, node];
      sizes.set(node.id, file instanceof Blob ? file.size : 0);
      return json(answered(node), 201);
    },
    "GET /api/v0/assets/*": (request) => {
      const node = server.nodes.find((each) => each.id === idOf(request) && each.kind === "asset");
      server.sent.push(`GET asset ${node?.name ?? idOf(request)}`);
      return node === undefined ? problem(404, "asset.not_found") : json(answered(node));
    },
  };
}

/** compare orders two strings as the server's keys are: by their code units. */
function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
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
        server.views.set(node.id, { html: tasksView(content), revision, assets_expire_at: null });
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
