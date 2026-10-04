import { observer } from "mobx-react-lite";
import { createContext, use, useContext, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Navigate, Outlet, useParams } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { useShellColumn } from "../../app/layout";
import { NotLoaded } from "../../app/not-loaded";
import { NavItem } from "../../components/nav-item";
import { useT } from "../../i18n/i18n";
import type { MessageKey } from "../../i18n/messages/en";
import type { Workspace } from "../../services/workspace.service";
import { useStore, useWorkspaces } from "../../stores/context";
import { NotFoundPage } from "../not-found";
import { NotebookNav } from "./notebook-nav";
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

/** The workspace's own pages, in the order the navigation lists them, before its notebooks. */
const sections: readonly { path: string; label: Extract<MessageKey, `workspace.${string}`>; end?: boolean }[] = [
  { path: "", label: "workspace.home", end: true },
  { path: "/settings", label: "workspace.settings" },
];

/**
 * WorkspaceLayout is the shell of a workspace's pages (M2/P5 design 3.2):
 * the left column, with the workspace's navigation: the switcher, its
 * pages and its notebooks (M3/P4 design 3.3), beside the page chosen,
 * which is the layout's main. The column is no landmark of its own: the
 * navigation is the one it holds, then a notebook's page tree; it goes
 * into the layout's place for it, outside the main (M4/P5 design 3.2).
 * It finds the workspace of the address in the account's list: a slug the
 * list does not have is no page of the app's, whether the account was
 * never a member or the workspace is gone; one this tab has just deleted
 * or left goes to the landing instead, arrived at. One gone otherwise
 * while this tab edits a page of it with changes not saved, the account
 * removed from it or the workspace deleted, stays as it was last found
 * until the edit ends, as the notebook's and the page's shells do (M5/P4
 * design 3.9; M4–M5 Codex review R3). A workspace found is the one this
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
  const shown = workspaces.bySlug(slug);
  if (shown !== undefined && shown !== last) {
    setLast(shown);
  }
  const found = shown !== undefined;

  useEffect(() => {
    if (found) {
      preferences.setLastWorkspace(slug);
    }
  }, [preferences, slug, found]);

  if (workspaces.list === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const removed = workspaces.wasRemoved(slug);
  const kept = !removed && last?.slug === slug && store.unsavedEdit({ workspaceId: last.id });
  const workspace = shown ?? (kept ? last : undefined);
  if (workspace === undefined) {
    return removed ? <Navigate replace to="/" state={arrived} /> : <NotFoundPage />;
  }
  const left = (
    <div data-shell className="space-y-4 border-b p-3 md:w-60 md:shrink-0 md:border-r md:border-b-0">
      <nav aria-label={workspace.name} className="space-y-4">
        <WorkspaceSwitcher current={workspace} />
        <div className="flex flex-col gap-1">
          {sections.map(({ path, label, end }) => (
            <NavItem key={path} to={`/${slug}${path}`} end={end}>
              {t(label)}
            </NavItem>
          ))}
        </div>
        <NotebookNav key={workspace.id} workspace={workspace} />
      </nav>
      <div ref={setNotebookColumn} />
    </div>
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
