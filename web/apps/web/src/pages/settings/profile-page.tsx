import { observer } from "mobx-react-lite";
import { useState } from "react";

import { DisplayNameForm } from "../../app/display-name-form";
import { isThemePreference, themeChoices } from "../../app/theme-menu";
import { useT } from "../../i18n/i18n";
import { isLocale, locales } from "../../i18n/locale";
import { useAccount, useStore } from "../../stores/context";

/** ProfilePage shows the account's name and address, and this browser's preferences (M1/P6 design 3.3). */
export function ProfilePage() {
  return (
    <div className="space-y-10">
      <ProfileSection />
      <PreferencesSection />
    </div>
  );
}

const ProfileSection = observer(function ProfileSection() {
  const { me } = useAccount();
  const t = useT();
  const [saved, setSaved] = useState(false);
  return (
    <section className="max-w-md space-y-6">
      <h2 className="text-lg font-semibold">{t("settings.profile")}</h2>
      <DisplayNameForm
        hint={t("profile.displayNameHint")}
        submitLabel={t("profile.save")}
        saved={() => setSaved(true)}
        onEdit={() => setSaved(false)}
        status={saved && <output className="text-sm text-muted-foreground">{t("profile.saved")}</output>}
      />
      <div className="space-y-1">
        <h3 className="text-sm font-medium">{t("form.email")}</h3>
        <p>{me.email}</p>
        <p className="text-sm text-muted-foreground">{t("profile.emailHint")}</p>
      </div>
    </section>
  );
});

/**
 * PreferencesSection chooses the theme and the language of this browser:
 * the same preferences as the top bar's menus, applied at once and kept on
 * the device, not on the server.
 */
const PreferencesSection = observer(function PreferencesSection() {
  const { preferences } = useStore();
  const t = useT();
  return (
    <section className="max-w-md space-y-6">
      <div className="space-y-1">
        <h2 className="text-lg font-semibold">{t("preferences.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("preferences.hint")}</p>
      </div>
      <fieldset className="space-y-2">
        <legend className="mb-2 text-sm font-medium">{t("theme.label")}</legend>
        {themeChoices.map(({ value, label }) => (
          <label key={value} className="flex items-center gap-2 text-sm">
            <input
              type="radio"
              name="theme"
              value={value}
              checked={preferences.theme === value}
              onChange={(event) => isThemePreference(event.target.value) && preferences.setTheme(event.target.value)}
            />
            {t(label)}
          </label>
        ))}
      </fieldset>
      <fieldset className="space-y-2">
        <legend className="mb-2 text-sm font-medium">{t("language.label")}</legend>
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
      </fieldset>
    </section>
  );
});
