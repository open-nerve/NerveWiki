import { useState, type FormEvent } from "react";

import { useForm } from "../../app/form";
import { passwordLength } from "../../app/password-length";
import { FormField } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { useAccount } from "../../stores/context";
import { DeactivateDialog } from "./deactivate-dialog";

/** SecurityPage changes the password and deactivates the account (M1/P6 design 3.4, 3.5). */
export function SecurityPage() {
  const t = useT();
  return (
    <div className="space-y-10">
      <section className="max-w-md space-y-6">
        <h2 className="text-lg font-semibold">{t("security.passwordTitle")}</h2>
        <ChangePasswordForm />
      </section>
      <section className="max-w-md space-y-3">
        <h2 className="text-lg font-semibold">{t("deactivate.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("deactivate.body")}</p>
        <DeactivateDialog />
      </section>
    </div>
  );
}

type Passwords = { current_password: string; new_password: string };

const fields: readonly (keyof Passwords)[] = ["current_password", "new_password"];

/**
 * ChangePasswordForm asks for the current password and a new one. A wrong
 * current password shows under its field. Once changed, the account's
 * other sessions are over; this one and the access tokens go on, so the
 * page stays as it is, with the fields emptied.
 */
function ChangePasswordForm() {
  const { account } = useAccount();
  const t = useT();
  const [passwords, setPasswords] = useState<Passwords>({ current_password: "", new_password: "" });
  const [changed, setChanged] = useState(false);
  const { ref, sending, banner, problemOf, submit } = useForm(fields, {
    "identity.current_password_incorrect": "current_password",
  });

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setChanged(false);
    const length = passwordLength(passwords.new_password);
    const found = {
      ...(passwords.current_password === "" && { current_password: "field.required" as const }),
      ...(length !== undefined && { new_password: `field.new_password.${length}` as const }),
    };
    const sent = await submit(found, () => account.changePassword(passwords.current_password, passwords.new_password));
    if (sent) {
      setPasswords({ current_password: "", new_password: "" });
      setChanged(true);
    }
  }

  const edit = (field: keyof Passwords, value: string) => {
    setPasswords({ ...passwords, [field]: value });
    setChanged(false);
  };

  return (
    <form ref={ref} noValidate onSubmit={(event) => void onSubmit(event)} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <FormField
        label={t("security.currentPassword")}
        type="password"
        name="current_password"
        autoComplete="current-password"
        value={passwords.current_password}
        error={problemOf("current_password")}
        onChange={(event) => edit("current_password", event.target.value)}
      />
      <FormField
        label={t("security.newPassword")}
        type="password"
        name="new_password"
        autoComplete="new-password"
        value={passwords.new_password}
        error={problemOf("new_password")}
        hint={t("security.newPasswordHint")}
        onChange={(event) => edit("new_password", event.target.value)}
      />
      <Button type="submit" disabled={sending}>
        {t("security.change")}
      </Button>
      {changed && <output className="block text-sm text-muted-foreground">{t("security.changed")}</output>}
    </form>
  );
}
