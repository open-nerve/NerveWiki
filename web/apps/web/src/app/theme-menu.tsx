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
import type { ThemePreference } from "../stores/preferences.store";

const choices = [
  { value: "system", label: "theme.system", icon: Monitor },
  { value: "light", label: "theme.light", icon: Sun },
  { value: "dark", label: "theme.dark", icon: Moon },
] as const;

function isThemePreference(value: string): value is ThemePreference {
  return choices.some((choice) => choice.value === value);
}

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
          {choices.map(({ value, label, icon: Icon }) => (
            <DropdownMenuRadioItem key={value} value={value}>
              <Icon />
              {t(label)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
});
