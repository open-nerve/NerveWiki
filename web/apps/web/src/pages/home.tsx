import { observer } from "mobx-react-lite";
import useSWR from "swr";

import { useT } from "../i18n/i18n";
import { useStore } from "../stores/context";

/** HomePage shows what this instance runs. */
export const HomePage = observer(function HomePage() {
  const { instance } = useStore();
  const t = useT();
  const { error } = useSWR("instance", () => instance.fetch());
  const info = instance.info;

  if (!info) {
    return error ? (
      <p role="alert" className="text-destructive">
        {t("home.loadFailed")}
      </p>
    ) : (
      <p className="text-muted-foreground">{t("home.loading")}</p>
    );
  }
  return (
    <section className="space-y-2">
      <h1 className="text-2xl font-semibold">{info.product}</h1>
      <p className="text-muted-foreground">{t("home.version", { version: info.version, commit: info.commit })}</p>
      <p className="text-muted-foreground">{t("home.apiVersion", { apiVersion: info.api_version })}</p>
    </section>
  );
});
