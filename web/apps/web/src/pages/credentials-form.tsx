import { useState, type FormEvent } from "react";

import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { FormField } from "../components/form-field";
import { formErrors, type FieldMessage } from "../app/problem-messages";
import { useT } from "../i18n/i18n";

export type Credentials = { email: string; password: string };

/** The fields' problems found before sending: a message for each field that has one. */
export type LocalProblems = Partial<Record<keyof Credentials, FieldMessage>>;

type CredentialsFormProps = {
  /** "current-password" signs in, "new-password" signs up: password managers fill or offer by it. */
  passwordAutoComplete: "current-password" | "new-password";
  passwordHint?: string;
  submitLabel: string;
  check: (credentials: Credentials) => LocalProblems;
  submit: (credentials: Credentials) => Promise<void>;
};

/**
 * CredentialsForm is the e-mail and password form of the sign-in and
 * sign-up pages (M1/P5 design 3.6). It sends nothing while check finds a
 * problem; the server's problems show under their field, the rest above
 * the form; what was typed stays. Once submit resolves, the session has
 * changed: the route guards take the tab on, the form does not.
 */
export function CredentialsForm({
  passwordAutoComplete,
  passwordHint,
  submitLabel,
  check,
  submit,
}: CredentialsFormProps) {
  const t = useT();
  const [credentials, setCredentials] = useState<Credentials>({ email: "", password: "" });
  const [local, setLocal] = useState<LocalProblems>({});
  const [failure, setFailure] = useState<unknown>();
  const [sending, setSending] = useState(false);
  const server = formErrors(failure, t);

  function problemOf(field: keyof Credentials): string | undefined {
    const key = local[field];
    return key === undefined ? server.fields[field] : t(key);
  }

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const found = check(credentials);
    setLocal(found);
    setFailure(undefined);
    if (Object.keys(found).length > 0) {
      return;
    }
    setSending(true);
    try {
      await submit(credentials);
    } catch (error) {
      setFailure(error);
      setSending(false);
    }
  }

  return (
    <form noValidate onSubmit={(event) => void onSubmit(event)} className="space-y-4">
      {server.banner !== undefined && <Alert>{server.banner}</Alert>}
      <FormField
        label={t("form.email")}
        type="email"
        name="email"
        autoComplete="username"
        value={credentials.email}
        error={problemOf("email")}
        onChange={(event) => setCredentials({ ...credentials, email: event.target.value })}
      />
      <FormField
        label={t("form.password")}
        type="password"
        name="password"
        autoComplete={passwordAutoComplete}
        value={credentials.password}
        error={problemOf("password")}
        hint={passwordHint}
        onChange={(event) => setCredentials({ ...credentials, password: event.target.value })}
      />
      <Button type="submit" className="w-full" disabled={sending}>
        {submitLabel}
      </Button>
    </form>
  );
}
