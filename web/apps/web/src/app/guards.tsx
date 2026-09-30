import { observer } from "mobx-react-lite";
import { useSyncExternalStore } from "react";
import { Navigate, Outlet, useLocation, useSearchParams } from "react-router";
import useSWR from "swr";

import { pendingSteps } from "../onboarding/steps";
import { SessionChangedError } from "../session/token-manager";
import { useAccount, useStore } from "../stores/context";
import type { SessionState } from "../stores/auth.store";
import { safeNextPath, withNext } from "./next-path";
import { Loading, SessionUnavailable } from "./session-unavailable";

// The route guards (M1/P5 design 3.5): the only place that decides where
// the tab goes as its session changes. The pages never navigate after a
// sign-in, a sign-up or a sign-out: the session changes, and the guard of
// the page shown sends the tab on.

/** useSession is the tab's session state; the component renders again on every change. */
function useSession(): SessionState {
  const { auth } = useStore();
  return useSyncExternalStore(auth.subscribe, () => auth.state);
}

/** GuestOnly shows the sign-in and sign-up pages to a tab without a session; one with a session goes to next. */
export function GuestOnly() {
  const { status } = useSession();
  const [params] = useSearchParams();
  if (status === "signed-out") {
    return <Outlet />;
  }
  if (status === "starting") {
    return <Loading />;
  }
  return <Navigate replace to={safeNextPath(params.get("next")) ?? "/"} />;
}

/**
 * SignedIn shows every other page to a signed-in tab, once its account is
 * loaded; a tab without a session goes to the sign-in page, which comes
 * back here.
 */
export function SignedIn() {
  const { status } = useSession();
  const { auth } = useStore();
  const { pathname, search, hash } = useLocation();
  switch (status) {
    case "starting":
      return <Loading />;
    case "unavailable":
      return <SessionUnavailable onRetry={() => void auth.retry()} />;
    case "signed-out":
      return <Navigate replace to={withNext("/sign-in", pathname + search + hash)} />;
    case "signed-in":
      return <Account />;
  }
}

/** Account loads the signed-in account of this generation, then shows the page. */
const Account = observer(function Account() {
  const { account } = useStore();
  if (account === undefined) {
    throw new Error("a signed-in generation has no account store");
  }
  const { error, mutate } = useSWR("me", () => account.load());
  if (account.me !== undefined) {
    return <Outlet />;
  }
  // A load cut by a change of session: the next generation takes over.
  if (error === undefined || error instanceof SessionChangedError) {
    return <Loading />;
  }
  return <SessionUnavailable onRetry={() => void mutate()} />;
});

/**
 * Onboarded shows the pages of an account done with onboarding; one with a
 * step left goes to the onboarding page, which comes back here (M1/P5
 * design 3.7).
 */
export const Onboarded = observer(function Onboarded() {
  const { me } = useAccount();
  const { pathname, search, hash } = useLocation();
  if (pendingSteps(me).length > 0) {
    return <Navigate replace to={withNext("/onboarding", pathname + search + hash)} />;
  }
  return <Outlet />;
});
