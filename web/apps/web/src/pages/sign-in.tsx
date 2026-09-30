import { observer } from "mobx-react-lite";
import { Link, useSearchParams } from "react-router";
import useSWR from "swr";

import { keepNext } from "../app/next-path";
import { useT } from "../i18n/i18n";
import { useStore } from "../stores/context";
import { CredentialsForm, type Credentials, type LocalProblems } from "./credentials-form";

function check({ email, password }: Credentials): LocalProblems {
  return {
    ...(email.trim() === "" && { email: "field.required" }),
    ...(password === "" && { password: "field.required" }),
  };
}

/** SignInPage signs in with an e-mail address and a password; it links to sign-up while the instance takes new accounts. */
export const SignInPage = observer(function SignInPage() {
  const { auth, instance } = useStore();
  const t = useT();
  const [params] = useSearchParams();
  useSWR("instance", () => instance.load());

  return (
    <section className="mx-auto max-w-sm space-y-6">
      <h1 className="text-2xl font-semibold">{t("signIn.title")}</h1>
      <CredentialsForm
        passwordAutoComplete="current-password"
        submitLabel={t("signIn.submit")}
        check={check}
        submit={({ email, password }) => auth.signIn(email.trim(), password)}
      />
      {instance.info?.signup_enabled === true && (
        <p className="text-sm text-muted-foreground">
          {t("signIn.noAccount")}{" "}
          <Link to={keepNext("/sign-up", params)} className="underline underline-offset-4">
            {t("signIn.signUp")}
          </Link>
        </p>
      )}
    </section>
  );
});
