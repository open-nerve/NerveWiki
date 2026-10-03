import { Outlet, useLocation } from "react-router";

import { useDocumentTitle } from "../../app/document-title";
import { NavItem } from "../../components/nav-item";
import { useT } from "../../i18n/i18n";
import type { MessageKey } from "../../i18n/messages/en";

/** The settings' pages, in the order the navigation lists them (M1/P6 design 3.2). */
const sections: readonly { path: string; label: Extract<MessageKey, `settings.${string}`> }[] = [
  { path: "/settings/profile", label: "settings.profile" },
  { path: "/settings/security", label: "settings.security" },
  { path: "/settings/tokens", label: "settings.tokens" },
];

/**
 * SettingsLayout is the navigation of the settings and the page chosen:
 * beside it on a wide screen, below it on a narrow one. The link of the
 * page shown has aria-current.
 */
export function SettingsLayout() {
  const t = useT();
  const { pathname } = useLocation();
  const shown = sections.find(({ path }) => path === pathname);
  useDocumentTitle(shown && t(shown.label), t("settings.title"));
  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <h1 className="text-2xl font-semibold">{t("settings.title")}</h1>
      <div className="flex flex-col gap-6 md:flex-row">
        <nav aria-label={t("settings.title")} className="flex gap-1 md:w-48 md:shrink-0 md:flex-col">
          {sections.map(({ path, label }) => (
            <NavItem key={path} to={path}>
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
}
