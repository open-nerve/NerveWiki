import { createContext, use, type ReactNode } from "react";

import type { User } from "../services/account.service";
import type { Notebook } from "../services/notebook.service";
import type { Workspace } from "../services/workspace.service";
import type { AccountStore } from "./account.store";
import type { ApiTokenStore } from "./api-token.store";
import type { InvitationStore } from "./invitation.store";
import type { MemberStore } from "./member.store";
import type { NotebookMemberStore } from "./notebook-member.store";
import type { NotebookStore } from "./notebook.store";
import type { RootStore } from "./root.store";
import type { WorkspaceStore } from "./workspace.store";

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

/**
 * useWorkspaces is the signed-in account's workspaces: only for a signed-in
 * generation, such as the pages the SignedIn guard shows, or the invitation
 * page once signed in.
 */
export function useWorkspaces(): WorkspaceStore {
  const { workspaces } = useStore();
  if (workspaces === undefined) {
    throw new Error("useWorkspaces is used signed out");
  }
  return workspaces;
}

/** useMembers is the member list of workspace: only for the pages the SignedIn guard shows. */
export function useMembers(workspace: Workspace): MemberStore {
  const members = useStore().membersOf(workspace);
  if (members === undefined) {
    throw new Error("useMembers is used outside SignedIn");
  }
  return members;
}

/** useNotebooks is the notebooks of workspace that the account sees: only for the pages the SignedIn guard shows. */
export function useNotebooks(workspace: Workspace): NotebookStore {
  const notebooks = useStore().notebooksOf(workspace);
  if (notebooks === undefined) {
    throw new Error("useNotebooks is used outside SignedIn");
  }
  return notebooks;
}

/** useNotebookMembers is the members of notebook: only for the pages the SignedIn guard shows. */
export function useNotebookMembers(notebook: Notebook): NotebookMemberStore {
  const members = useStore().notebookMembersOf(notebook);
  if (members === undefined) {
    throw new Error("useNotebookMembers is used outside SignedIn");
  }
  return members;
}

/** useInvitations is the pending invitations of workspace: only for the pages the SignedIn guard shows. */
export function useInvitations(workspace: Workspace): InvitationStore {
  const invitations = useStore().invitationsOf(workspace);
  if (invitations === undefined) {
    throw new Error("useInvitations is used outside SignedIn");
  }
  return invitations;
}
