import { observer } from "mobx-react-lite";
import { Outlet } from "react-router";

import { NavItem } from "../../components/nav-item";
import { useT } from "../../i18n/i18n";
import { useWorkspace } from "./workspace-layout";

/** The workspace settings' pages, in the order the navigation lists them. */
const sections = [
  { path: "general", label: "workspaceSettings.general" },
  { path: "members", label: "workspaceSettings.members" },
] as const;

/**
 * WorkspaceSettingsLayout is the navigation of a workspace's settings and
 * the page chosen, as the account's settings have theirs (M2/P5 design
 * 3.6).
 */
export const WorkspaceSettingsLayout = observer(function WorkspaceSettingsLayout() {
  const { slug } = useWorkspace();
  const t = useT();
  return (
    <div className="max-w-4xl space-y-6">
      <h1 className="text-2xl font-semibold">{t("workspaceSettings.title")}</h1>
      <div className="flex flex-col gap-6 md:flex-row">
        <nav aria-label={t("workspaceSettings.title")} className="flex gap-1 md:w-40 md:shrink-0 md:flex-col">
          {sections.map(({ path, label }) => (
            <NavItem key={path} to={`/${slug}/settings/${path}`}>
              {t(label)}
            </NavItem>
          ))}
        </nav>
        <div className="min-w-0 flex-1">
          <Outlet />
        </div>
      </div>
    </div>
  );
});
