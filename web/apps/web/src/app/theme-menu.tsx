import { Monitor, Moon, Sun } from "lucide-react";
import { observer } from "mobx-react-lite";

import { Button } from "../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu";
import { useT } from "../i18n/i18n";
import { useStore } from "../stores/context";
import { isThemePreference, themePreferences } from "../stores/preferences.store";

const icons = { system: Monitor, light: Sun, dark: Moon } as const;

export const ThemeMenu = observer(function ThemeMenu() {
  const { preferences } = useStore();
  const t = useT();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t("theme.label")}>
          {preferences.resolvedTheme === "dark" ? <Moon /> : <Sun />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuRadioGroup
          value={preferences.theme}
          onValueChange={(value) => isThemePreference(value) && preferences.setTheme(value)}
        >
          {themePreferences.map((value) => {
            const Icon = icons[value];
            return (
              <DropdownMenuRadioItem key={value} value={value}>
                <Icon />
                {t(`theme.${value}`)}
              </DropdownMenuRadioItem>
            );
          })}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
});
