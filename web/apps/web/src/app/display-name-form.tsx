import { observer } from "mobx-react-lite";
import { useState, type FormEvent, type ReactNode } from "react";

import { FormField } from "../components/form-field";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { useAccount } from "../stores/context";
import { useForm } from "./form";

type DisplayNameFormProps = {
  hint: string;
  submitLabel: string;
  /** What the form does once the name is saved; its failure shows as the save's does. */
  saved: () => Promise<void> | void;
  /** Shown beside the button, such as the saved state. */
  status?: ReactNode;
  /** Called on every edit, before the name is saved again. */
  onEdit?: () => void;
  /** The button spans the form, as onboarding's steps have it. */
  wide?: boolean;
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
  wide = false,
}: DisplayNameFormProps) {
  const { account, me } = useAccount();
  const t = useT();
  const [name, setName] = useState(me.display_name);
  const { ref, sending, banner, problemOf, submit } = useForm(["display_name"]);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const displayName = name.trim();
    void submit(displayName === "" ? { display_name: "field.required" } : {}, async () => {
      if (displayName !== me.display_name) {
        await account.update({ display_name: displayName });
      }
      await saved();
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <FormField
        label={t("account.displayName")}
        name="display_name"
        autoComplete="nickname"
        value={name}
        error={problemOf("display_name")}
        hint={hint}
        onChange={(event) => {
          setName(event.target.value);
          onEdit?.();
        }}
      />
      <div className="flex items-center gap-3">
        <Button type="submit" className={wide ? "w-full" : undefined} disabled={sending}>
          {submitLabel}
        </Button>
        {status}
      </div>
    </form>
  );
});
