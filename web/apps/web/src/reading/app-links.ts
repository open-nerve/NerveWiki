import type { Enhancement } from "./enhancement";

/** The paths the server answers itself, which are no page of the app: its API, its probes, the build's files. */
const serverPaths = /^\/(?:api|healthz|readyz|assets)(?:\/|$)/;

const xlink = "http://www.w3.org/1999/xlink";

/**
 * appLinks has the view's links that lead into the app go there through
 * the router (M6/P3 design 6.7, M6/P6 design 8, 9). The server writes the
 * page a link leads to (data-nw-node) and the id of its anchor's heading
 * (data-nw-anchor), a property's link too, and a tag's name (data-nw-tag),
 * not their addresses, which are the app's: each gets its address here. A
 * full address of this site that the content writes goes through the
 * router as well, when it is a page of the app (inApp). A click with a
 * modifier or another button is the browser's, the address being real; a
 * link to no page (nw-unresolved) has none. The click is the container's:
 * a link added later, a diagram's, goes the same way, by its address
 * alone: what a diagram writes is mermaid's, not the server's, its marks
 * left be.
 */
export const appLinks: Enhancement = (container, { workspace, notebook, navigate }) => {
  const notebookPath = `/${workspace}/notebooks/${notebook}`;
  const given = [...container.querySelectorAll<HTMLAnchorElement>("a[data-nw-node], a[data-nw-tag]")].filter(
    (link) => link.closest(".nw-diagram") === null
  );
  for (const link of given) {
    link.setAttribute("href", addressOf(link, notebookPath));
  }
  const onClick = (event: MouseEvent) => {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return;
    }
    const link = event.target instanceof Element ? event.target.closest<Element>("a") : null;
    if (link === null || !container.contains(link)) {
      return;
    }
    const to =
      link instanceof HTMLAnchorElement && given.includes(link)
        ? link.getAttribute("href")
        : inApp(link.getAttribute("href") ?? link.getAttributeNS(xlink, "href"), window.location.origin);
    if (to === null || to === undefined) {
      return;
    }
    event.preventDefault();
    navigate(to);
  };
  container.addEventListener("click", onClick);
  return () => {
    container.removeEventListener("click", onClick);
    for (const link of given) {
      link.removeAttribute("href");
    }
  };
};

/** addressOf is the app's address of link, to a page (with its anchor) or to a tag's pages, in the notebook at notebookPath. */
function addressOf(link: HTMLAnchorElement, notebookPath: string): string {
  const { nwNode: node, nwAnchor: anchor, nwTag: tag } = link.dataset;
  if (node === undefined) {
    // The tag's name is one segment, its '/' too: a nested tag (a/b), and one the page writes ending with '/' (#a//).
    return `${notebookPath}/tags/${encodeURIComponent(tag ?? "")}`;
  }
  const page = `${notebookPath}/pages/${encodeURIComponent(node)}`;
  return anchor === undefined ? page : `${page}#${anchor}`;
}

/**
 * inApp is the address in the app that href names, when href is a full
 * address of this site, of origin, and a page of the app (M6/P6 design 9):
 * not one the server answers itself (its API, its probes, the build's
 * files), its segments decoded as the server decodes them, nor a file
 * (its last segment has a dot). It is undefined for any other: of another
 * site, the browser's to follow; an anchor of this page (#…), which the
 * server writes as it is; and a path starting with "//", which the router
 * would take for another site's address.
 */
export function inApp(href: string | null, origin: string): string | undefined {
  if (href === null || !/^https?:/i.test(href)) {
    return undefined;
  }
  let url: URL;
  try {
    url = new URL(href);
  } catch {
    return undefined;
  }
  const path = decodedPath(url.pathname);
  const last = path.slice(path.lastIndexOf("/") + 1);
  if (url.origin !== origin || url.pathname.startsWith("//") || serverPaths.test(path) || last.includes(".")) {
    return undefined;
  }
  return url.pathname + url.search + url.hash;
}

/** decodedPath is path with each segment's escapes decoded, a segment that does not decode as it is. */
function decodedPath(path: string): string {
  return path
    .split("/")
    .map((segment) => {
      try {
        return decodeURIComponent(segment);
      } catch {
        return segment;
      }
    })
    .join("/");
}
