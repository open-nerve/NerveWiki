import type { WorkspaceRole } from "../services/member.service";
import type { Notebook, NotebookRole, WorkspaceAccess } from "../services/notebook.service";

/** The order of the roles, lowest first. */
const rank: Record<NotebookRole, number> = { reader: 1, editor: 2, admin: 3 };

/** The role a workspace's admin or member has by a notebook's access, or none. */
const byAccess: Record<WorkspaceAccess, NotebookRole | undefined> = {
  none: undefined,
  viewer: "reader",
  editor: "editor",
};

/**
 * effectiveNotebookRole is the role a member of a workspace has in one of
 * its notebooks: the higher of explicit, the role of their membership
 * (undefined for none), and the role the notebook's access gives their
 * role in the workspace, which a guest never gets. undefined: none, the
 * notebook is not theirs to see. It is the server's
 * shared.EffectiveNotebookRole (server/internal/shared/notebook_role.go),
 * with the same cases in effective-role.test.ts.
 */
export function effectiveNotebookRole(
  explicit: NotebookRole | undefined,
  access: WorkspaceAccess,
  workspace: WorkspaceRole
): NotebookRole | undefined {
  const reached = workspace === "guest" ? undefined : byAccess[access];
  if (reached !== undefined && (explicit === undefined || rank[reached] > rank[explicit])) {
    return reached;
  }
  return explicit;
}

/**
 * writesPages tells whether role writes a notebook's pages (M4 design 5):
 * an editor and an admin do, a reader does not; a role this does not know
 * does not either.
 */
export function writesPages(role: NotebookRole): boolean {
  return role === "editor" || role === "admin";
}

/**
 * writesNotebook tells whether the account writes notebook's pages from
 * the web: its role writes them (writesPages), and no upgrade is under
 * way, which freezes the notebook but for its initiator's MCP writes, its
 * initiator's web included (M11/P1 design 4.4). Each control of the web
 * that writes asks this, not the role alone.
 */
export function writesNotebook(notebook: Pick<Notebook, "role" | "upgrade">): boolean {
  return writesPages(notebook.role) && notebook.upgrade === null;
}

/**
 * frozenByUpgrade tells whether only the upgrade under way keeps the
 * account from writing notebook's pages: its role writes them. What the
 * web says of a control it does not offer then names the upgrade, not the
 * role (decision 247).
 */
export function frozenByUpgrade(notebook: Pick<Notebook, "role" | "upgrade">): boolean {
  return writesPages(notebook.role) && notebook.upgrade !== null;
}
