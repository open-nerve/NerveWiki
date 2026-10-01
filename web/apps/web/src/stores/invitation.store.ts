import { makeAutoObservable, runInAction } from "mobx";

import { ApiError } from "../services/api";
import type {
  InvitationLink,
  InvitationPreview,
  InvitationPreviewService,
  InvitationService,
  WorkspaceInvitation,
  WorkspaceInvitationCreate,
} from "../services/invitation.service";

/**
 * InvitationStore holds the pending invitations of one workspace, newest
 * first, for one generation: an admin's (M2/P6 design 3.2). Their tokens
 * are no secret shown once: the server makes them again on every list
 * (v0.1 design 13.2, rule 13).
 */
export class InvitationStore {
  list: WorkspaceInvitation[] | undefined = undefined;
  /** How many changes have been answered: a read that overlaps one may have read the list before it. */
  private changesAnswered = 0;

  constructor(
    private readonly service: Pick<InvitationService, "list" | "create" | "remove">,
    /** The slug of the workspace whose invitations these are. */
    private readonly slug: string
  ) {
    makeAutoObservable<this, "service" | "slug" | "changesAnswered">(this, {
      service: false,
      slug: false,
      changesAnswered: false,
    });
  }

  /**
   * load reads the list; SWR calls it. A change answered while the read was
   * out is newer than what it read: the list kept stays the one with it.
   */
  async load(): Promise<WorkspaceInvitation[]> {
    const answeredBefore = this.changesAnswered;
    const list = await this.service.list(this.slug);
    if (this.changesAnswered !== answeredBefore && this.list !== undefined) {
      return this.list;
    }
    runInAction(() => {
      this.list = list;
    });
    return list;
  }

  async invite(body: WorkspaceInvitationCreate): Promise<WorkspaceInvitation> {
    const created = await this.service.create(this.slug, body);
    // A read answered before the creation may hold it already.
    this.changed((list) => [created, ...list.filter((invitation) => invitation.id !== created.id)]);
    return created;
  }

  /** withdraw deletes the invitation id; one gone already (accepted, or withdrawn elsewhere) is gone as well. */
  async withdraw(id: string): Promise<void> {
    try {
      await this.service.remove(id);
    } catch (error) {
      if (!(error instanceof ApiError && error.code === "workspace.invitation_not_found")) {
        throw error;
      }
    }
    this.changed((list) => list.filter((invitation) => invitation.id !== id));
  }

  private changed(change: (list: WorkspaceInvitation[]) => WorkspaceInvitation[]): void {
    runInAction(() => {
      this.changesAnswered += 1;
      if (this.list !== undefined) {
        this.list = change(this.list);
      }
    });
  }
}

/**
 * InvitationPreviewStore shows what an invitation's link invites to, to a
 * tab signed out too (M2/P6 design 3.2): it holds nothing, the page's SWR
 * keeps the answer.
 */
export class InvitationPreviewStore {
  constructor(private readonly service: Pick<InvitationPreviewService, "preview">) {}

  preview(link: InvitationLink): Promise<InvitationPreview> {
    return this.service.preview(link);
  }
}
