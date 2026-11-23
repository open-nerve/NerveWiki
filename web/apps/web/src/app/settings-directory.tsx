import {
  ArrowLeftRight,
  Box,
  ChevronLeft,
  KeyRound,
  LockKeyhole,
  SlidersHorizontal,
  SunMoon,
  UserRound,
  Users,
  type LucideIcon,
} from "lucide-react";
import { observer } from "mobx-react-lite";
import { useState } from "react";
import { Link, useMatch } from "react-router";
import useSWR from "swr";

import { NavItem } from "../components/nav-item";
import { useT, type PlainKey } from "../i18n/i18n";
import { isolate, Isolated } from "../i18n/isolated";
import type { Workspace } from "../services/workspace.service";
import { useStore, useWorkspaces } from "../stores/context";
import { arrived } from "./arrival";
import { useSettingsOrigin } from "./settings-origin";
import { useFollow } from "./shell-sidebar";

/** A settings page as the directory lists it: its path under its group's, its name, its icon, whether admins' alone. */
type Section = { path: string; label: PlainKey; icon: LucideIcon; admins: boolean };

/** The account's settings (M1/P6 design 3.2; the appearance and the language, M13/P3/S1), in the directory's order. */
export const personalSections = [
  { path: "profile", label: "settings.profile", icon: UserRound, admins: false },
  { path: "appearance", label: "settings.appearance", icon: SunMoon, admins: false },
  { path: "security", label: "settings.security", icon: LockKeyhole, admins: false },
  { path: "tokens", label: "settings.tokens", icon: KeyRound, admins: false },
] as const satisfies readonly Section[];

/** A workspace's settings (M2/P5 design 3.6); the ownerless notebooks its admins' alone (M3/P5 design 3.3). */
export const workspaceSections = [
  { path: "general", label: "workspaceSettings.general", icon: SlidersHorizontal, admins: false },
  { path: "members", label: "workspaceSettings.members", icon: Users, admins: false },
  { path: "ownerless", label: "workspaceSettings.ownerless", icon: Box, admins: true },
] as const satisfies readonly Section[];

/** A notebook's settings (M3/P4 design 3.4; its imports and exports, M9). */
export const notebookSections = [
  { path: "general", label: "notebookSettings.general", icon: SlidersHorizontal, admins: false },
  { path: "members", label: "notebookSettings.members", icon: Users, admins: false },
  { path: "transfer", label: "notebookSettings.transfer", icon: ArrowLeftRight, admins: false },
] as const satisfies readonly Section[];

/** sectionAt is the section of sections whose page, under base, is pathname: the page shown, if any. */
export function sectionAt<S extends Section>(sections: readonly S[], base: string, pathname: string): S | undefined {
  return sections.find(({ path }) => pathname === `${base}/${path}`);
}

/**
 * useDirectoryWorkspace is the workspace the account's settings offer in their directory (decisions 818, 837, 843):
 * the origin's, if the account's list has it, else the one this device showed last (read as the settings mount: it is
 * not observable), as the list has it. The list is read if no shell read it yet and there is a workspace to look for;
 * without one, the list would serve nothing. None while it loads, if the read failed, or if the list has neither.
 */
export function useDirectoryWorkspace(): Workspace | undefined {
  const workspaces = useWorkspaces();
  const { preferences } = useStore();
  const origin = useSettingsOrigin();
  const [last] = useState(() => preferences.lastWorkspace());
  const wanted = origin?.workspace ?? last;
  useSWR(wanted !== undefined && workspaces.list === undefined ? "workspaces" : null, () => workspaces.load());
  const from = origin?.workspace === undefined ? undefined : workspaces.bySlug(origin.workspace);
  return from ?? (last === undefined ? undefined : workspaces.bySlug(last));
}

/** Where Back goes, and the name it says: none, "Back" alone. */
type Back = { to: string; name: string | undefined };

/**
 * useBack is the directory's Back (decisions 813, 834): to the origin, named by its notebook, else its workspace, as
 * their lists have them, renamed too; the origin's notebook gone from its list, to its workspace's home; its workspace
 * gone from the list read (left, deleted, or the origin its 404), to /; an origin in no workspace (the page that
 * creates one, an invitation's), "Back" alone, its fragment kept: an invitation's token is in it. An origin in a
 * workspace goes without its fragment: the page's h1 takes the arrival focus, its anchor's element the reading view's,
 * which of the two last depending on whether the view was cached. Without an origin, the settings' address opened,
 * to the home of the directory's workspace (fallback), else to /.
 */
