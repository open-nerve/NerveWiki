/**
 * twinsOf is the ids of the items of list that another of list has the
 * same key as (keyOf): nothing else they show tells them apart, so their
 * rows and controls name them by more (v0.1 design 13.2, item 17): the
 * ownerless notebooks of one name and former owner, the trash's items of
 * one name and place, and those deleted in the same minute too.
 */
export function twinsOf<T extends { id: string }>(list: readonly T[], keyOf: (item: T) => string): Set<string> {
  const counts = new Map<string, number>();
  for (const item of list) {
    const key = keyOf(item);
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return new Set(list.filter((item) => (counts.get(keyOf(item)) ?? 0) > 1).map((item) => item.id));
}

/** shortId is the last six digits of id, a UUID, which tell twins apart where nothing else does ("ID 0000e1"). */
export function shortId(id: string): string {
  return id.replaceAll("-", "").slice(-6);
}
