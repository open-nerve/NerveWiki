import { observer } from "mobx-react-lite";

import { SettingsCard, SettingsRow, SettingsSection } from "../../app/settings-page";
import { useT } from "../../i18n/i18n";
import { isLocale, locales } from "../../i18n/locale";
import { useStore } from "../../stores/context";
import { isThemePreference, themePreferences } from "../../stores/preferences.store";

/**
 * AppearancePage chooses this browser's theme and language (00-M13 4.4.1; M13/P3/S1, decision 824): the same
 * preferences as the account menu's Appearance, once the profile page's (M1/P6 design 3.3), applied as they are chosen
 * and kept on the device, never sent. A card of two rows, each a group of native radio buttons named by its row's
 * title, the theme's described by its row's description. The choices are read where the controls are drawn, in
 * SettingsRow's render, which it observes (decision 849).
 */
export const AppearancePage = observer(function AppearancePage() {
  const { preferences } = useStore();
  const t = useT();
  return (
    <SettingsSection>
      <SettingsCard>
        <SettingsRow title={t("theme.label")} description={t("preferences.hint")}>
          {({ title, description }) => (
            <div
              role="radiogroup"
              aria-labelledby={title}
              aria-describedby={description}
              className="flex flex-wrap items-center gap-x-4 gap-y-2"
            >
              {themePreferences.map((value) => (
                <label key={value} className="flex items-center gap-2 text-sm">
                  <input
                    type="radio"
                    name="theme"
                    value={value}
                    checked={preferences.theme === value}
                    onChange={(event) =>
                      isThemePreference(event.target.value) && preferences.setTheme(event.target.value)
                    }
                  />
                  {t(`theme.${value}`)}
                </label>
              ))}
            </div>
          )}
        </SettingsRow>
        <SettingsRow title={t("language.label")}>
          {({ title }) => (
            <div role="radiogroup" aria-labelledby={title} className="flex flex-wrap items-center gap-x-4 gap-y-2">
              {locales.map(({ value, name }) => (
                <label key={value} className="flex items-center gap-2 text-sm" lang={value}>
                  <input
                    type="radio"
                    name="locale"
                    value={value}
                    checked={preferences.locale === value}
                    onChange={(event) => isLocale(event.target.value) && preferences.setLocale(event.target.value)}
                  />
                  {name}
                </label>
              ))}
            </div>
          )}
        </SettingsRow>
      </SettingsCard>
    </SettingsSection>
  );
});
