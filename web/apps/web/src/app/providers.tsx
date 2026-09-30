import type { ReactNode } from "react";
import { SWRConfig, type SWRConfiguration } from "swr";

import { I18nProvider } from "../i18n/i18n";
import { ApiError } from "../services/api";
import { StoreProvider } from "../stores/context";
import type { RootStore } from "../stores/root.store";

// SWR drives the loading of the stores. Each RootStore gets its own cache, so
// a new one (from M1 on, per login) starts with nothing cached. A request the
// API refused (4xx) is not retried: it would be refused again.
const swr: SWRConfiguration = {
  provider: () => new Map(),
  shouldRetryOnError: (error: unknown) => !(error instanceof ApiError && error.status < 500),
};

export function AppProviders({ store, children }: { store: RootStore; children: ReactNode }) {
  return (
    <StoreProvider store={store}>
      <SWRConfig value={swr}>
        <I18nProvider>{children}</I18nProvider>
      </SWRConfig>
    </StoreProvider>
  );
}
