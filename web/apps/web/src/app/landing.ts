import type { Workspace } from "../services/workspace.service";
import { createsNotebooks } from "../stores/notebook.store";

/**
 * landingPath is where / goes (M2/P5 design 3.3): the workspace this device
 * showed last, if the account still has it; else the first of the list,
 * which the server sorts by name; else the page that creates one, which
 * says what to do instead while creation is off.
 */
export function landingPath(workspaces: readonly { slug: string }[], last: string | undefined): string {
  const slug = workspaces.find((workspace) => workspace.slug === last)?.slug ?? workspaces[0]?.slug;
  return slug === undefined ? "/create-workspace" : `/${slug}`;
}

/**
 * notebookTarget is the workspace onboarding's notebook step creates in
 * (M3/P4 design 3.6): in the landing's order, the workspace this device
 * showed last, then the list's by name, the first of which the account is
 * an admin or a member; a guest cannot create one. None: the step only
 * says how.
 */
export function notebookTarget<W extends Pick<Workspace, "slug" | "role">>(
  workspaces: readonly W[],
  last: string | undefined
): W | undefined {
  const creating = workspaces.filter(createsNotebooks);
  return creating.find((workspace) => workspace.slug === last) ?? creating[0];
}
