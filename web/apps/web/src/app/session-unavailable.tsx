import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";

/**
 * SessionUnavailable stands in for a page while the session cannot be used
 * for now: a refresh or the account's load failed for a passing reason
 * (429, 5xx, no network). The tab is still signed in, and the address
 * stays (M1/P5 design 3.5).
 */
export function SessionUnavailable({ onRetry }: { onRetry: () => void }) {
  const t = useT();
  return (
    <section role="alert" className="space-y-3">
      <h1 className="text-xl font-semibold">{t("session.unavailableTitle")}</h1>
      <p className="text-muted-foreground">{t("session.unavailableBody")}</p>
      <Button variant="outline" onClick={onRetry}>
        {t("session.retry")}
      </Button>
    </section>
  );
}

/** Loading stands in for a page while the session or the account is decided. */
export function Loading() {
  const t = useT();
  return <p className="text-muted-foreground">{t("session.loading")}</p>;
}
