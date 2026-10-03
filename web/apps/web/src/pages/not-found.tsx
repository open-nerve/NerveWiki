import { Link } from "react-router";

import { useDocumentTitle } from "../app/document-title";
import { useT } from "../i18n/i18n";

/** NotFoundPage answers a path that is no page of the app. */
export function NotFoundPage() {
  const t = useT();
  useDocumentTitle(t("notFound.title"));
  return (
    <section className="space-y-2">
      <h1 className="text-2xl font-semibold">{t("notFound.title")}</h1>
      <p className="text-muted-foreground">{t("notFound.body")}</p>
      <Link to="/" className="underline underline-offset-4">
        {t("notFound.home")}
      </Link>
    </section>
  );
}
