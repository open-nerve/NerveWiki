import { useEffect } from "react";
import { useRouteError } from "react-router";

import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { ApiError } from "../services/api";

/**
 * RouteError is the error boundary of the routes: an exception while a page
 * loads (a chunk of an older build that is gone) or renders. It shows the
 * API's error code, if any, and nothing else of the exception, which goes to
 * the console.
 */
export function RouteError() {
  const error = useRouteError();
  const t = useT();
  useEffect(() => {
    console.error(error);
  }, [error]);
  return (
    <section role="alert" className="space-y-2">
      <h1 className="text-2xl font-semibold">{t("error.title")}</h1>
      <p className="text-muted-foreground">{t("error.body")}</p>
      {error instanceof ApiError && error.code ? (
        <p className="text-muted-foreground">{t("error.code", { code: error.code })}</p>
      ) : null}
      <Button variant="outline" onClick={() => window.location.reload()}>
        {t("error.reload")}
      </Button>
    </section>
  );
}
