import { observable } from "mobx";

import { TabChannel, type Port } from "../events/channel";
import { browserPageLifecycle, type EventDeps } from "../events/deps";
import { EventHub, type PageLifecycle } from "../events/hub";
import { leaseLeadership, webLockLeadership } from "../events/leadership";
import { Refresher } from "../events/refresher";
import { AccountService } from "../services/account.service";
import { ApiTokenService } from "../services/api-token.service";
import { AuthService } from "../services/auth.service";
import { EventService } from "../services/event.service";
import { InstanceService } from "../services/instance.service";
import { InvitationPreviewService, InvitationService } from "../services/invitation.service";
import { MemberService } from "../services/member.service";
import { NotebookMemberService } from "../services/notebook-member.service";
import { NotebookService, type Notebook } from "../services/notebook.service";
import { OwnerlessService } from "../services/ownerless.service";
import { EditLeaveService, PageService } from "../services/page.service";
import { WorkspaceService, type Workspace } from "../services/workspace.service";
import type { Session } from "../session/session";
import { AccountStore } from "./account.store";
import { ApiTokenStore } from "./api-token.store";
import { AuditStore } from "./audit.store";
import { AuthStore } from "./auth.store";
import { answerClosings, closeOtherTabs } from "./edit-closing";
import { EditSession } from "./edit-session";
import { InstanceStore } from "./instance.store";
import { InvitationPreviewStore, InvitationStore } from "./invitation.store";
import { MemberStore } from "./member.store";
import { NotebookMemberStore } from "./notebook-member.store";
import { NotebookStore } from "./notebook.store";
import { OwnerlessStore } from "./ownerless.store";
import { PageEditing } from "./page-editing";
import { PageTreeStore } from "./page-tree.store";
import type { PreferencesStore } from "./preferences.store";
import { WorkspaceStore } from "./workspace.store";

/**
 * AppStores are what the page keeps for as long as it lives, whoever is
 * signed in: the device's preferences, what the instance runs, the
 * session, and what the event stream needs of the browser (none in tests
 * that do not open it).
 */
export class AppStores {
  readonly instance: InstanceStore;

  constructor(
    readonly preferences: PreferencesStore,
    readonly session: Session,
    readonly events?: EventDeps
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
  /** The edits of this generation, from as they begin until they end or are not opened (M5/P4 design 3.4). */
  readonly edits = observable.set<PageEditing>([], { deep: false });
  private readonly members: MemberService | undefined;
  private readonly invitations: InvitationService | undefined;
  private readonly notebooks: NotebookService | undefined;
  private readonly notebookMembers: NotebookMemberService | undefined;
  private readonly ownerless: OwnerlessService | undefined;
  private readonly pages: PageService | undefined;
  private readonly hub: EventHub | undefined;
  private readonly eventDeps: EventDeps | undefined;
  private readonly page: PageLifecycle;
  /** Ends an edit session as the page is left, with this login's token while it is valid. */
  private readonly leave: ((id: string) => void) | undefined;
  /** This login and tab, and the channel between the login's tabs that a sign-out closes their edits over. */
  private readonly closing: Closing | undefined;
  /** How many sign-outs this generation asked the other tabs to close their edits for. */
  private closings = 0;
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
    this.auth = new AuthStore(new AuthService(app.session.public), app.session.tokens, loginId, () => this.endEdits());
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
    this.hub =
      client && app.events && loginId !== undefined
        ? eventHub(new EventService(client), app.events, loginId)
        : undefined;
    this.eventDeps = this.hub && app.events;
    this.page = app.events?.page ?? browserPageLifecycle();
    if (loginId !== undefined) {
      const leaving = new EditLeaveService(app.session.public);
      this.leave = (id) => {
        const token = app.session.tokens.currentAccessToken(loginId);
        if (token !== undefined) {
          leaving.endOnLeave(id, token);
        }
      };
      const deps = app.events;
      this.closing = deps && { loginId, tabId: deps.tabId, port: () => deps.channel("nwiki.edits") };
    }
  }

  /**
   * unsavedEdit tells whether an edit of the page pageId, of the notebook
   * notebookId, of the workspace workspaceId, each when given, is open with
   * changes not saved: its page, its notebook, or its workspace, stays
   * shown while it is gone (M5/P4 design 3.9).
   */
  unsavedEdit({
    pageId,
    notebookId,
    workspaceId,
  }: {
    pageId?: string;
    notebookId?: string;
    workspaceId?: string;
  }): boolean {
    return [...this.edits].some(
      (editing) =>
        editing.unsaved &&
        (pageId === undefined || editing.pageId === pageId) &&
        (notebookId === undefined || editing.notebookId === notebookId) &&
        (workspaceId === undefined || editing.workspaceId === workspaceId)
    );
  }

