import type { PreferencesStore } from "./preferences.store";

/**
 * RootStore is where the app's stores and services are wired, and the only
 * place. Components reach it through useStore.
 */
export class RootStore {
  constructor(readonly preferences: PreferencesStore) {}
}
