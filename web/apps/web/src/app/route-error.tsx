import { TriangleAlert } from "lucide-react";
import { useEffect } from "react";
import { Link, useRouteError } from "react-router";

import { entryTitleClass } from "../components/entry-card";
import { StatusPage, statusActionsClass, statusBodyClass } from "../components/status-page";
import { Button, buttonClass } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { ApiError } from "../services/api";
import { arrived } from "./arrival";
import { useCornerBack } from "./corner";
import { useDocumentTitle } from "./document-title";

/**
 * RouteError is the error boundary of the routes: an exception while a page
 * loads (a chunk of an older build that is gone) or renders. It shows the
 * API's error code, if any, and nothing else of the exception, which goes to
 * the console. In place of a page with a left column it has none: it gives
 * the top right back (M13/P2/S1). Centred, as a status page, its ways on
 * Reload and the home, arrived at (M13/P5/S1, decisions 921, 924; 00-M13
 * 4.2.3's way back).
 */
export function RouteError() {
  useCornerBack();
  const error = useRouteError();
  const t = useT();
  useDocumentTitle(t("error.title"));
  useEffect(() => {
    console.error(error);
  }, [error]);
  return (
    <StatusPage icon={TriangleAlert} alert>
      <h1 className={entryTitleClass}>{t("error.title")}</h1>
      <p className={statusBodyClass}>{t("error.body")}</p>
      {error instanceof ApiError && error.code ? (
        <p className="text-compact text-faint">{t("error.code", { code: error.code })}</p>
      ) : null}
      <div className={statusActionsClass}>
        <Button variant="outline" size="sm" shape="pill" onClick={() => window.location.reload()}>
          {t("error.reload")}
        </Button>
        <Link to="/" state={arrived} className={buttonClass({ variant: "outline", size: "sm", shape: "pill" })}>
          {t("notFound.home")}
        </Link>
      </div>
    </StatusPage>
  );
}
