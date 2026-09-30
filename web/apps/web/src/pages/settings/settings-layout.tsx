import { NavLink, Outlet } from "react-router";

import { useT } from "../../i18n/i18n";
import type { MessageKey } from "../../i18n/messages/en";
import { cn } from "../../lib/cn";

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
  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <h1 className="text-2xl font-semibold">{t("settings.title")}</h1>
      <div className="flex flex-col gap-6 md:flex-row">
        <nav aria-label={t("settings.title")} className="flex gap-1 md:w-48 md:shrink-0 md:flex-col">
          {sections.map(({ path, label }) => (
            <NavLink
              key={path}
              to={path}
              className={({ isActive }) =>
                cn(
                  "rounded-md px-3 py-2 text-sm hover:bg-accent",
                  isActive ? "bg-accent font-medium" : "text-muted-foreground"
                )
              }
            >
              {t(label)}
            </NavLink>
          ))}
        </nav>
        <div className="min-w-0 flex-1">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
