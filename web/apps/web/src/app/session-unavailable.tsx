import { useRef } from "react";

import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { useDocumentTitle } from "./document-title";
import { useSignOut } from "./sign-out";

/**
 * SessionUnavailable stands in for a page while the session cannot be used
 * for now: a refresh or the account's load failed for a passing reason
 * (429, 5xx, no network). The tab is still signed in, and the address
 * stays (M1/P5 design 3.5). Signing out works all the same: it forgets the
 * session in this browser even when the server cannot be told, asking
 * first while an edit has changes not saved (useSignOut). Under a page
 * (the invitation's) the page keeps naming the tab.
 */
export function SessionUnavailable({ onRetry }: { onRetry: () => void }) {
  const t = useT();
  const leave = useRef<HTMLButtonElement>(null);
  const { signOut, dialog } = useSignOut(() => leave.current?.focus());
  return (
    <section role="alert" className="space-y-3">
      <h1 className="text-xl font-semibold">{t("session.unavailableTitle")}</h1>
      <p className="text-muted-foreground">{t("session.unavailableBody")}</p>
      <div className="flex gap-2">
        <Button variant="outline" onClick={onRetry}>
          {t("session.retry")}
        </Button>
        <Button ref={leave} variant="ghost" onClick={signOut}>
          {t("session.signOut")}
        </Button>
      </div>
      {dialog}
    </section>
  );
}

/** SessionUnavailablePage is SessionUnavailable in place of a whole page: it names the tab as well. */
export function SessionUnavailablePage({ onRetry }: { onRetry: () => void }) {
  const t = useT();
  useDocumentTitle(t("session.unavailableTitle"));
  return <SessionUnavailable onRetry={onRetry} />;
}
