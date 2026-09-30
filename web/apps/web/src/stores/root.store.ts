import { AccountService } from "../services/account.service";
import { ApiTokenService } from "../services/api-token.service";
import { AuthService } from "../services/auth.service";
import { InstanceService } from "../services/instance.service";
import type { Session } from "../session/session";
import { AccountStore } from "./account.store";
import { ApiTokenStore } from "./api-token.store";
import { AuthStore } from "./auth.store";
import { InstanceStore } from "./instance.store";
import type { PreferencesStore } from "./preferences.store";

/**
 * AppStores are what the page keeps for as long as it lives, whoever is
 * signed in: the device's preferences, what the instance runs, and the
 * session.
 */
export class AppStores {
  readonly instance: InstanceStore;

  constructor(
    readonly preferences: PreferencesStore,
    readonly session: Session
  ) {
    this.instance = new InstanceStore(new InstanceService(session.public));
  }
}

/**
 * RootStore is one generation of the app's stores: one per login, made
 * anew whenever the tab's session changes (M1/P5 design 3.3). With the
 * page's AppStores, shared by every generation, it is the only place where
 * services and stores are wired: each service gets its client from here.
 * Components reach it through useStore.
 */
export class RootStore {
  readonly preferences: PreferencesStore;
  readonly instance: InstanceStore;
  readonly auth: AuthStore;
  /** The signed-in account's store; undefined while the tab is signed out. */
  readonly account: AccountStore | undefined;
  /** The signed-in account's personal access tokens; undefined while the tab is signed out. */
  readonly apiTokens: ApiTokenStore | undefined;

  constructor(
    app: AppStores,
    readonly loginId: string | undefined
  ) {
    this.preferences = app.preferences;
    this.instance = app.instance;
    this.auth = new AuthStore(new AuthService(app.session.public), app.session.tokens, loginId);
    const client = loginId === undefined ? undefined : app.session.clientFor(loginId);
    this.account = client && new AccountStore(new AccountService(client));
    this.apiTokens = client && new ApiTokenStore(new ApiTokenService(client));
  }
}
