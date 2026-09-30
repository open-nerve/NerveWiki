import { useT } from "../i18n/i18n";

/** Loading stands in for a page, or a part of it, while what it shows is on its way; output is a status. */
export function Loading() {
  const t = useT();
  return <output className="block text-muted-foreground">{t("status.loading")}</output>;
}
