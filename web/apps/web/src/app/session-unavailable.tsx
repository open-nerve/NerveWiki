import { CloudOff } from "lucide-react";
import { useRef, type ReactNode } from "react";

import { entryTitleClass } from "../components/entry-card";
import { StatusPage, statusActionsClass, statusBodyClass } from "../components/status-page";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { useCornerBack } from "./corner";
import { useDocumentTitle } from "./document-title";
import { useSignOut } from "./sign-out";

/**
 * useUnavailable is what says the session cannot be used for now, as either shell lays it out: its title's and its
 * line's texts, and its two buttons, Try again and Sign out, with Sign out's dialog. Signing out works all the same: it
 * forgets the session in this browser even when the server cannot be told, asking first while an edit has changes not
 * saved (useSignOut), the focus given back to Sign out.
 */
function useUnavailable(
  onRetry: () => void,
  size?: "sm"
): { title: string; body: string; buttons: ReactNode; dialog: ReactNode } {
  const t = useT();
  const leave = useRef<HTMLButtonElement>(null);
  const { signOut, dialog } = useSignOut(() => leave.current?.focus());
  const shape = size === undefined ? undefined : "pill";
  return {
    title: t("session.unavailableTitle"),
    body: t("session.unavailableBody"),
    buttons: (
      <>
        <Button variant="outline" size={size} shape={shape} onClick={onRetry}>
          {t("session.retry")}
        </Button>
        <Button ref={leave} variant="ghost" size={size} shape={shape} onClick={signOut}>
          {t("session.signOut")}
        </Button>
      </>
    ),
    dialog,
  };
}

/**
 * SessionUnavailable says the session cannot be used for now under a page, the invitation's, while a refresh or the
 * account's load failed for a passing reason (429, 5xx, no network). The tab is still signed in, and the address stays
 * (M1/P5 design 3.5); the page keeps naming the tab. Its title is an h2, the page's one h1 the invitation's, its lines
 * left as the card's, its text the card's 14px (M13/P5/S1, decisions 922, 948). An alert, its title in it. In place of
 * a whole page, SessionUnavailablePage says it.
 */
export function SessionUnavailable({ onRetry }: { onRetry: () => void }) {
  const { title, body, buttons, dialog } = useUnavailable(onRetry);
  return (
    <section role="alert" className="space-y-3">
      <h2 className="text-base font-semibold">{title}</h2>
      <p className="text-sm text-muted-foreground">{body}</p>
      <div className="flex gap-2">{buttons}</div>
      {dialog}
    </section>
  );
}

/**
 * SessionUnavailablePage says the session cannot be used for now in place of a whole page, centred as a status page
 * (M13/P5/S1, decision 921): Try again and Sign out, no way home, the home needing the session too. It names the tab
 * as well. In place of a page with a left column it has none: it gives the top right back, Appearance there (M13/P2/S1,
 * decision 792).
 */
export function SessionUnavailablePage({ onRetry }: { onRetry: () => void }) {
  useCornerBack();
  const { title, body, buttons, dialog } = useUnavailable(onRetry, "sm");
  useDocumentTitle(title);
  return (
    <StatusPage icon={CloudOff} alert>
      <h1 className={entryTitleClass}>{title}</h1>
      <p className={statusBodyClass}>{body}</p>
      <div className={statusActionsClass}>{buttons}</div>
      {dialog}
    </StatusPage>
  );
}
