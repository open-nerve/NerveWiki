import { observer } from "mobx-react-lite";
import { Outlet, useLocation } from "react-router";

import { useDocumentTitle } from "../../app/document-title";
import { sectionAt, workspaceSections } from "../../app/settings-directory";
import { SettingsPage } from "../../app/settings-page";
import { useT } from "../../i18n/i18n";
import { useWorkspace } from "./workspace-layout";

/**
 * WorkspaceSettingsLayout is a workspace's settings page (M2/P5 design 3.6) in the settings' mode (M13/P3/S1): their
 * directory is WorkspaceLayout's left column, where the ownerless notebooks are their admins' alone (decision 810);
 * here the page chosen, keyed by its path, its h1 its name, taking the arrival focus as it mounts (SettingsPage,
 * decision 815; M13/P3/S2).
 */
export const WorkspaceSettingsLayout = observer(function WorkspaceSettingsLayout() {
  const { slug, name } = useWorkspace();
  const t = useT();
  const { pathname } = useLocation();
  const shown = sectionAt(workspaceSections, `/${slug}/settings`, pathname);
  useDocumentTitle(shown && t(shown.label), t("workspaceSettings.title"), name);
  return shown === undefined ? (
    <Outlet />
  ) : (
    <SettingsPage key={shown.path} title={t(shown.label)}>
      <Outlet />
    </SettingsPage>
  );
});
