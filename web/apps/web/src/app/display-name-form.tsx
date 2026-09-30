import { observer } from "mobx-react-lite";
import { useState, type FormEvent, type ReactNode } from "react";

import { FormField, useFocusOnInvalid } from "../components/form-field";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { useAccount } from "../stores/context";
import { formErrors, type FieldMessage } from "./problem-messages";

type DisplayNameFormProps = {
  hint: string;
  submitLabel: string;
  /** What the form does once the name is saved; its failure shows as the save's does. */
  saved: () => Promise<void> | void;
  /** Shown beside the button, such as the saved state. */
  status?: ReactNode;
  /** Called on every edit, before the name is saved again. */
  onEdit?: () => void;
  className?: string;
};

/**
 * DisplayNameForm edits the name others see, filled in with the one the
 * account has: onboarding's profile step and the profile settings. The name
 * goes out only when it changed (without its surrounding blanks, as the
 * server keeps it), through the account's queue of changes; then saved runs.
 * After a failure both may be done again: saving a name is idempotent.
 */
export const DisplayNameForm = observer(function DisplayNameForm({
  hint,
  submitLabel,
  saved,
  status,
  onEdit,
  className,
}: DisplayNameFormProps) {
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
      await saved();
    } catch (error) {
      setFailure(error);
      setFailures(failures + 1);
    } finally {
      setSending(false);
    }
  }

  return (
    <form ref={form} noValidate onSubmit={(event) => void onSubmit(event)} className={className ?? "space-y-4"}>
      {server.banner !== undefined && <Alert>{server.banner}</Alert>}
      <FormField
        label={t("account.displayName")}
        name="display_name"
        autoComplete="nickname"
        value={name}
        error={local === undefined ? server.fields.display_name : t(local)}
        hint={hint}
        onChange={(event) => {
          setName(event.target.value);
          onEdit?.();
        }}
      />
      <div className="flex items-center gap-3">
        <Button type="submit" disabled={sending}>
          {submitLabel}
        </Button>
        {status}
      </div>
    </form>
  );
});
