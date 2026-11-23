import { observer } from "mobx-react-lite";
import { createPortal } from "react-dom";
import { Outlet, useLocation } from "react-router";

import { useDocumentTitle } from "../../app/document-title";
import { useShellColumn } from "../../app/layout";
import {
  personalSections,
  sectionAt,
  SettingsDirectory,
  SettingsTop,
  useDirectoryWorkspace,
} from "../../app/settings-directory";
import { SettingsPage } from "../../app/settings-page";
import { Drawer, ShellSidebar, useDrawer } from "../../app/shell-sidebar";
import { useT } from "../../i18n/i18n";

/**
 * SettingsLayout is the account's settings in the settings' mode (M13/P3/S1; 00-M13 4.4.1): outside any workspace's
 * shell, it draws its own left column (decision 810), their directory, its drawer below md its own (useDrawer, an
 * ancestor of the pages: its effects after theirs), the workspace it offers the one they were entered from or this
 * device showed last (useDirectoryWorkspace); beside it the page chosen, its name the h1 (SettingsPage), keyed by the
 * page: each page's takes the arrival focus as it mounts. No search box, nor its keys: Mod+K and Mod+Shift+F are the
 * browser's here (decision 819).
 */
export const SettingsLayout = observer(function SettingsLayout() {
  const t = useT();
  const { pathname } = useLocation();
  const shown = sectionAt(personalSections, "/settings", pathname);
  useDocumentTitle(shown && t(shown.label), t("settings.title"));
  const column = useShellColumn();
  const drawer = useDrawer();
  const workspace = useDirectoryWorkspace();
  const left = (
    <ShellSidebar drawer={drawer} place={t("settings.title")} top={<SettingsTop fallback={workspace} />}>
      <SettingsDirectory here="personal" workspace={workspace} />
    </ShellSidebar>
  );
  return (
    <Drawer value={drawer.actions}>
      {column !== null && createPortal(left, column)}
      {/* The index's redirect has no page of its own: its Outlet alone, for a commit. */}
      {shown === undefined ? (
        <Outlet />
      ) : (
        <SettingsPage key={shown.path} title={t(shown.label)}>
          <Outlet />
        </SettingsPage>
      )}
    </Drawer>
  );
});
