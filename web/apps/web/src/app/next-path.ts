/**
 * safeNextPath is where to go after signing in, from the next parameter: a
 * path on this site, trimmed, or undefined for anything else (M1/P5 design
 * 3.5). Anyone can put a link with any next in front of a user, and every
 * other address would be an open redirect. What starts with one slash and
 * has no backslash and no control character can only be read as a path of
 * this site; each rule below closes a way out:
 */
export function safeNextPath(next: string | null): string | undefined {
  // Browsers drop tabs and newlines from an address: "/\t/evil" would be "//evil".
  if (next === null || [...next].some((c) => c.charCodeAt(0) <= 0x1f || c.charCodeAt(0) === 0x7f)) {
    return undefined;
  }
  const path = next.trim();
  // "//evil" is an address on another host; browsers read "/\evil" as "//evil".
  if (!path.startsWith("/") || path.startsWith("//") || path.includes("\\")) {
    return undefined;
  }
  return path;
}

/** signInPath is the sign-in page that comes back to path, its query and fragment included. */
export function signInPath(path: string): string {
  return path === "/" ? "/sign-in" : `/sign-in?next=${encodeURIComponent(path)}`;
}

/** keepNext is path with the next parameter of search, if it has one: the sign-in and sign-up pages link to each other with it. */
export function keepNext(path: string, search: URLSearchParams): string {
  const next = search.get("next");
  return next === null ? path : `${path}?next=${encodeURIComponent(next)}`;
}
