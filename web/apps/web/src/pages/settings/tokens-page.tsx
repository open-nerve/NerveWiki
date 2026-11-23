import { observer } from "mobx-react-lite";
import useSWR from "swr";

import { errorText } from "../../app/problem-messages";
import { SettingsCard, SettingsSection, useSettingsHeading } from "../../app/settings-page";
import { Loading } from "../../components/loading";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { useApiTokens } from "../../stores/context";
import { CreateTokenDialog } from "./create-token-dialog";
import { TokenRow } from "./token-row";

/**
 * TokensPage lists the account's personal access tokens, creates them and revokes them (M1/P6 design 3.6): what they
 * are, New token beside it (under it at a phone's width, the text taking the row as a card's row's does; review B's
 * M1), above their card, a row each (M13/P3/S1, decision 820); while the list loads or if it cannot be read, in the
 * card's place; none, a line of text in it.
 */
export const TokensPage = observer(function TokensPage() {
  const apiTokens = useApiTokens();
  const t = useT();
  const { error, mutate } = useSWR("api-tokens", () => apiTokens.load());
  // A revoked token's row leaves with its button: the focus comes to the page's h1, its name (M13/P3/S1, decision 815).
  const heading = useSettingsHeading();
  const revoked = () => heading.current?.focus();
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
    list = (
      <SettingsCard>
        <p className="px-4.5 py-3.5 text-sm text-muted-foreground">{t("tokens.empty")}</p>
      </SettingsCard>
    );
  } else {
    list = (
      <SettingsCard list>
        {tokens.map((token) => (
          <TokenRow key={token.id} token={token} revoked={revoked} />
        ))}
      </SettingsCard>
    );
  }

  return (
    <SettingsSection>
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 px-0.5">
        <p className="max-w-xl min-w-0 flex-1 basis-56 text-sm text-muted-foreground">{t("tokens.body")}</p>
        {tokens !== undefined && <CreateTokenDialog />}
      </div>
      {list}
    </SettingsSection>
  );
});
