import { createContext, use, type ReactNode } from "react";

import type { User } from "../services/account.service";
import type { Workspace } from "../services/workspace.service";
import type { AccountStore } from "./account.store";
import type { ApiTokenStore } from "./api-token.store";
import type { InvitationStore } from "./invitation.store";
import type { MemberStore } from "./member.store";
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

/** useWorkspaces is the signed-in account's workspaces: only for the pages the SignedIn guard shows. */
export function useWorkspaces(): WorkspaceStore {
  const { workspaces } = useStore();
  if (workspaces === undefined) {
    throw new Error("useWorkspaces is used outside SignedIn");
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

/** useInvitations is the pending invitations of workspace: only for the pages the SignedIn guard shows. */
export function useInvitations(workspace: Workspace): InvitationStore {
  const invitations = useStore().invitationsOf(workspace);
  if (invitations === undefined) {
    throw new Error("useInvitations is used outside SignedIn");
  }
  return invitations;
}
