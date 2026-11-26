import { House, SlidersHorizontal, type LucideIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { createContext, use, useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";
import { createPortal } from "react-dom";
import { Navigate, Outlet, useMatch, useParams } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { GiveCornerBack } from "../../app/corner";
import { useShellColumn } from "../../app/layout";
import { NotLoaded } from "../../app/not-loaded";
import { SettingsDirectory, SettingsTop } from "../../app/settings-directory";
import { useSettingsMode } from "../../app/settings-origin";
import { Drawer, ShellSidebar, useDrawer } from "../../app/shell-sidebar";
import { NavItem } from "../../components/nav-item";
import { useT } from "../../i18n/i18n";
import type { MessageKey } from "../../i18n/messages/en";
import type { Workspace } from "../../services/workspace.service";
import { SessionChangedError } from "../../session/token-manager";
import { useNotebooks, useStore, useWorkspaces } from "../../stores/context";
import { NotFoundPage } from "../not-found";
import { NotebookNav } from "./notebook-nav";
import { makeSlot, NotebookSlot, SlotHolder } from "./notebook-slot";
import { SearchBox } from "./search-box";
import { useSearchShortcuts } from "./search-shortcuts";
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

/**
 * The workspace's own pages, in the order the navigation lists them, before its notebooks, each with its icon. The
 * settings' link leaves with the column, their directory taking its place: arrived at (M13/P3/S1, decision 816), at
 * their first page, no index's redirect between.
 */
const sections: readonly {
  path: string;
  label: Extract<MessageKey, `workspace.${string}`>;
  icon: LucideIcon;
  end?: boolean;
  leaves?: boolean;
}[] = [
  { path: "", label: "workspace.home", icon: House, end: true },
  { path: "/settings/general", label: "workspace.settings", icon: SlidersHorizontal, leaves: true },
];

/**
 * ShellPlace is where the page is, for the strip of a phone's width (decision 784): the notebook of the address,
 * when the workspace's list has it, else the workspace.
 */
const ShellPlace = observer(function ShellPlace({ workspace }: { workspace: Workspace }) {
  const notebooks = useNotebooks(workspace);
  // Not useParams: the column is the workspace's, above the routes that name a notebook.
  const id = useMatch("/:slug/notebooks/:id/*")?.params.id;
  return (id === undefined ? undefined : notebooks.byId(id)?.name) ?? workspace.name;
});

/**
 * WorkspaceLayout is the shell of a workspace's pages (M2/P5 design 3.2):
 * the left column (ShellSidebar, M13/P2/S1), its top the switcher and the
 * workspace's search box (M8/P5 design 4.2), then its navigation: its
 * pages and its notebooks (M3/P4 design 3.3), the open notebook in its
 * item, its row, page tree and places, in a container the shell keeps by
 * the notebook's id (notebook-slot.tsx, M13/P2/S2), and in its foot the
 * account; beside the page chosen, which is the
 * layout's main. The column is no landmark of its own: the search and the
 * navigation are the ones it holds; it goes into the layout's place for
 * it, outside the main (M4/P5 design 3.2). Without it, the load failed or
 * the workspace not found, the account is at the top right. It listens
 * for Mod+K and Mod+Shift+F, which give the box the focus
 * (useSearchShortcuts, M13/P2/S3). Below md the column is a drawer
 * (useDrawer), whose actions it gives what is below it (Drawer): the
 * tree's links, an unsaved edit's guard. In the settings' mode
 * (useSettingsMode, M13/P3/S1, decisions 810, 814, 819) the column is
 * their directory: the same ShellSidebar, its drawer, menu and strip kept;
 * no search box, the keys then the browser's; the notebook's row, tree and
 * places held out of sight, still mounted (a hidden SlotHolder); where the
 * workspace's column was scrolled kept for its return (ScrollKept).
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
  // For the shell's life: a notebook's tree stays in its container as the notebook moves in the list (decision 776).
  const [slot] = useState(makeSlot);
  // The search box's field, which Mod+K and Mod+Shift+F give the focus: none until the workspace is shown.
  const field = useRef<HTMLInputElement>(null);
  const drawer = useDrawer();
  // In the settings' mode no search box is mounted: field is null, the keys left to the browser (decision 819).
  useSearchShortcuts({ field, actions: drawer.actions });
  const settings = useSettingsMode();
  // Where the workspace's column was scrolled, through the settings (ScrollKept).
  const scrolled = useRef(0);
  // Not useParams: the column is the workspace's, above the routes that name a notebook.
  const notebookId = useMatch("/:slug/notebooks/:id/*")?.params.id;
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
  // ShellSidebar): a probe of the three engines saw no difference, harmless kept (M13/P2/S1). One column whatever the
  // mode: the drawer, its menu and its strip stay the same elements as the settings come and go (decision 810). In the
  // settings' mode, their directory, the notebook's row, tree and places held out of sight (decisions 814, 836).
  const left = settings ? (
    <ShellSidebar drawer={drawer} place={t("settings.title")} top={<SettingsTop fallback={workspace} />}>
      <SettingsDirectory here="workspace" workspace={workspace} />
      {notebookId !== undefined && (
        <div hidden>
          <SlotHolder id={notebookId} />
        </div>
      )}
      <ScrollKept at={scrolled} settings />
    </ShellSidebar>
  ) : (
    <ShellSidebar
      drawer={drawer}
      place={<ShellPlace workspace={workspace} />}
      top={
        <>
          <WorkspaceSwitcher current={workspace} />
          <SearchBox key={workspace.id} workspace={workspace} field={field} />
        </>
      }
    >
      <nav aria-label={workspace.name} className="flex flex-col gap-0.5">
        {sections.map(({ path, label, icon: Icon, end, leaves }) => (
          <NavItem key={path} to={`/${slug}${path}`} end={end} state={leaves === true ? arrived : undefined}>
            <Icon aria-hidden />
            {t(label)}
          </NavItem>
        ))}
        <NotebookNav key={workspace.id} workspace={workspace} />
      </nav>
      <ScrollKept at={scrolled} settings={false} />
    </ShellSidebar>
  );
  return (
    <WorkspaceContext value={workspace}>
      <NotebookSlot value={slot}>
        <Drawer value={drawer.actions}>
          {column !== null && createPortal(left, column)}
          <Outlet key={workspace.id} />
        </Drawer>
      </NotebookSlot>
    </WorkspaceContext>
  );
});

/**
 * ScrollKept keeps where the workspace's left column was scrolled (at) through the settings' mode, which shows another
 * middle in the same scrolling area: shorter, the directory would leave the browser to clamp the column's scrollTop
 * (M13/P3/S1; review A's M3). Last in the workspace's middle, its layout effect runs after the notebook's container is
 * held there again (the tree's rows in, the scrollTop not clamped) and before the tree brings the page shown into view
 * (PageTree's, below the Outlet, later in the commit): it puts the column back, then follows its scroll, the event's
 * own (asynchronous: the clamp of the commit that leaves the workspace's mode comes after its listener went). In the
 * settings' mode the directory starts at its top.
 */
function ScrollKept({ at, settings }: { at: RefObject<number>; settings: boolean }) {
  const marker = useRef<HTMLSpanElement>(null);
  useLayoutEffect(() => {
    const area = marker.current?.closest<HTMLElement>("[data-sidebar-scroll]");
    if (area === null || area === undefined) {
      return undefined;
    }
    if (settings) {
      area.scrollTop = 0;
      return undefined;
    }
    area.scrollTop = at.current;
    const keep = () => {
      at.current = area.scrollTop;
    };
    area.addEventListener("scroll", keep, { passive: true });
    return () => area.removeEventListener("scroll", keep);
  }, [at, settings]);
  return <span ref={marker} hidden />;
}
