import { observer } from "mobx-react-lite";
import type { ReactNode } from "react";
import { SWRConfig, type SWRConfiguration } from "swr";

import { I18nProvider } from "../i18n/i18n";
import { StoreProvider } from "../stores/context";
import type { RootStore } from "../stores/root.store";
import { DocumentSync } from "./document-sync";
import { EventStream } from "./event-stream";
import { onErrorRetry } from "./retry";
import { SignOutElsewhere } from "./sign-out-elsewhere";

// SWR drives the loading of the stores. SWR makes its cache when SWRConfig
// mounts and keeps it for as long as it stays mounted: each generation of
// RootStore (one per login) mounts AppProviders anew, with its loginId as
// the key, so that it starts with nothing cached (M1/P5 design 3.3).
const swr: SWRConfiguration = {
  provider: () => new Map(),
  onErrorRetry,
};

export const AppProviders = observer(function AppProviders({
  store,
  children,
}: {
  store: RootStore;
  children: ReactNode;
}) {
  return (
    <StoreProvider store={store}>
      <SWRConfig value={swr}>
        <I18nProvider locale={store.preferences.locale}>
          <DocumentSync />
          <EventStream />
          <SignOutElsewhere />
          {children}
        </I18nProvider>
      </SWRConfig>
    </StoreProvider>
  );
});
