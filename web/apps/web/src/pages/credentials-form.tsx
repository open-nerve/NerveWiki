import { useState, type FormEvent } from "react";

import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { FormField } from "../components/form-field";
import { useForm, type LocalProblems as FormProblems } from "../app/form";
import type { ProblemTexts } from "../app/problem-messages";
import { passwordLength } from "../app/password-length";
import { useT } from "../i18n/i18n";

export type Credentials = { email: string; password: string };

/** The fields of the form, as the API names them. */
const fields: readonly (keyof Credentials)[] = ["email", "password"];

/** The fields' problems found before sending: a message for each field that has one. */
type LocalProblems = FormProblems<keyof Credentials>;

/** signInProblems is what a sign-in lacks before it goes out. */
export function signInProblems({ email, password }: Credentials): LocalProblems {
  return {
    ...(email.trim() === "" && { email: "field.required" }),
    ...(password === "" && { password: "field.required" }),
  };
}

/** signUpProblems is what a sign-up lacks before it goes out: a password of the length the server takes. */
export function signUpProblems({ email, password }: Credentials): LocalProblems {
  const length = passwordLength(password);
  return {
    ...(email.trim() === "" && { email: "field.required" }),
    ...(length !== undefined && { password: `field.password.${length}` as const }),
  };
}

type CredentialsFormProps = {
  /** "current-password" signs in, "new-password" signs up: password managers fill or offer by it. */
  passwordAutoComplete: "current-password" | "new-password";
  passwordHint?: string;
  submitLabel: string;
  check: (credentials: Credentials) => LocalProblems;
  submit: (credentials: Credentials) => Promise<void>;
  /** The form's own texts for some problem codes. */
  texts?: ProblemTexts;
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
  texts,
}: CredentialsFormProps) {
  const t = useT();
  const [credentials, setCredentials] = useState<Credentials>({ email: "", password: "" });
  const { ref, sending, banner, problemOf, submit: send } = useForm(fields, { texts });

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void send(check(credentials), () => submit(credentials));
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
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
