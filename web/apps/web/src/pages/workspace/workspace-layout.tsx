import { House, SlidersHorizontal, type LucideIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { createContext, use, useContext, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Navigate, Outlet, useParams } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { GiveCornerBack } from "../../app/corner";
import { useShellColumn } from "../../app/layout";
import { NotLoaded } from "../../app/not-loaded";
import { ShellSidebar } from "../../app/shell-sidebar";
import { NavItem } from "../../components/nav-item";
import { useT } from "../../i18n/i18n";
import type { MessageKey } from "../../i18n/messages/en";
import type { Workspace } from "../../services/workspace.service";
import { SessionChangedError } from "../../session/token-manager";
import { useStore, useWorkspaces } from "../../stores/context";
import { NotFoundPage } from "../not-found";
import { NotebookNav } from "./notebook-nav";
import { SearchBox } from "./search-box";
import { WorkspaceSwitcher } from "./workspace-switcher";

/** The workspace WorkspaceLayout shows. */
const WorkspaceContext = createContext<Workspace | undefined>(undefined);

/**
 * useWorkspace is the workspace of the page's address, as the account's list
 * holds it, renamed too, or as it was last found while it stays: only below
 * WorkspaceLayout, which shows its pages once it has found it.
 */
export function useWorkspace(): Workspace {
  const workspace = use(WorkspaceContext);
  if (workspace === undefined) {
    throw new Error("useWorkspace is used outside WorkspaceLayout");
  }
  return workspace;
}

/** NotebookColumn is where a notebook's pages put their part of the left column: below the workspace's navigation. */
const NotebookColumn = createContext<HTMLElement | null>(null);

/**
 * useNotebookColumn is the element a notebook's pages portal their part of
 * the left column into, the notebook's page tree (M4/P5 design 3.6), or
 * null before the column has it.
 */
export function useNotebookColumn(): HTMLElement | null {
  return useContext(NotebookColumn);
}

/** The workspace's own pages, in the order the navigation lists them, before its notebooks, each with its icon. */
const sections: readonly {
  path: string;
  label: Extract<MessageKey, `workspace.${string}`>;
  icon: LucideIcon;
  end?: boolean;
}[] = [
  { path: "", label: "workspace.home", icon: House, end: true },
  { path: "/settings", label: "workspace.settings", icon: SlidersHorizontal },
];

/**
 * WorkspaceLayout is the shell of a workspace's pages (M2/P5 design 3.2):
 * the left column (ShellSidebar, M13/P2/S1), its top the switcher and the
 * workspace's search box (M8/P5 design 4.2), then its navigation: its
 * pages and its notebooks (M3/P4 design 3.3), then a notebook's page tree,
 * and in its foot the account; beside the page chosen, which is the
 * layout's main. The column is no landmark of its own: the search and the
 * navigation are the ones it holds; it goes into the layout's place for
 * it, outside the main (M4/P5 design 3.2). Without it, the load failed or
 * the workspace not found, the account is at the top right.
 * It finds the workspace of the address in the account's list: a slug the
 * list does not have is no page of the app's, whether the account was
 * never a member or the workspace is gone; one this tab has just deleted
 * or left goes to the landing instead, arrived at. One gone otherwise
 * while this tab edits a page of it with changes not saved, the account
 * removed from it or the workspace deleted, stays as it was last found
 * until the edit ends, as the notebook's and the page's shells do (M5/P4
 * design 3.9; M4–M5 Codex review R3), by its id: another made since by
 * its name waits for the edit too. A workspace found is the one this
 * device showed last. Its pages start anew with each workspace: what a
 * form holds of one is never sent to another.
 */
export const WorkspaceLayout = observer(function WorkspaceLayout() {
  const { slug = "" } = useParams();
  const workspaces = useWorkspaces();
  const store = useStore();
  const { preferences } = store;
  const t = useT();
  const column = useShellColumn();
  const [notebookColumn, setNotebookColumn] = useState<HTMLElement | null>(null);
  const { error, mutate } = useSWR("workspaces", () => workspaces.load());
  const [last, setLast] = useState<Workspace | undefined>(undefined);
  const listed = workspaces.bySlug(slug);
  const kept =
    last?.slug === slug &&
    last.id !== listed?.id &&
    !workspaces.wasRemoved(slug) &&
    store.unsavedEdit({ workspaceId: last.id });
  const workspace = kept ? last : listed;
  if (workspace !== undefined && workspace !== last) {
    setLast(workspace);
  }
  const found = listed !== undefined;

  useEffect(() => {
    if (found) {
      preferences.setLastWorkspace(slug);
    }
  }, [preferences, slug, found]);

  if (workspaces.list === undefined) {
    // A load cut by a change of session is loading, as NotLoaded shows it.
    const failed = error !== undefined && !(error instanceof SessionChangedError);
    return (
      <>
        {failed && <GiveCornerBack />}
        <NotLoaded error={error} retry={() => void mutate()} />
      </>
    );
  }
  if (workspace === undefined) {
    return workspaces.wasRemoved(slug) ? (
      <Navigate replace to="/" state={arrived} />
    ) : (
      <>
        <GiveCornerBack />
        <NotFoundPage />
      </>
    );
  }
  // Stuck and as tall as the window, the column is still no scroll anchor of the window's (overflow-anchor: none,
  // ShellSidebar): a probe of the three engines saw no difference, harmless kept (M13/P2/S1).
  const left = (
    <ShellSidebar
      top={
        <>
          <WorkspaceSwitcher current={workspace} />
          <SearchBox key={workspace.id} workspace={workspace} />
        </>
      }
    >
      <nav aria-label={workspace.name} className="flex flex-col gap-0.5">
        {sections.map(({ path, label, icon: Icon, end }) => (
          <NavItem key={path} to={`/${slug}${path}`} end={end}>
            <Icon aria-hidden />
            {t(label)}
          </NavItem>
        ))}
        <NotebookNav key={workspace.id} workspace={workspace} />
      </nav>
      <div ref={setNotebookColumn} className="mt-3" />
    </ShellSidebar>
  );
  return (
    <WorkspaceContext value={workspace}>
      {column !== null && createPortal(left, column)}
      <NotebookColumn value={notebookColumn}>
        <Outlet key={workspace.id} />
      </NotebookColumn>
    </WorkspaceContext>
  );
});
