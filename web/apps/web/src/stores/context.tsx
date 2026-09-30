import { createContext, use, type ReactNode } from "react";

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
