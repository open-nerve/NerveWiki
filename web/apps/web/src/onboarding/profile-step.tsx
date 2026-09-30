import { observer } from "mobx-react-lite";
import { useState, type FormEvent } from "react";

import { formErrors, type FieldMessage } from "../app/problem-messages";
import { FormField, useFocusOnInvalid } from "../components/form-field";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { useAccount } from "../stores/context";

/**
 * ProfileStep asks for the name others see, filled in with the one the
 * account has (at sign-up, what comes before the @ of its e-mail address).
 * Continue saves the name if it was changed, then completes the step; both
 * go out one after the other, and both may be sent again after a failure.
 */
export const ProfileStep = observer(function ProfileStep({ complete }: { complete: () => Promise<void> }) {
  const { account, me } = useAccount();
  const t = useT();
  const [name, setName] = useState(me.display_name);
  const [local, setLocal] = useState<FieldMessage>();
  const [failure, setFailure] = useState<unknown>();
  const [sending, setSending] = useState(false);
  const [failures, setFailures] = useState(0);
  const form = useFocusOnInvalid(failures);
  const server = formErrors(failure, t, ["display_name"]);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const displayName = name.trim();
    setLocal(displayName === "" ? "field.required" : undefined);
    setFailure(undefined);
    if (displayName === "") {
      setFailures(failures + 1);
      return;
    }
    setSending(true);
    try {
      if (displayName !== me.display_name) {
        await account.update({ display_name: displayName });
      }
      await complete();
    } catch (error) {
      setFailure(error);
      setFailures(failures + 1);
    } finally {
      setSending(false);
    }
  }

  return (
    <form ref={form} noValidate onSubmit={(event) => void onSubmit(event)} className="space-y-4">
      {server.banner !== undefined && <Alert>{server.banner}</Alert>}
      <FormField
        label={t("onboarding.profile.displayName")}
        name="display_name"
        autoComplete="nickname"
        value={name}
        error={local === undefined ? server.fields.display_name : t(local)}
        hint={t("onboarding.profile.hint")}
        onChange={(event) => setName(event.target.value)}
      />
      <Button type="submit" className="w-full" disabled={sending}>
        {t("onboarding.continue")}
      </Button>
    </form>
  );
});
