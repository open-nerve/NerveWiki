import { observer } from "mobx-react-lite";
import { Outlet, useLocation } from "react-router";

import { useDocumentTitle } from "../../app/document-title";
import { notebookSections, sectionAt } from "../../app/settings-directory";
import { SettingsPage } from "../../app/settings-page";
import { useT } from "../../i18n/i18n";
import { useWorkspace } from "../workspace/workspace-layout";
import { useNotebook } from "./notebook-layout";

/**
 * NotebookSettingsLayout is a notebook's settings page (M3/P4 design 3.4) in the settings' mode (M13/P3/S1): whoever
 * sees the notebook comes in, only its admins change it; their directory is WorkspaceLayout's left column (decision
 * 810); here the page chosen, keyed by its path, its h1 taking the arrival focus as it mounts (SettingsPage, decision
 * 815).
 */
export const NotebookSettingsLayout = observer(function NotebookSettingsLayout() {
  const { slug } = useWorkspace();
  const notebook = useNotebook();
  const t = useT();
  const { pathname } = useLocation();
  const shown = sectionAt(notebookSections, `/${slug}/notebooks/${notebook.id}/settings`, pathname);
  useDocumentTitle(shown && t(shown.label), t("notebookSettings.heading", { name: notebook.name }));
  return shown === undefined ? (
    <Outlet />
  ) : (
    <SettingsPage key={shown.path} title={t("notebookSettings.heading", { name: notebook.name })}>
      <Outlet />
    </SettingsPage>
  );
});
