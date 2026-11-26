import { observer } from "mobx-react-lite";
import { Link, useSearchParams } from "react-router";
import useSWR from "swr";

import { useDocumentTitle } from "../app/document-title";
import { keepNext } from "../app/next-path";
import { EntryCard, entryTitleClass } from "../components/entry-card";
import { useT } from "../i18n/i18n";
import { useStore } from "../stores/context";
import { CredentialsForm, signInProblems } from "./credentials-form";

/**
 * SignInPage signs in with an e-mail address and a password; it links to sign-up while the instance takes new accounts.
 * In the entry's card, under the product's name (M13/P5/S1, decision 920).
 */
export const SignInPage = observer(function SignInPage() {
  const { auth, instance } = useStore();
  const t = useT();
  useDocumentTitle(t("signIn.title"));
  const [params] = useSearchParams();
  useSWR("instance", () => instance.load());

  return (
    <EntryCard>
      <h1 className={entryTitleClass}>{t("signIn.title")}</h1>
      <CredentialsForm
        passwordAutoComplete="current-password"
        submitLabel={t("signIn.submit")}
        check={signInProblems}
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
    </EntryCard>
  );
});
