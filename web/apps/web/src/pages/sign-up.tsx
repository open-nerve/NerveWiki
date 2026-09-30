import { observer } from "mobx-react-lite";
import { Link, useSearchParams } from "react-router";
import useSWR from "swr";

import { keepNext } from "../app/next-path";
import { passwordLength } from "../app/password-length";
import { Loading } from "../components/loading";
import { useT } from "../i18n/i18n";
import { useStore } from "../stores/context";
import { CredentialsForm, type Credentials, type LocalProblems } from "./credentials-form";

function check({ email, password }: Credentials): LocalProblems {
  const length = passwordLength(password);
  return {
    ...(email.trim() === "" && { email: "field.required" }),
    ...(length !== undefined && { password: `field.password.${length}` as const }),
  };
}

/**
 * SignUpPage creates an account and signs in to it. While the instance
 * takes no new accounts, it says so instead of showing the form.
 */
export const SignUpPage = observer(function SignUpPage() {
  const { auth, instance } = useStore();
  const t = useT();
  const [params] = useSearchParams();
  const { error } = useSWR("instance", () => instance.load());
  const signInLink = (
    <Link to={keepNext("/sign-in", params)} className="underline underline-offset-4">
      {t("signUp.signIn")}
    </Link>
  );

  // Unknown: the form, whose sending answers 403 with its text if sign-up is closed.
  const open = instance.info?.signup_enabled ?? (error === undefined ? undefined : true);
  if (open === undefined) {
    return <Loading />;
  }
  return (
    <section className="mx-auto max-w-sm space-y-6">
      <h1 className="text-2xl font-semibold">{t("signUp.title")}</h1>
      {open ? (
        <>
          <CredentialsForm
            passwordAutoComplete="new-password"
            passwordHint={t("signUp.passwordHint")}
            submitLabel={t("signUp.submit")}
            check={check}
            submit={({ email, password }) => auth.signUp(email.trim(), password)}
          />
          <p className="text-sm text-muted-foreground">
            {t("signUp.haveAccount")} {signInLink}
          </p>
        </>
      ) : (
        <>
          <p>{t("signUp.closed")}</p>
          <p className="text-sm">{signInLink}</p>
        </>
      )}
    </section>
  );
});
