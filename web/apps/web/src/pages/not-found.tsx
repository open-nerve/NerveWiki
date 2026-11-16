import { Link, useLocation } from "react-router";

import { useArrivalFocus } from "../app/arrival";
import { useDocumentTitle } from "../app/document-title";
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
 * navigation was (M12/P2/S1).
 */
export function NotFoundPage({ replacing = false }: { replacing?: boolean }) {
  const t = useT();
  useDocumentTitle(t("notFound.title"));
  const heading = useArrivalFocus<HTMLHeadingElement>(locatedOf(useLocation().state) !== undefined, replacing);
  return (
    <section className="space-y-2">
      <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold outline-none">
        {t("notFound.title")}
      </h1>
      <p className="text-muted-foreground">{t("notFound.body")}</p>
      <Link to="/" className="underline underline-offset-4">
        {t("notFound.home")}
      </Link>
    </section>
  );
}
