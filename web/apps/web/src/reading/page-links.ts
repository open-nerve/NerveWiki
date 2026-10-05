import type { Enhancement } from "./enhancement";

/**
 * pageLinks has the links to the notebook's pages lead there (M6/P3 design
 * 6.7). The server writes the page a link leads to (data-nw-node) and the
 * id of its anchor's heading (data-nw-anchor), not its address, which is
 * the app's: each gets its address here, so that a click with a modifier
 * or the middle button opens it as the browser does, and a plain click
 * goes through the router. A link to no page (nw-unresolved) has no
 * address.
 */
export const pageLinks: Enhancement = (container, { workspace, notebook, navigate }) => {
  const links = [...container.querySelectorAll<HTMLAnchorElement>("a[data-nw-node]")];
  if (links.length === 0) {
    return undefined;
  }
  for (const link of links) {
    const anchor = link.dataset.nwAnchor;
    const page = `/${workspace}/notebooks/${notebook}/pages/${link.dataset.nwNode ?? ""}`;
    link.setAttribute("href", anchor === undefined ? page : `${page}#${anchor}`);
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
    const link = event.target instanceof Element ? event.target.closest("a") : null;
    const to = link?.getAttribute("href");
    if (link === null || to === null || to === undefined || !links.includes(link)) {
      return;
    }
    event.preventDefault();
    navigate(to);
  };
  container.addEventListener("click", onClick);
  return () => {
    container.removeEventListener("click", onClick);
    for (const link of links) {
      link.removeAttribute("href");
    }
  };
};
