import { FileQuestionMark } from "lucide-react";
import { Link, useLocation } from "react-router";

import { arrived, useArrivalFocus } from "../app/arrival";
import { useDocumentTitle } from "../app/document-title";
import { entryTitleClass } from "../components/entry-card";
import { StatusPage, statusActionsClass, statusBodyClass } from "../components/status-page";
import { buttonClass } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { locatedOf } from "./page/attachment-row";

/**
 * NotFoundPage answers a path that is no page of the app. Arrived at, or
 * gone to for an attachment (Located), its title takes the focus, as a
 * page's would: a link that led to a page deleted meanwhile, a search's
 * hit say, leaves the focus nowhere else (M8 closeout B-M1). Shown later
 * for the same navigation, the page deleted under a reader working
 * elsewhere, it takes the focus only from no one (useArrivalFocus; K1-1,
 * K2-1). In place of what this navigation showed (replacing: a changeset
 * an undo found gone), it takes the focus only from no one, whatever the
 * navigation was (M12/P2/S1). Centred, as a status page (M13/P5/S1,
 * decision 921), in a notebook's main too; its way home a link drawn as a
 * button, arrived at: the landing passes it on, the workspace's home's
 * title taking the focus (decision 924).
 */
export function NotFoundPage({ replacing = false }: { replacing?: boolean }) {
  const t = useT();
  useDocumentTitle(t("notFound.title"));
  const heading = useArrivalFocus<HTMLHeadingElement>(locatedOf(useLocation().state) !== undefined, replacing);
  return (
    <StatusPage icon={FileQuestionMark}>
      <h1 ref={heading} tabIndex={-1} className={entryTitleClass}>
        {t("notFound.title")}
      </h1>
      <p className={statusBodyClass}>{t("notFound.body")}</p>
      <div className={statusActionsClass}>
        <Link to="/" state={arrived} className={buttonClass({ variant: "outline", size: "sm", shape: "pill" })}>
          {t("notFound.home")}
        </Link>
      </div>
    </StatusPage>
  );
}
