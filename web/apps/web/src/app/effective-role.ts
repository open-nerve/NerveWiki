import type { WorkspaceRole } from "../services/member.service";
import type { NotebookRole, WorkspaceAccess } from "../services/notebook.service";

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
