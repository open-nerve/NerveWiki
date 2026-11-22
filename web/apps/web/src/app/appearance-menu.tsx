import { Monitor, Moon, Sun, SunMoon } from "lucide-react";
import { observer } from "mobx-react-lite";

import { Button } from "../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu";
import { useT } from "../i18n/i18n";
import { isLocale, locales } from "../i18n/locale";
import { useStore } from "../stores/context";
import { isThemePreference, themePreferences } from "../stores/preferences.store";

const icons = { system: Monitor, light: Sun, dark: Moon } as const;

/** A choice made stays in its menu: the theme and the language change in place, both in one go (decision 770). */
const keepOpen = (event: Event) => event.preventDefault();

/**
 * AppearanceItems are the theme's three choices and the language's two, each a group named for what it chooses: the
 * account menu's Appearance and the top right's own Appearance (M13/P2/S1). A choice applies at once and leaves the
 * menu open.
 */
export const AppearanceItems = observer(function AppearanceItems() {
  const { preferences } = useStore();
  const t = useT();
  return (
    <>
      <DropdownMenuLabel>{t("theme.label")}</DropdownMenuLabel>
      <DropdownMenuRadioGroup
        aria-label={t("theme.label")}
        value={preferences.theme}
        onValueChange={(value) => isThemePreference(value) && preferences.setTheme(value)}
      >
        {themePreferences.map((value) => {
          const Icon = icons[value];
          return (
            <DropdownMenuRadioItem key={value} value={value} onSelect={keepOpen}>
              <Icon />
              {t(`theme.${value}`)}
            </DropdownMenuRadioItem>
          );
        })}
      </DropdownMenuRadioGroup>
      <DropdownMenuSeparator />
      <DropdownMenuLabel>{t("language.label")}</DropdownMenuLabel>
      <DropdownMenuRadioGroup
        aria-label={t("language.label")}
        value={preferences.locale}
        onValueChange={(value) => isLocale(value) && preferences.setLocale(value)}
      >
        {locales.map((locale) => (
          <DropdownMenuRadioItem key={locale.value} value={locale.value} lang={locale.value} onSelect={keepOpen}>
            {/* An icon's room: the names on the themes' line (review B, S1B-N1). */}
            <span aria-hidden className="size-4 shrink-0" />
            {locale.name}
          </DropdownMenuRadioItem>
        ))}
      </DropdownMenuRadioGroup>
    </>
  );
});

/**
 * AppearanceMenu is the top right's where no account is shown: signed out, the session unavailable, the account's load
 * failed (M13/P2/S1, decision 792).
 */
export function AppearanceMenu() {
  const t = useT();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" shape="pill" aria-label={t("appearance.label")}>
          <SunMoon />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <AppearanceItems />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
