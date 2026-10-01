/**
 * byName is the order of the server's lists of workspaces and notebooks:
 * lower(name), name, id. The database collates by code point (C.UTF-8,
 * v0.1 design 7.1), this by UTF-16 code unit, which orders the same but for
 * characters beyond the BMP (an emoji against a full-width letter); its
 * lower() maps one character to one, toLowerCase a few to two. Where that
 * changes the order, the next load puts the list back in the server's.
 */
export function byName(a: { name: string; id: string }, b: { name: string; id: string }): number {
  const [x, y] = [a.name.toLowerCase(), b.name.toLowerCase()];
  return compare(x, y) || compare(a.name, b.name) || compare(a.id, b.id);
}

export function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}
