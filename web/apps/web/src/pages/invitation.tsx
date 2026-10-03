import { observer } from "mobx-react-lite";
import { useState, type FormEvent } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import useSWR from "swr";

import { arrived } from "../app/arrival";
import { useDocumentTitle } from "../app/document-title";
import { useForm } from "../app/form";
import { useSession } from "../app/guards";
import { linkOf } from "../app/invitation-link";
import { useMounted } from "../app/mounted";
import { NotLoaded } from "../app/not-loaded";
import { SessionUnavailable } from "../app/session-unavailable";
import { Loading } from "../components/loading";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import type { InvitationLink } from "../services/invitation.service";
import { useStore, useWorkspaces } from "../stores/context";
import { CredentialsForm, signInProblems, signUpProblems } from "./credentials-form";

/**
 * InvitationPage is the page of an invitation's link (M2/P6 design 3.4),
 * outside the guards: whoever holds the link sees what it invites to; one
 * signed out signs in or up here, one signed in accepts and goes into the
 * workspace. Signing in, up or out only changes the session (v0.1 design
 * 13.2, rule 10): the next generation shows this page again, at the same
 * address, whose fragment holds the token, which never goes into a next.
 */
export const InvitationPage = observer(function InvitationPage() {
  const { id = "" } = useParams();
  const { hash } = useLocation();
  const t = useT();
  useDocumentTitle(t("invitation.title"));
  const link = linkOf(id, hash);
  return (
    <section className="mx-auto max-w-sm space-y-6">
      <h1 className="text-2xl font-semibold">{t("invitation.title")}</h1>
      {link === undefined ? <Alert>{t("problem.workspace.invitation_not_found")}</Alert> : <Invitation link={link} />}
    </section>
  );
});

/** Invitation is what link invites to, then what the session lets do with it. */
const Invitation = observer(function Invitation({ link }: { link: InvitationLink }) {
  const { auth, invitationPreviews } = useStore();
  const { status } = useSession();
  const t = useT();
  const preview = useSWR(["invitation-preview", link.id, link.token], () => invitationPreviews.preview(link));
  if (preview.data === undefined) {
    return <NotLoaded error={preview.error} retry={() => void preview.mutate()} />;
  }
  const { workspace, role } = preview.data;
  let next;
  switch (status) {
    case "starting":
      next = <Loading />;
      break;
    case "unavailable":
      next = <SessionUnavailable onRetry={() => void auth.retry()} />;
      break;
    case "signed-out":
      next = <SignInOrUp link={link} />;
      break;
    case "signed-in":
      next = <Accept link={link} />;
      break;
  }
  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <p className="text-lg">{t("invitation.invites", { workspace: workspace.name })}</p>
        <p className="text-sm text-muted-foreground">{t("invitation.role", { role: t(`role.${role}`) })}</p>
      </div>
      {next}
    </div>
  );
});

/**
 * SignInOrUp signs in, or signs up with the invitation, which lets its
 * address register while sign-up is closed; either way the session
 * changes, and this page shows again, signed in.
 */
function SignInOrUp({ link }: { link: InvitationLink }) {
  const { auth } = useStore();
  const t = useT();
  const [signingUp, setSigningUp] = useState(false);
  const toggle = "underline underline-offset-4";
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        {t(signingUp ? "invitation.signUpHint" : "invitation.signInHint")}
      </p>
      {signingUp ? (
        <CredentialsForm
          key="sign-up"
          passwordAutoComplete="new-password"
          passwordHint={t("signUp.passwordHint")}
          submitLabel={t("signUp.submit")}
          check={signUpProblems}
          submit={({ email, password }) => auth.signUp(email.trim(), password, link)}
          texts={{ "identity.signup_disabled": "invitation.signUpRefused" }}
        />
      ) : (
        <CredentialsForm
          key="sign-in"
          passwordAutoComplete="current-password"
          submitLabel={t("signIn.submit")}
          check={signInProblems}
          submit={({ email, password }) => auth.signIn(email.trim(), password)}
        />
      )}
      <p className="text-sm text-muted-foreground">
        {t(signingUp ? "signUp.haveAccount" : "signIn.noAccount")}{" "}
        <button type="button" className={toggle} onClick={() => setSigningUp(!signingUp)}>
          {t(signingUp ? "signUp.signIn" : "signIn.signUp")}
        </button>
      </p>
    </div>
  );
}

/**
 * Accept accepts the invitation as the signed-in account, then goes into
 * the workspace, onboarding first if the account has steps left; an
 * account of another address is told so, and can sign out to sign in with
 * that one.
 */
const Accept = observer(function Accept({ link }: { link: InvitationLink }) {
  const { account, auth } = useStore();
  const workspaces = useWorkspaces();
  const t = useT();
  const navigate = useNavigate();
  const mounted = useMounted();
  if (account === undefined) {
    throw new Error("Accept is shown signed out");
  }
  const me = useSWR("me", () => account.load());
  const { ref, sending, banner, submit } = useForm([]);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void submit({}, async () => {
      const joined = await workspaces.accept(link);
      // The used link is not a page to come back to; the workspace's heading takes the focus. One who left the
      // page meanwhile is not taken back: the switcher lists the workspace.
      if (mounted()) {
        void navigate(`/${joined.slug}`, { replace: true, state: arrived });
      }
    });
  }

  if (account.me === undefined) {
    return <NotLoaded error={me.error} retry={() => void me.mutate()} />;
  }
  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <p className="text-sm text-muted-foreground">
        {t("invitation.signedInAs", { name: account.me.display_name, email: account.me.email })}
      </p>
      <div className="flex flex-wrap gap-2">
        <Button type="submit" disabled={sending}>
          {t("invitation.accept")}
        </Button>
        <Button type="button" variant="outline" onClick={() => void auth.signOut()}>
          {t("invitation.signOut")}
        </Button>
      </div>
    </form>
  );
});
