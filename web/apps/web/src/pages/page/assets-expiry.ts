import { useEffect, useMemo, useRef } from "react";

/** How long before the first of its addresses expires what holds them is read again. */
const expiryMargin = 60_000;
/** How soon at the earliest: a clock far from the server's would read it again and again. */
const expiryFloor = 30_000;
/** How long an address is good for at the least, from as it is read: the server signs it for an hour or more. */
const signedFor = 60 * 60_000;

/**
 * rereadIn is how long after read what holds attachments' addresses, the
 * first of them expiring at expires, is read again: expiryMargin before
 * that, expiryFloor at the soonest, and before an hour from the read is
 * over at the latest, which an address is good for whatever the clocks
 * say.
 */
export function rereadIn(expires: number, read: number): number {
  return Math.min(Math.max(expires - read - expiryMargin, expiryFloor), signedFor - expiryMargin);
}

/** An answer that holds attachments' addresses: when the first of them expires, null for none. */
type Expiring = { assets_expire_at: string | null };

/** readAt is when each answer was read: as it came (stamped), or else as it was first seen. */
const readAt = new WeakMap<Expiring, number>();

/** readAgain are the answers read again as they were due: once each, whichever of their readers' timers is first. */
const readAgain = new WeakSet<Expiring>();

/** stamped is the answer read answers, with when it came: its addresses expire from then, whether shown or not. */
export async function stamped<T extends Expiring>(read: Promise<T>): Promise<T> {
  const answer = await read;
  readAt.set(answer, Date.now());
  return answer;
}

/**
 * eachRead is SWR's options for an answer that holds attachments'
 * addresses: each read is its own answer, read again on its own time,
 * though it says what the one before it said (the server signs the same
 * addresses for an hour).
 */
export const eachRead = { compare: Object.is };

/** dueOf is when answer is to be read again, from when it was read; undefined, holding no address. */
function dueOf(answer: Expiring): number | undefined {
  const expires = answer.assets_expire_at === null ? Number.NaN : Date.parse(answer.assets_expire_at);
  if (!Number.isFinite(expires)) {
    return undefined;
  }
  let read = readAt.get(answer);
  if (read === undefined) {
    read = Date.now();
    readAt.set(answer, read);
  }
  return read + rereadIn(expires, read);
}

/**
 * useAssetsExpiry is data, an answer that holds attachments' addresses,
 * read (stamped, eachRead) with SWR: read again (reread) as it is due,
 * rereadIn after it was read, once, by the first of the hooks that have
 * it; or nothing, when it had expired as the hook had it (M7/P4 design
 * 4.5): the cache's, left since, which SWR reads again as the hook mounts.
 * One shown stays as it expires: SWR reads it again as the tab is shown
 * or online again, and its images that fail to load meanwhile read it
 * again (reading/assets.ts). A hidden tab's is not read as it is due, but
 * as the tab is shown (SWR). A clock far ahead of the server's has it
 * read again each expiryFloor, as the attachments' lists are.
 */
export function useAssetsExpiry<T extends Expiring>(data: T | undefined, reread: () => void): T | undefined {
  const latest = useRef(reread);
  useEffect(() => {
    latest.current = reread;
  });
  const expired = useMemo(() => {
    const due = data === undefined ? undefined : dueOf(data);
    // oxlint-disable-next-line react/purity -- once for each answer: whether it had expired as it came, not as the hook renders again
    return due !== undefined && Date.now() >= due + expiryMargin;
  }, [data]);
  useEffect(() => {
    const due = data === undefined ? undefined : dueOf(data);
    if (data === undefined || due === undefined || expired) {
      return undefined;
    }
    const timer = setTimeout(
      () => {
        if (!document.hidden && !readAgain.has(data)) {
          readAgain.add(data);
          latest.current();
        }
      },
      Math.max(due - Date.now(), 0)
    );
    return () => clearTimeout(timer);
  }, [data, expired]);
  return expired ? undefined : data;
}
