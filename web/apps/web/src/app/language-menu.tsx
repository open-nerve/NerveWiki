import { Languages } from "lucide-react";
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
import { isLocale, locales } from "../i18n/locale";
import { useStore } from "../stores/context";

export const LanguageMenu = observer(function LanguageMenu() {
  const { preferences } = useStore();
  const t = useT();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t("language.label")}>
          <Languages />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuRadioGroup
          value={preferences.locale}
          onValueChange={(value) => isLocale(value) && preferences.setLocale(value)}
        >
          {locales.map((locale) => (
            <DropdownMenuRadioItem key={locale.value} value={locale.value} lang={locale.value}>
              {locale.name}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
});
