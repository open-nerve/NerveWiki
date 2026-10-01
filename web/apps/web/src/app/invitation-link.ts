import type { InvitationLink } from "../services/invitation.service";

/**
 * invitationLink is the address of the invitation page of link at origin
 * (M2 design 4): the token is in the fragment alone, which browsers send
 * to no server, nor in a Referer.
 */
export function invitationLink(origin: string, { id, token }: InvitationLink): string {
  return `${origin}/invitations/${encodeURIComponent(id)}#${encodeURIComponent(token)}`;
}

/**
 * linkOf is the link of the invitation page of id, whose fragment is hash
 * ("#" and the token); undefined when the fragment holds none, as when the
 * link was cut short.
 */
export function linkOf(id: string, hash: string): InvitationLink | undefined {
  let token;
  try {
    token = decodeURIComponent(hash.slice(1));
  } catch {
    return undefined; // a fragment mangled on its way: no token
  }
  return token === "" ? undefined : { id, token };
}
