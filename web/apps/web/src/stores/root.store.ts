import { AccountService } from "../services/account.service";
import { ApiTokenService } from "../services/api-token.service";
import { AuthService } from "../services/auth.service";
import { InstanceService } from "../services/instance.service";
import { InvitationPreviewService, InvitationService } from "../services/invitation.service";
import { MemberService } from "../services/member.service";
import { NotebookMemberService } from "../services/notebook-member.service";
import { NotebookService, type Notebook } from "../services/notebook.service";
import { OwnerlessService } from "../services/ownerless.service";
import { PageService } from "../services/page.service";
import { WorkspaceService, type Workspace } from "../services/workspace.service";
import type { Session } from "../session/session";
import { AccountStore } from "./account.store";
import { ApiTokenStore } from "./api-token.store";
import { AuditStore } from "./audit.store";
import { AuthStore } from "./auth.store";
import { InstanceStore } from "./instance.store";
import { InvitationPreviewStore, InvitationStore } from "./invitation.store";
import { MemberStore } from "./member.store";
import { NotebookMemberStore } from "./notebook-member.store";
import { NotebookStore } from "./notebook.store";
import { OwnerlessStore } from "./ownerless.store";
import { PageTreeStore } from "./page-tree.store";
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
  /** What invitations' links invite to: signed out too. */
  readonly invitationPreviews: InvitationPreviewStore;
  /** The signed-in account's store; undefined while the tab is signed out. */
  readonly account: AccountStore | undefined;
  /** The signed-in account's personal access tokens; undefined while the tab is signed out. */
  readonly apiTokens: ApiTokenStore | undefined;
  /** The signed-in account's workspaces; undefined while the tab is signed out. */
  readonly workspaces: WorkspaceStore | undefined;
  private readonly members: MemberService | undefined;
  private readonly invitations: InvitationService | undefined;
  private readonly notebooks: NotebookService | undefined;
  private readonly notebookMembers: NotebookMemberService | undefined;
  private readonly ownerless: OwnerlessService | undefined;
  private readonly pages: PageService | undefined;
  /** The member, invitation and notebook lists this generation holds, by workspace id. */
  private readonly memberLists = new Map<string, MemberStore>();
  private readonly invitationLists = new Map<string, InvitationStore>();
  private readonly notebookLists = new Map<string, NotebookStore>();
  /** The ownerless notebooks and the audit events this generation holds, by workspace id. */
  private readonly ownerlessLists = new Map<string, OwnerlessStore>();
  private readonly auditLists = new Map<string, AuditStore>();
  /** The notebook member lists and page trees this generation holds, by notebook id. */
  private readonly notebookMemberLists = new Map<string, NotebookMemberStore>();
  private readonly pageTrees = new Map<string, PageTreeStore>();

  constructor(
    app: AppStores,
    readonly loginId: string | undefined
  ) {
    this.preferences = app.preferences;
    this.instance = app.instance;
    this.auth = new AuthStore(new AuthService(app.session.public), app.session.tokens, loginId);
    this.invitationPreviews = new InvitationPreviewStore(new InvitationPreviewService(app.session.public));
    const client = loginId === undefined ? undefined : app.session.clientFor(loginId);
    this.account = client && new AccountStore(new AccountService(client));
    this.apiTokens = client && new ApiTokenStore(new ApiTokenService(client));
    this.workspaces = client && new WorkspaceStore(new WorkspaceService(client));
    this.members = client && new MemberService(client);
    this.invitations = client && new InvitationService(client);
    this.notebooks = client && new NotebookService(client);
    this.notebookMembers = client && new NotebookMemberService(client);
    this.ownerless = client && new OwnerlessService(client);
    this.pages = client && new PageService(client);
  }

  /**
   * membersOf is the member list of workspace, the same one for as long as
   * this generation lives (M2/P6 design 3.2); undefined while the tab is
   * signed out. It goes by the workspace's id: a workspace deleted and
   * another made with its slug have lists of their own.
   */
  membersOf(workspace: Workspace): MemberStore | undefined {
    const service = this.members;
    return service && once(this.memberLists, workspace.id, () => new MemberStore(service, workspace.slug));
  }

  /** invitationsOf is the pending invitations of workspace, as membersOf is its members. */
  invitationsOf(workspace: Workspace): InvitationStore | undefined {
    const service = this.invitations;
    return service && once(this.invitationLists, workspace.id, () => new InvitationStore(service, workspace.slug));
  }

  /**
   * notebooksOf is the notebooks of workspace that the account sees, as
   * membersOf is its members (M3/P4 design 3.2).
   */
  notebooksOf(workspace: Workspace): NotebookStore | undefined {
    const service = this.notebooks;
    return service && once(this.notebookLists, workspace.id, () => new NotebookStore(service, workspace.slug));
  }

  /** ownerlessOf is the ownerless notebooks of workspace, as its admins see them (M3/P5 design 3.2). */
  ownerlessOf(workspace: Workspace): OwnerlessStore | undefined {
    const service = this.ownerless;
    return service && once(this.ownerlessLists, workspace.id, () => new OwnerlessStore(service, workspace.slug));
  }

  /** auditOf is the audit events of workspace's ownerless notebooks, as ownerlessOf is the notebooks. */
  auditOf(workspace: Workspace): AuditStore | undefined {
    const service = this.ownerless;
    return service && once(this.auditLists, workspace.id, () => new AuditStore(service, workspace.slug));
  }

  /** notebookMembersOf is the members of notebook, the same list for as long as this generation lives. */
  notebookMembersOf(notebook: Notebook): NotebookMemberStore | undefined {
    const service = this.notebookMembers;
    return service && once(this.notebookMemberLists, notebook.id, () => new NotebookMemberStore(service, notebook.id));
  }

  /** pagesOf is the page tree of notebook, the same one for as long as this generation lives (M4/P5 design 3.4). */
  pagesOf(notebook: Notebook): PageTreeStore | undefined {
    const service = this.pages;
    return service && once(this.pageTrees, notebook.id, () => new PageTreeStore(service, notebook.id));
  }
}

/** once is the value of key in cache, which make makes the first time. */
function once<T>(cache: Map<string, T>, key: string, make: () => T): T {
  let value = cache.get(key);
  if (value === undefined) {
    value = make();
    cache.set(key, value);
  }
  return value;
}