  /**
   * endEdits ends the edits of this login, as the tab signs out: this
   * tab's and its other tabs', whose saves the logout would end (M4–M5
   * Codex review R2), each once what it has unsaved is saved, or after 1.5
   * seconds of saving. It resolves once their ends are answered, or after
   * at most 2 seconds, so that a network down does not hold the sign-out.
   */
  endEdits(): Promise<void> {
    const tabs = this.closing;
    let others: Promise<void> | undefined;
    if (tabs !== undefined) {
      const port = tabs.port();
      this.closings += 1;
      others = closeOtherTabs(port, tabs, `${tabs.tabId}-${this.closings}`, signOutWait).finally(() => port.close());
    }
    return atMost(signOutWait, Promise.all([this.closeEdits(), others]));
  }

  /**
   * answerSignOuts closes this tab's edits as another tab of the login
   * signs out, while it has some; it returns the unsubscribe. Where the
   * page has no EventDeps it does nothing.
   */
  answerSignOuts(): () => void {
    const tabs = this.closing;
    if (tabs === undefined) {
      return () => undefined;
    }
    const port = tabs.port();
    const unsubscribe = answerClosings(
      port,
      tabs,
      () => this.edits.size > 0,
      () => atMost(signOutWait, this.closeEdits())
    );
    return () => {
      unsubscribe();
      port.close();
    };
  }

  /** closeEdits ends this tab's edits, each once what it has unsaved is saved, or after 1.5 seconds of saving. */
  private closeEdits(): Promise<unknown> {
    return Promise.all([...this.edits].map((editing) => editing.close(signOutSave)));
  }

  /**
   * events is the event stream of this generation's login, not started:
   * the one hub for as long as this generation lives (M5/P3 design 3.7);
   * undefined while the tab is signed out, or where the page has no
   * EventDeps.
   */
  events(): EventHub | undefined {
    return this.hub;
  }

  /**
   * newRefresher is a new merger of the re-reads that events ask for (M5/P3
   * design 3.8), over the page's visibility; its holder stops it. Undefined
   * where events is.
   */
  newRefresher(): Refresher | undefined {
    const deps = this.eventDeps;
    return (
      deps &&
      new Refresher(
        { visible: () => deps.page.visible(), onChange: (l) => deps.page.on("visibilitychange", l) },
        deps.now
      )
    );
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

  /**
   * editPage is a new edit of the page pageId of notebook, in its workspace,
   * which the page holds, not this generation (M4/P6 design 3.6); edits
   * has it from as it begins until it ends, or its session is not opened.
   * Its session follows the page, and this login's events where the tab
   * has a stream (M5/P4 design 3.4).
   */
  editPage(notebook: { id: string; workspace_id: string }, pageId: string): PageEditing | undefined {
    const { pages, leave, hub } = this;
    if (pages === undefined || leave === undefined) {
      return undefined;
    }
    const events = hub && ((listener: Parameters<EventHub["subscribe"]>[0]) => hub.subscribe(listener));
    const session = new EditSession({ service: pages, page: this.page, events, leave }, pageId);
    return new PageEditing(pages, session, notebook, this.edits);
  }
}

/** How long signing out waits for the tab's edits to end, and for their saves before the ends go out. */
const signOutWait = 2_000;
const signOutSave = 1_500;

/** A login's tab and the channel its tabs close their edits over as one signs out. */
type Closing = { loginId: string; tabId: string; port: () => Port };

/** atMost resolves once work does, or after ms. */
function atMost(ms: number, work: Promise<unknown>): Promise<void> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const waited = new Promise<void>((resolve) => (timer = setTimeout(resolve, ms)));
  return Promise.race([work, waited]).then(() => clearTimeout(timer));
}

/**
 * eventHub is the hub of login: the tabs of one login elect one holder of
 * its stream, by its Web Lock or its lease, and talk on one channel.
 */
function eventHub(service: EventService, deps: EventDeps, loginId: string): EventHub {
  const name = `nwiki.events.${loginId}`;
  return new EventHub({
    open: (signal) => service.open(signal),
    leadership: () =>
      deps.locks
        ? webLockLeadership(deps.locks, name)
        : leaseLeadership({ storage: deps.storage, onStorage: deps.onStorage, now: deps.now, tabId: deps.tabId }, name),
    channel: () => new TabChannel(deps.channel("nwiki.events"), loginId),
    page: deps.page,
    now: deps.now,
  });
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
