import { createContext, use, type ReactNode } from "react";

import type { User } from "../services/account.service";
import type { AccountStore } from "./account.store";
import type { ApiTokenStore } from "./api-token.store";
import type { RootStore } from "./root.store";

const StoreContext = createContext<RootStore | null>(null);

export function StoreProvider({ store, children }: { store: RootStore; children: ReactNode }) {
  return <StoreContext value={store}>{children}</StoreContext>;
}

/** useStore returns the RootStore of the StoreProvider above. */
export function useStore(): RootStore {
  const store = use(StoreContext);
  if (!store) {
    throw new Error("useStore is used outside a StoreProvider");
  }
  return store;
}

/** useAccount is the signed-in account, loaded: only for the pages the SignedIn guard shows. */
export function useAccount(): { account: AccountStore; me: User } {
  const { account } = useStore();
  const me = account?.me;
  if (account === undefined || me === undefined) {
    throw new Error("useAccount is used outside SignedIn");
  }
  return { account, me };
}

/** useApiTokens is the signed-in account's personal access tokens: only for the pages the SignedIn guard shows. */
export function useApiTokens(): ApiTokenStore {
  const { apiTokens } = useStore();
  if (apiTokens === undefined) {
    throw new Error("useApiTokens is used outside SignedIn");
  }
  return apiTokens;
}
