import { observer } from "mobx-react-lite";
import { useEffect } from "react";
import { Outlet, useParams } from "react-router";
import useSWR from "swr";

import { NotLoaded } from "../../app/not-loaded";
import { NavItem } from "../../components/nav-item";
import { useT } from "../../i18n/i18n";
import type { MessageKey } from "../../i18n/messages/en";
import type { Workspace } from "../../services/workspace.service";
import { useStore, useWorkspaces } from "../../stores/context";
import { NotFoundPage } from "../not-found";
import { WorkspaceSwitcher } from "./workspace-switcher";

/**
 * useWorkspace is the workspace of the page's address, as the account's list
 * holds it, renamed too: only below WorkspaceLayout, which shows its pages
 * once it has found it.
 */
export function useWorkspace(): Workspace {
  const { slug = "" } = useParams();
  const workspace = useWorkspaces().bySlug(slug);
  if (workspace === undefined) {
    throw new Error("useWorkspace is used outside WorkspaceLayout");
  }
  return workspace;
}

/** The workspace's pages, in the order the navigation lists them; M3 puts the notebooks in between. */
const sections: readonly { path: string; label: Extract<MessageKey, `workspace.${string}`>; end?: boolean }[] = [
  { path: "", label: "workspace.home", end: true },
  { path: "/settings", label: "workspace.settings" },
];

/**
 * WorkspaceLayout is the shell of a workspace's pages (M2/P5 design 3.2):
 * the left column, with the switcher and the workspace's navigation, beside
 * the page chosen. It finds the workspace of the address in the account's
 * list: a slug the list does not have is no page of the app's, whether the
 * account was never a member or the workspace is gone. A workspace found is
 * the one this device showed last.
 */
export const WorkspaceLayout = observer(function WorkspaceLayout() {
  const { slug = "" } = useParams();
  const workspaces = useWorkspaces();
  const { preferences } = useStore();
  const t = useT();
  const { error, mutate } = useSWR("workspaces", () => workspaces.load());
  const workspace = workspaces.bySlug(slug);
  const found = workspace !== undefined;

  useEffect(() => {
    if (found) {
      preferences.setLastWorkspace(slug);
    }
  }, [preferences, slug, found]);

  if (workspaces.list === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  if (workspace === undefined) {
    return <NotFoundPage />;
  }
  return (
    <div data-shell className="flex flex-1 flex-col md:flex-row">
      <aside
        aria-label={t("workspace.sidebar")}
        className="space-y-4 border-b p-3 md:w-60 md:shrink-0 md:border-r md:border-b-0"
      >
        <WorkspaceSwitcher current={workspace} />
        <nav aria-label={workspace.name} className="flex flex-col gap-1">
          {sections.map(({ path, label, end }) => (
            <NavItem key={path} to={`/${slug}${path}`} end={end}>
              {t(label)}
            </NavItem>
          ))}
        </nav>
      </aside>
      <div className="min-w-0 flex-1 p-6">
        <Outlet />
      </div>
    </div>
  );
});
