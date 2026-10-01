import { AccountService } from "../services/account.service";
import { ApiTokenService } from "../services/api-token.service";
import { AuthService } from "../services/auth.service";
import { InstanceService } from "../services/instance.service";
import { MemberService } from "../services/member.service";
import { WorkspaceService, type Workspace } from "../services/workspace.service";
import type { Session } from "../session/session";
import { AccountStore } from "./account.store";
import { ApiTokenStore } from "./api-token.store";
import { AuthStore } from "./auth.store";
import { InstanceStore } from "./instance.store";
import { MemberStore } from "./member.store";
import type { PreferencesStore } from "./preferences.store";
import { WorkspaceStore } from "./workspace.store";

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
  /** The signed-in account's workspaces; undefined while the tab is signed out. */
  readonly workspaces: WorkspaceStore | undefined;
  private readonly members: MemberService | undefined;
  /** The member lists this generation holds, by workspace id. */
  private readonly memberLists = new Map<string, MemberStore>();

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
    this.workspaces = client && new WorkspaceStore(new WorkspaceService(client));
    this.members = client && new MemberService(client);
  }

  /**
   * membersOf is the member list of workspace, the same one for as long as
   * this generation lives (M2/P6 design 3.2); undefined while the tab is
   * signed out. It goes by the workspace's id: a workspace deleted and
   * another made with its slug have lists of their own.
   */
  membersOf(workspace: Workspace): MemberStore | undefined {
    if (this.members === undefined) {
      return undefined;
    }
    let list = this.memberLists.get(workspace.id);
    if (list === undefined) {
      list = new MemberStore(this.members, workspace.slug);
      this.memberLists.set(workspace.id, list);
    }
    return list;
  }
}
