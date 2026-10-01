import { Loading } from "../components/loading";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { errorText } from "./problem-messages";

/**
 * NotLoaded stands in for what a page cannot show yet: loading while it is
 * on its way, or why its load failed, with a way to try again. A load cut
 * by a change of session is loading: the next generation takes over.
 */
export function NotLoaded({ error, retry }: { error: unknown; retry: () => void }) {
  const t = useT();
  const failed = error === undefined ? undefined : errorText(error, t);
  if (failed === undefined) {
    return <Loading />;
  }
  return (
    <div className="space-y-3">
      <Alert>{failed}</Alert>
      <Button variant="outline" onClick={retry}>
        {t("status.retry")}
      </Button>
    </div>
  );
}
