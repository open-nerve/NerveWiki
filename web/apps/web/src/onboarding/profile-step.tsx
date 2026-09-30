import { DisplayNameForm } from "../app/display-name-form";
import { useT } from "../i18n/i18n";

/**
 * ProfileStep asks for the name others see, filled in with the one the
 * account has (at sign-up, what comes before the @ of its e-mail address).
 * Continue saves the name if it was changed, then completes the step; both
 * go out one after the other, and both may be sent again after a failure.
 */
export function ProfileStep({ complete }: { complete: () => Promise<void> }) {
  const t = useT();
  return (
    <DisplayNameForm hint={t("onboarding.profile.hint")} submitLabel={t("onboarding.continue")} saved={complete} />
  );
}
