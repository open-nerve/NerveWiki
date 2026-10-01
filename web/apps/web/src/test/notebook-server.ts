import type { WorkspaceMember } from "../services/member.service";
import type { NotebookMember } from "../services/notebook-member.service";
import type { Notebook, NotebookRole, WorkspaceAccess } from "../services/notebook.service";
import type { Workspace } from "../services/workspace.service";
import { json, notebookJSON, problem, signedInApp, userJSON, workspaceJSON, type Answer } from "./fakes";

/** A member of Lab, as GET members lists them; the account is Ada. */
function workspaceMember(n: number, name: string, role: Workspace["role"]): WorkspaceMember {
  return {
    id: `0199a2b4-0000-7000-8000-0000000000d${n}`,
    user_id: n === 1 ? userJSON.id : `0199a2b4-0000-7000-8000-00000000000${n}`,
    role,
    display_name: name,
    email: `${name.toLowerCase()}@example.com`,
    created_at: `2026-10-0${n}T08:00:00Z`,
  };
}

export const ada = workspaceMember(1, "Ada", "admin");
export const bob = workspaceMember(2, "Bob", "member");
export const cy = workspaceMember(3, "Cy", "guest");

/** notebookMember is member's membership of Plans, with role. */
export function notebookMember(member: WorkspaceMember, role: NotebookRole): NotebookMember {
  return {
    id: member.id.replace(/d(\d)$/, "f$1"),
    user_id: member.user_id,
    role,
    display_name: member.display_name,
    email: member.email,
    created_at: member.created_at,
  };
}

/** The role a workspace's admin or member has by a notebook's access, or none. */
const byAccess: Record<WorkspaceAccess, NotebookRole | undefined> = {
  none: undefined,
  viewer: "reader",
  editor: "editor",
};

type NotebookServerOptions = {
  /** Ada's role in Lab. */
  workspaceRole?: Workspace["role"];
  /** Plans' access. */
  access?: WorkspaceAccess;
  /** Plans' members; Ada's role in it is hers among them, else by its access. */
  members?: NotebookMember[];
  /** Answers instead of the server's, by "METHOD /path" as byRoute takes them; an answer not ok changes nothing. */
  answers?: Record<string, Answer>;
};

/**
 * notebookServer is the server of Lab's notebook Plans as Ada (userJSON)
 * sees it (M3/P4 design 3.4): Lab's members are Ada, Bob and Cy (a guest);
 * Plans' members are Ada, its admin, and Bob, its editor, unless members
 * says otherwise. Plans and its members change as the requests say,
 * except where answers answers otherwise; Ada sees Plans while she is its
 * member or its access opens it to her; as a guest of Lab, she sees no
 * member's address. What went out is in sent; the test changes the state
 * as another tab would.
 */
export function notebookServer({
  workspaceRole = "admin",
  access = "none",
  members = [notebookMember(ada, "admin"), notebookMember(bob, "editor")],
  answers = {},
}: NotebookServerOptions = {}) {
  const server = {
    sent: [] as string[],
    plans: { ...notebookJSON, workspace_access: access } as Notebook,
    deleted: false,
    members,
    workspaceMembers: [{ ...ada, role: workspaceRole }, bob, cy],
  };
  /** Plans as Ada sees it, or undefined. */
  const seen = (): Notebook | undefined => {
    const own = server.members.find((m) => m.user_id === ada.user_id)?.role;
    const role = own ?? (workspaceRole === "guest" ? undefined : byAccess[server.plans.workspace_access]);
    return server.deleted || role === undefined
      ? undefined
      : { ...server.plans, role, member_count: server.members.length };
  };
  /** answer is the test's answer to route, else the server's; only an answer that is ok changes the state. */
  const answer = async (route: string, request: Request, ours: () => Response, change: () => void) => {
    const theirs = await answers[route]?.(request);
    const response = theirs ?? ours();
    if (response.ok) {
      change();
    }
    return response;
  };
  const memberOf = (request: Request) => server.members.find((m) => request.url.endsWith(m.id));
  /** A guest of the workspace sees no member's address. */
  const shown = <T extends { email: string | null }>(list: T[]) =>
    workspaceRole === "guest" ? list.map((each) => ({ ...each, email: null })) : list;
  const plans = `/api/v0/notebooks/${notebookJSON.id}`;
  const app = signedInApp({
    "GET /api/v0/workspaces": () => json({ data: [{ ...workspaceJSON, role: workspaceRole }] }),
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: seen() === undefined ? [] : [seen()] }),
    "GET /api/v0/workspaces/lab/members": () => {
      server.sent.push("GET workspace members");
      return json({ data: shown(server.workspaceMembers) });
    },
    [`GET ${plans}`]: () => {
      const notebook = seen();
      return notebook === undefined ? problem(404, "notebook.not_found") : json(notebook);
    },
    [`PATCH ${plans}`]: async (request) => {
      const body = (await request.clone().json()) as { name?: string; workspace_access?: WorkspaceAccess };
      server.sent.push(`PATCH ${JSON.stringify(body)}`);
      const changed = { ...server.plans, ...body, updated_at: "2026-10-03T08:00:00Z" };
      return answer(
        `PATCH ${plans}`,
        request,
        () => json({ ...seen(), ...changed }),
        () => (server.plans = changed)
      );
    },
    [`DELETE ${plans}`]: (request) => {
      server.sent.push("DELETE");
      return answer(
        `DELETE ${plans}`,
        request,
        () => new Response(null, { status: 204 }),
        () => (server.deleted = true)
      );
    },
    [`POST ${plans}/leave`]: (request) => {
      server.sent.push("leave");
      return answer(
        `POST ${plans}/leave`,
        request,
        () => new Response(null, { status: 204 }),
        () => (server.members = server.members.filter((m) => m.user_id !== ada.user_id))
      );
    },
    [`GET ${plans}/members`]: () => {
      server.sent.push("GET members");
      return json({ data: shown(server.members) });
    },
    [`POST ${plans}/members`]: async (request) => {
      const { user_id: userId, role } = (await request.clone().json()) as { user_id: string; role: NotebookRole };
      const member = server.workspaceMembers.find((m) => m.user_id === userId);
      server.sent.push(`POST ${member?.display_name} ${role}`);
      const added =
        member === undefined ? undefined : { ...notebookMember(member, role), created_at: "2026-10-03T08:00:00Z" };
      return answer(
        `POST ${plans}/members`,
        request,
        () => (added === undefined ? problem(422, "validation_failed") : json(added, 201)),
        () => added !== undefined && server.members.push(added)
      );
    },
    "PATCH /api/v0/notebook-members/*": async (request) => {
      const { role } = (await request.clone().json()) as { role: NotebookRole };
      const member = memberOf(request);
      server.sent.push(`PATCH ${member?.display_name} ${role}`);
      return answer(
        "PATCH /api/v0/notebook-members/*",
        request,
        () => (member === undefined ? problem(404, "notebook.member_not_found") : json({ ...member, role })),
        () => (server.members = server.members.map((m) => (m === member ? { ...m, role } : m)))
      );
    },
    "DELETE /api/v0/notebook-members/*": (request) => {
      const member = memberOf(request);
      server.sent.push(`DELETE ${member?.display_name}`);
      return answer(
        "DELETE /api/v0/notebook-members/*",
        request,
        () => (member === undefined ? problem(404, "notebook.member_not_found") : new Response(null, { status: 204 })),
        () => (server.members = server.members.filter((m) => m !== member))
      );
    },
  });
  return Object.assign(server, { app });
}