function useBack(fallback: Workspace | undefined): Back {
  const origin = useSettingsOrigin();
  const workspaces = useWorkspaces();
  const store = useStore();
  if (origin === undefined) {
    return fallback === undefined ? { to: "/", name: undefined } : { to: `/${fallback.slug}`, name: fallback.name };
  }
  if (origin.workspace === undefined) {
    return { to: `${origin.pathname}${origin.search}${origin.hash}`, name: undefined };
  }
  const to = `${origin.pathname}${origin.search}`;
  // By the route's slug, not the path's first part: a workspace gone is not found either, as the app's own pages are
  // not, and goes to / where they go back to themselves (review A's N3).
  const workspace = workspaces.bySlug(origin.workspace);
  if (workspace === undefined) {
    return workspaces.list === undefined ? { to, name: undefined } : { to: "/", name: undefined };
  }
  const [, , part, id] = origin.pathname.split("/");
  const notebooks = part === "notebooks" && id !== undefined ? store.notebooksOf(workspace) : undefined;
  if (id !== undefined && notebooks?.list !== undefined) {
    const notebook = notebooks.byId(id);
    return notebook === undefined ? { to: `/${workspace.slug}`, name: workspace.name } : { to, name: notebook.name };
  }
  return { to, name: workspace.name };
}

/**
 * SettingsTop is the top of the settings' left column (decisions 813, 817): Back, arrived at, its name the notebook's
 * or the workspace's it goes to, in a bdi; then the title, a line of text, no heading: the page's h1 is its own name.
 * fallback is the directory's workspace, where Back goes without an origin. Back is no NavItem: it marks no page.
 */
export const SettingsTop = observer(function SettingsTop({ fallback }: { fallback: Workspace | undefined }) {
  const t = useT();
  const { to, name } = useBack(fallback);
  return (
    <>
      <Link
        to={to}
        state={arrived}
        className="flex h-7.5 items-center gap-2.25 rounded-row px-2 text-row text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground [&_svg]:size-4 [&_svg]:shrink-0"
      >
        <ChevronLeft aria-hidden />
        <span className="min-w-0 truncate">
          {name === undefined ? (
            t("settings.back")
          ) : (
            <Isolated {...isolate((at) => t("settings.backTo", { name: at(name) }))} />
          )}
        </span>
      </Link>
      <p className="px-2 pt-1.5 text-lg font-[650]">{t("settings.title")}</p>
    </>
  );
});

/**
 * GroupTitle is a group's title in the directory: a line of text, no heading (a page has one h1, and the column's h2
 * is the notebook's row's), the name it says in a bdi, a space before it outside the bdi: read as "Workspace Lab"
 * (review A's N1). The title does not shrink, the name alone cut short (review B's I4).
 */
function GroupTitle({ title, name }: { title: string; name?: string }) {
  return (
    <p className="flex min-w-0 gap-1.5 px-2 pt-3.5 pb-1 text-xs text-faint">
      <span className="shrink-0">{title}</span>
      {name !== undefined && (
        <>
          {" "}
          <bdi className="min-w-0 truncate font-medium text-muted-foreground">{name}</bdi>
        </>
      )}
    </p>
  );
}

/**
 * SettingsDirectory is the settings' left column below its top (decisions 810, 816, 817): a navigation for each group,
 * named as the settings' layouts named theirs: the account's, always; the workspace's, workspace's when the column has
 * one; the notebook's, the notebook of the address as the workspace's list has it. A link to a page of the same shell
 * (here) is followed as the column's links are (useFollow: opened, the focus staying on it, arrived at from the drawer
 * open); one to the other shell's is arrived at, the directory mounting anew there.
 */
export const SettingsDirectory = observer(function SettingsDirectory({
  here,
  workspace,
}: {
  here: "personal" | "workspace";
  workspace: Workspace | undefined;
}) {
  const t = useT();
  const follow = useFollow();
  const store = useStore();
  const id = useMatch("/:slug/notebooks/:id/settings/*")?.params.id;
  const notebook = workspace === undefined || id === undefined ? undefined : store.notebooksOf(workspace)?.byId(id);
  const item = (to: string, same: boolean, { path, label, icon: Icon }: Section) => (
    <NavItem key={path} to={to} {...(same ? { onClick: follow(to) } : { state: arrived })}>
      <Icon aria-hidden />
      {t(label)}
    </NavItem>
  );
  return (
    <>
      <nav aria-label={t("settings.title")} className="flex flex-col gap-0.5">
        <GroupTitle title={t("settings.groupPersonal")} />
        {personalSections.map((section) => item(`/settings/${section.path}`, here === "personal", section))}
      </nav>
      {workspace !== undefined && (
        <nav aria-label={t("workspaceSettings.title")} className="flex flex-col gap-0.5">
          <GroupTitle title={t("settings.groupWorkspace")} name={workspace.name} />
          {workspaceSections
            .filter((section) => !section.admins || workspace.role === "admin")
            .map((section) => item(`/${workspace.slug}/settings/${section.path}`, here === "workspace", section))}
        </nav>
      )}
      {workspace !== undefined && notebook !== undefined && (
        <nav aria-label={t("notebookSettings.title")} className="flex flex-col gap-0.5">
          <GroupTitle title={t("settings.groupNotebook")} name={notebook.name} />
          {notebookSections.map((section) =>
            item(`/${workspace.slug}/notebooks/${notebook.id}/settings/${section.path}`, here === "workspace", section)
          )}
        </nav>
      )}
    </>
  );
});
