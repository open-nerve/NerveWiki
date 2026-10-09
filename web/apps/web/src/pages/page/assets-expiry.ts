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

/** firstSeen is when each answer was first seen: as it was read, near enough. */
const firstSeen = new WeakMap<Expiring, number>();

/** readAgain are the answers read again as they were due: once each, whichever of their readers' timers is first. */
const readAgain = new WeakSet<Expiring>();

/** dueOf is when answer is to be read again, from when it was first seen; undefined, holding no address. */
function dueOf(answer: Expiring): number | undefined {
  if (answer.assets_expire_at === null) {
    return undefined;
  }
  const expires = Date.parse(answer.assets_expire_at);
  let seen = firstSeen.get(answer);
  if (seen === undefined) {
    seen = Date.now();
    firstSeen.set(answer, seen);
  }
  return seen + rereadIn(expires, seen);
}

/**
 * useAssetsExpiry is data, an answer that holds attachments' addresses,
 * read again (reread) as it is due (rereadIn after it was first seen),
 * once, by the first of the hooks that have it; or nothing, when it had
 * expired as the hook had it (M7/P4 design 4.5): the cache's, left since,
 * read again at once. One shown stays as it expires: a tab asleep
 * reads it again as it wakes, its timers late, and its images that fail
 * to load meanwhile read it again (reading/assets.ts). An answer read
 * again the same is kept, not read again (SWR keeps the one it had): a
 * clock far ahead of the server's reads each at most once.
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
    if (data === undefined || due === undefined) {
      return undefined;
    }
    // Past it, at once: SWR's own read of what it had in its cache is the same read.
    const timer = setTimeout(
      () => {
        if (!readAgain.has(data)) {
          readAgain.add(data);
          latest.current();
        }
      },
      Math.max(due - Date.now(), 0)
    );
    return () => clearTimeout(timer);
  }, [data]);
  return expired ? undefined : data;
}
