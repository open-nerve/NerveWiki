import { observer } from "mobx-react-lite";
import useSWR from "swr";

import { errorText } from "../../app/problem-messages";
import { Loading } from "../../components/loading";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { useApiTokens } from "../../stores/context";
import { CreateTokenDialog } from "./create-token-dialog";
import { TokenRow } from "./token-row";

/** TokensPage lists the account's personal access tokens, creates them and revokes them (M1/P6 design 3.6). */
export const TokensPage = observer(function TokensPage() {
  const apiTokens = useApiTokens();
  const t = useT();
  const { error, mutate } = useSWR("api-tokens", () => apiTokens.load());
  const tokens = apiTokens.tokens;
  const failed = error === undefined ? undefined : errorText(error, t);

  let list;
  if (tokens === undefined) {
    list =
      failed === undefined ? (
        <Loading />
      ) : (
        <div className="space-y-3">
          <Alert>{failed}</Alert>
          <Button variant="outline" onClick={() => void mutate()}>
            {t("tokens.retry")}
          </Button>
        </div>
      );
  } else if (tokens.length === 0) {
    list = <p className="text-muted-foreground">{t("tokens.empty")}</p>;
  } else {
    list = (
      <ul className="divide-y rounded-md border">
        {tokens.map((token) => (
          <TokenRow key={token.id} token={token} />
        ))}
      </ul>
    );
  }

  return (
    <section className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="max-w-xl space-y-1">
          <h2 className="text-lg font-semibold">{t("settings.tokens")}</h2>
          <p className="text-sm text-muted-foreground">{t("tokens.body")}</p>
        </div>
        {tokens !== undefined && <CreateTokenDialog />}
      </div>
      {list}
    </section>
  );
});
