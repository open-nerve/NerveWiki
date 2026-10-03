import { observer } from "mobx-react-lite";
import { Outlet, useLocation } from "react-router";

import { useDocumentTitle } from "../../app/document-title";
import { NavItem } from "../../components/nav-item";
import { useT } from "../../i18n/i18n";
import { useWorkspace } from "../workspace/workspace-layout";
import { useNotebook } from "./notebook-layout";

/** The notebook settings' pages, in the order the navigation lists them. */
const sections = [
  { path: "general", label: "notebookSettings.general" },
  { path: "members", label: "notebookSettings.members" },
] as const;

/**
 * NotebookSettingsLayout is the navigation of a notebook's settings and the
 * page chosen, as a workspace's settings have theirs (M3/P4 design 3.4):
 * whoever sees the notebook comes in; only its admins change it.
 */
export const NotebookSettingsLayout = observer(function NotebookSettingsLayout() {
  const { slug } = useWorkspace();
  const notebook = useNotebook();
  const t = useT();
  const { pathname } = useLocation();
  const shown = sections.find(({ path }) => pathname === `/${slug}/notebooks/${notebook.id}/settings/${path}`);
  useDocumentTitle(shown && t(shown.label), t("notebookSettings.heading", { name: notebook.name }));
  return (
    <div className="max-w-4xl space-y-6">
      <h1 className="text-2xl font-semibold break-words">{t("notebookSettings.heading", { name: notebook.name })}</h1>
      <div className="flex flex-col gap-6 md:flex-row">
        <nav aria-label={t("notebookSettings.title")} className="flex gap-1 md:w-40 md:shrink-0 md:flex-col">
          {sections.map(({ path, label }) => (
            <NavItem key={path} to={`/${slug}/notebooks/${notebook.id}/settings/${path}`}>
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
