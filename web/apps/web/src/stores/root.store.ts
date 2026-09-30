import { AccountService } from "../services/account.service";
import { AuthService } from "../services/auth.service";
import { InstanceService } from "../services/instance.service";
import type { Session } from "../session/session";
import { AccountStore } from "./account.store";
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
 * anew whenever the tab's session changes (M1/P5 design 3.3), and the only
 * place where services and stores are wired: each service gets its client
 * from here. Components reach it through useStore. The page's AppStores
 * are shared by every generation.
 */
export class RootStore {
  readonly preferences: PreferencesStore;
  readonly instance: InstanceStore;
  readonly auth: AuthStore;
  /** The signed-in account's store; undefined while the tab is signed out. */
  readonly account: AccountStore | undefined;

  constructor(
    app: AppStores,
    readonly loginId: string | undefined
  ) {
    this.preferences = app.preferences;
    this.instance = app.instance;
    this.auth = new AuthStore(new AuthService(app.session.public), app.session.tokens);
    this.account =
      loginId === undefined ? undefined : new AccountStore(new AccountService(app.session.clientFor(loginId)));
  }
}
