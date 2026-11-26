import { observer } from "mobx-react-lite";

import { NameRow } from "../../app/name-row";
import { SettingsCard, SettingsRow, SettingsSection } from "../../app/settings-page";
import { useT } from "../../i18n/i18n";
import { useAccount } from "../../stores/context";

/**
 * ProfilePage shows the account's name and address (M1/P6 design 3.3) in a card (M13/P3/S1, decision 820): the name
 * others see, saved as it is changed (NameRow), checked here only for being empty, as onboarding's step checks it;
 * the e-mail address, which only the server's administrator changes, a value it shows. The page's h1 is its name; the
 * theme and the language are a page of their own (AppearancePage).
 */
export const ProfilePage = observer(function ProfilePage() {
  const { account, me } = useAccount();
  const t = useT();
  return (
    <SettingsSection>
      <SettingsCard>
        <NameRow
          current={me.display_name}
          field="display_name"
          label={t("account.displayName")}
          description={t("profile.displayNameHint")}
          autoComplete="nickname"
          check={(name) => (name.trim() === "" ? "field.required" : undefined)}
          save={(name) => account.update({ display_name: name })}
          saveLabel={t("profile.save")}
          savedLabel={t("profile.saved")}
        />
        <SettingsRow title={t("form.email")} description={t("profile.emailHint")} value={me.email} />
      </SettingsCard>
    </SettingsSection>
  );
});
