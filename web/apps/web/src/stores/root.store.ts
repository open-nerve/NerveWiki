import type { ApiClient } from "@nervewiki/api-client";

import { InstanceService } from "../services/instance.service";
import { InstanceStore } from "./instance.store";
import type { PreferencesStore } from "./preferences.store";

/**
 * RootStore is where the app's services and stores are wired, and the only
 * place: each service gets the API client from here, never from a module of
 * its own. Components reach it through useStore. The preferences are the
 * device's and are passed in: from M1 on, a RootStore is made per login and
 * they outlive it.
 */
export class RootStore {
  readonly instance: InstanceStore;

  constructor(
    readonly preferences: PreferencesStore,
    api: ApiClient
  ) {
    this.instance = new InstanceStore(new InstanceService(api));
  }
}
