/** InvitationLink is what the link of an invitation carries: its id, and its token. */
export type InvitationLink = { id: string; token: string };

/**
 * invitationLink is the address of the invitation page of link at origin
 * (M2 design 4): the token is in the fragment alone, which browsers send
 * to no server, nor in a Referer.
 */
export function invitationLink(origin: string, { id, token }: InvitationLink): string {
  return `${origin}/invitations/${encodeURIComponent(id)}#${encodeURIComponent(token)}`;
}
